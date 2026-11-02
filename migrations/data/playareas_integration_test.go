//go:build integration

//nolint:testpackage // Exercise private source ingestion and membership refresh in one transaction.
package data

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func TestPlayAreaImportAndMembershipBackfill(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)
	container := containers.NewTestPostgres(t)
	pool := containers.NewTestPostgresPool(t, container.ConnectionString)
	require.NoError(t, tables.RunGooseMigrations(ctx, pool, "up"))

	fixture, err := dataFiles.ReadFile("playareas/testdata/regions.geojson")
	require.NoError(t, err)

	hash := sha256.Sum256(fixture)
	source := playAreaSource{
		File: "regions.geojson", URL: "https://example.org/immutable-regions.geojson",
		SHA256: hex.EncodeToString(hash[:]), Size: int64(len(fixture)),
		Format: "geoboundaries", Kind: "region", CountryCode: "TST", SourceCode: "test",
		Version: "test-v1", License: "test", Attribution: "test", Providers: []string{"google"}, FeatureCount: 2,
	}

	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write(fixture)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	sources := fstest.MapFS{source.File + ".gz": &fstest.MapFile{Data: compressed.Bytes()}}

	_, err = pool.Exec(ctx, "INSERT INTO panorama_location (lat,lng,provider) VALUES "+
		"(0.5,0.5,'google'),(1.5,1.5,'google'),(1,0,'google'),(9,9,'google'),(0.5,0.5,'yandex')")
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	defer func() { _ = tx.Rollback(ctx) }()

	for range 2 {
		require.NoError(t, importPlayAreaSource(ctx, tx, sources, source))
		require.NoError(t, refreshPlayAreaMemberships(ctx, tx))
	}

	var presets, memberships, distinctLocations, locations int

	err = tx.QueryRow(ctx, "SELECT "+
		"(SELECT COUNT(*) FROM play_area_preset),"+
		"(SELECT COUNT(*) FROM play_area_preset_location),"+
		"(SELECT COUNT(DISTINCT location_id) FROM play_area_preset_location),"+
		"(SELECT COUNT(*) FROM panorama_location)").Scan(&presets, &memberships, &distinctLocations, &locations)
	require.NoError(t, err)
	require.Equal(t, 2, presets)
	require.Equal(t, 4, memberships)
	require.Equal(t, 3, distinctLocations)
	require.Equal(t, 5, locations)

	changed := source
	changed.Attribution = "changed metadata"
	require.ErrorIs(t, importPlayAreaSource(ctx, tx, sources, changed), errInvalidPlayAreaCatalog)
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, "INSERT INTO panorama_location (lat,lng,provider) VALUES (0.75,0.75,'google')")
	require.NoError(t, err)
	require.NoError(t, RefreshPlayAreaMemberships(ctx, pool))
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM play_area_preset_location").Scan(&memberships))
	require.Equal(t, 5, memberships)
}

func TestBundledPlayAreaImport(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	t.Cleanup(cancel)
	container := containers.NewTestPostgres(t)
	pool := containers.NewTestPostgresPool(t, container.ConnectionString)
	require.NoError(t, tables.RunGooseMigrations(ctx, pool, "up"))

	_, err := pool.Exec(ctx, `INSERT INTO panorama_location (lat,lng,provider) VALUES
        (50.0755,14.4378,'google'),(55.7558,37.6173,'yandex'),
        (55.7558,37.6173,'yandex_air'),(50.0755,14.4378,'seznam'),(0,-140,'google')`)
	require.NoError(t, err)
	require.NoError(t, ImportPlayAreas(ctx, pool))

	var countries, russianRegions, czechRegions, classifiedLocations int

	err = pool.QueryRow(ctx, `SELECT
        COUNT(*) FILTER (WHERE kind = 'country'),
        COUNT(*) FILTER (WHERE kind = 'region' AND country_code = 'RUS'),
        COUNT(*) FILTER (WHERE kind = 'region' AND country_code = 'CZE')
        FROM play_area_preset`).Scan(&countries, &russianRegions, &czechRegions)
	require.NoError(t, err)
	require.Equal(t, 230, countries)
	require.Equal(t, 83, russianRegions)
	require.Equal(t, 14, czechRegions)

	err = pool.QueryRow(ctx, `SELECT COUNT(DISTINCT location_id) FROM play_area_preset_location`).
		Scan(&classifiedLocations)
	require.NoError(t, err)
	require.Equal(t, 4, classifiedLocations)

	repository := postgres.New(postgres.NewTxManager(pool))

	for _, provider := range []game.PanoramaProvider{game.GoogleProvider, game.YandexAirProvider} {
		t.Run(string(provider)+" scoring bounds", func(t *testing.T) {
			t.Parallel()

			var ids []uuid.UUID

			err := pool.QueryRow(ctx, `SELECT ARRAY_AGG(id) FROM play_area_preset
				WHERE country_code IN ('RUS','USA','AUS') AND $1=ANY(providers)`, string(provider)).Scan(&ids)
			require.NoError(t, err)
			require.NotEmpty(t, ids)

			queryCtx, stop := context.WithTimeout(ctx, 3*time.Second)
			defer stop()

			started := time.Now()
			bounds, err := repository.GetPlayAreaPresetBounds(queryCtx, provider, ids)
			require.NoError(t, err)
			t.Logf("full-resolution scoring bounds for %d presets took %s", len(ids), time.Since(started))

			_, err = game.AreaScoring(bounds)
			require.NoError(t, err)
		})
	}
}
