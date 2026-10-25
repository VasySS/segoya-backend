package data

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type playAreaManifest struct {
	Version int              `json:"version"`
	Sources []playAreaSource `json:"sources"`
}

type playAreaSource struct {
	File         string   `json:"file"`
	URL          string   `json:"url"`
	SHA256       string   `json:"sha256"`
	Size         int64    `json:"size"`
	Format       string   `json:"format"`
	Kind         string   `json:"kind"`
	CountryCode  string   `json:"countryCode"`
	Name         string   `json:"name"`
	SourceCode   string   `json:"sourceCode"`
	Version      string   `json:"version"`
	License      string   `json:"license"`
	Attribution  string   `json:"attribution"`
	Providers    []string `json:"providers"`
	FeatureCount int      `json:"featureCount"`
}

type playAreaFeature struct {
	Type       string          `json:"type"`
	Properties map[string]any  `json:"properties"`
	Geometry   json.RawMessage `json:"geometry"`
}

var errInvalidPlayAreaCatalog = errors.New("invalid play area catalog")

//nolint:cyclop // Validate every pinned manifest field before reading the bundled catalog.
func readPlayAreaManifest() (playAreaManifest, error) {
	content, err := dataFiles.ReadFile("playareas/sources.json")
	if err != nil {
		return playAreaManifest{}, fmt.Errorf("read play area manifest: %w", err)
	}

	var manifest playAreaManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return manifest, fmt.Errorf("decode play area manifest: %w", err)
	}

	if manifest.Version != 1 || len(manifest.Sources) == 0 {
		return manifest, fmt.Errorf("%w: unsupported or empty manifest", errInvalidPlayAreaCatalog)
	}

	seen := make(map[string]bool, len(manifest.Sources))

	for _, source := range manifest.Sources {
		checksum, checksumErr := hex.DecodeString(source.SHA256)
		if source.File != filepath.Base(source.File) || !strings.HasSuffix(source.File, ".geojson") ||
			seen[source.File] || checksumErr != nil || len(checksum) != sha256.Size || source.Size <= 0 ||
			source.FeatureCount <= 0 || source.Version == "" || source.SourceCode == "" ||
			(source.Format != "geoboundaries" && source.Format != "nuts") ||
			(source.Kind != "country" && source.Kind != "region") || !strings.HasPrefix(source.URL, "https://") {
			return manifest, fmt.Errorf("%w: malformed source %q", errInvalidPlayAreaCatalog, source.File)
		}

		seen[source.File] = true
	}

	return manifest, nil
}

func verifyPlayAreaSource(reader io.Reader, source playAreaSource) error {
	hash := sha256.New()

	size, err := io.Copy(hash, io.LimitReader(reader, source.Size+1))
	if err != nil {
		return fmt.Errorf("read boundary file for checksum: %w", err)
	}

	if size != source.Size || hex.EncodeToString(hash.Sum(nil)) != source.SHA256 {
		return fmt.Errorf("%w: bundled %s has wrong size or SHA-256",
			errInvalidPlayAreaCatalog, source.File)
	}

	return nil
}

// ImportPlayAreas imports the embedded boundary catalog and backfills memberships
// in one transaction, without network or filesystem dependencies. Repeating the same
// pinned import preserves preset IDs and does not insert panorama locations.
func ImportPlayAreas(ctx context.Context, pool *pgxpool.Pool) error {
	manifest, err := readPlayAreaManifest()
	if err != nil {
		return err
	}

	sources, err := fs.Sub(dataFiles, "playareas/boundaries")
	if err != nil {
		return fmt.Errorf("open bundled play area catalog: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin play area import: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// Serializes catalog imports with one another, without touching game locks.
	if err := lockPlayAreaCatalog(ctx, tx); err != nil {
		return err
	}

	for _, source := range manifest.Sources {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("import play area catalog: %w", err)
		}

		if err := importPlayAreaSource(ctx, tx, sources, source); err != nil {
			return fmt.Errorf("import %s: %w", source.File, err)
		}

		slog.InfoContext(ctx, "imported play area boundary source", slog.String("file", source.File))
	}

	if err := refreshPlayAreaMemberships(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit play area import: %w", err)
	}

	return nil
}

func importPlayAreaSource(ctx context.Context, tx pgx.Tx, sources fs.FS, source playAreaSource) error {
	return readVerifiedPlayAreaSource(sources, source, func(id uuid.UUID, name string, geometry json.RawMessage) error {
		return insertPlayAreaPreset(ctx, tx, source, geometry, id, name)
	})
}

// VerifyPlayAreaSources checks every bundled source's checksum, GeoJSON structure,
// feature identities, and expected region count without connecting to a database.
func VerifyPlayAreaSources(ctx context.Context) error {
	manifest, err := readPlayAreaManifest()
	if err != nil {
		return err
	}

	sources, err := fs.Sub(dataFiles, "playareas/boundaries")
	if err != nil {
		return fmt.Errorf("open bundled play area catalog: %w", err)
	}

	for _, source := range manifest.Sources {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("verify play area catalog: %w", err)
		}

		if err := readVerifiedPlayAreaSource(sources, source, func(uuid.UUID, string, json.RawMessage) error {
			return ctx.Err()
		}); err != nil {
			return fmt.Errorf("verify %s: %w", source.File, err)
		}
	}

	return nil
}

func readVerifiedPlayAreaSource(
	sources fs.FS,
	source playAreaSource,
	visit func(uuid.UUID, string, json.RawMessage) error,
) error {
	if err := verifyCompressedPlayAreaSource(sources, source); err != nil {
		return err
	}

	file, err := sources.Open(source.File + ".gz")
	if err != nil {
		return fmt.Errorf("open bundled boundary source: %w", err)
	}

	defer func() { _ = file.Close() }()

	reader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("decompress boundary source: %w", err)
	}

	defer func() { _ = reader.Close() }()

	count := 0
	seen := make(map[uuid.UUID]bool, source.FeatureCount)

	err = readPlayAreaFeatures(reader, func(feature playAreaFeature) error {
		id, name, include, err := playAreaFeatureIdentity(source, feature)
		if err != nil || !include {
			return err
		}

		if seen[id] {
			return fmt.Errorf("%w: duplicate feature ID %s", errInvalidPlayAreaCatalog, id)
		}

		seen[id] = true

		if err := visit(id, name, feature.Geometry); err != nil {
			return err
		}

		count++

		return nil
	})
	if err != nil {
		return err
	}

	if count != source.FeatureCount {
		return fmt.Errorf("%w: expected %d features, found %d", errInvalidPlayAreaCatalog, source.FeatureCount, count)
	}

	return nil
}

func verifyCompressedPlayAreaSource(sources fs.FS, source playAreaSource) error {
	file, err := sources.Open(source.File + ".gz")
	if err != nil {
		return fmt.Errorf("open bundled boundary source: %w", err)
	}

	defer func() { _ = file.Close() }()

	reader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("decompress boundary source: %w", err)
	}

	defer func() { _ = reader.Close() }()

	return verifyPlayAreaSource(reader, source)
}

//nolint:cyclop // Streaming GeoJSON token dispatch keeps large source files out of memory.
func readPlayAreaFeatures(reader io.Reader, visit func(playAreaFeature) error) error {
	decoder := json.NewDecoder(reader)

	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("%w: expected GeoJSON object", errInvalidPlayAreaCatalog)
	}

	collectionType := ""
	foundFeatures := false

	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("read GeoJSON key: %w", err)
		}

		switch key {
		case "type":
			if err := decoder.Decode(&collectionType); err != nil {
				return fmt.Errorf("decode GeoJSON type: %w", err)
			}
		case "features":
			if foundFeatures {
				return fmt.Errorf("%w: duplicate features member", errInvalidPlayAreaCatalog)
			}

			foundFeatures = true

			if err := readPlayAreaFeatureArray(decoder, visit); err != nil {
				return err
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return fmt.Errorf("decode GeoJSON metadata: %w", err)
			}
		}
	}

	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("close GeoJSON collection: %w", err)
	}

	if collectionType != "FeatureCollection" || !foundFeatures {
		return fmt.Errorf("%w: expected FeatureCollection", errInvalidPlayAreaCatalog)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing GeoJSON content", errInvalidPlayAreaCatalog)
	}

	return nil
}

func readPlayAreaFeatureArray(decoder *json.Decoder, visit func(playAreaFeature) error) error {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return fmt.Errorf("%w: expected feature array", errInvalidPlayAreaCatalog)
	}

	for decoder.More() {
		var feature playAreaFeature
		if err := decoder.Decode(&feature); err != nil {
			return fmt.Errorf("decode GeoJSON feature: %w", err)
		}

		if feature.Type != "Feature" {
			return fmt.Errorf("%w: expected GeoJSON feature", errInvalidPlayAreaCatalog)
		}

		if err := visit(feature); err != nil {
			return err
		}
	}

	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("close GeoJSON feature array: %w", err)
	}

	return nil
}

func playAreaFeatureIdentity(source playAreaSource, feature playAreaFeature) (uuid.UUID, string, bool, error) {
	code := source.SourceCode

	name := source.Name

	switch source.Format {
	case "geoboundaries":
		if source.Kind == "region" {
			code, _ = feature.Properties["shapeID"].(string)
			name, _ = feature.Properties["shapeName"].(string)
		}
	case "nuts":
		if feature.Properties["CNTR_CODE"] != "CZ" {
			return uuid.Nil, "", false, nil
		}

		code, _ = feature.Properties["NUTS_ID"].(string)
		name, _ = feature.Properties["NAME_LATN"].(string)
	}

	var geometry struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(feature.Geometry, &geometry); err != nil {
		return uuid.Nil, "", false, fmt.Errorf("decode boundary geometry: %w", err)
	}

	if name == "" || code == "" || (geometry.Type != "Polygon" && geometry.Type != "MultiPolygon") {
		return uuid.Nil, "", false, fmt.Errorf("%w: missing feature identity or polygon geometry", errInvalidPlayAreaCatalog)
	}
	// UUID v5 is used solely as a stable identifier, never as a security primitive.
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://segoya.app/play-areas/"+source.Version+"/"+code))

	return id, name, true, nil
}

func insertPlayAreaPreset(
	ctx context.Context,
	tx pgx.Tx,
	source playAreaSource,
	geometry json.RawMessage,
	id uuid.UUID,
	name string,
) error {
	const query = `
WITH boundary AS (
    SELECT ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SetSRID(ST_GeomFromGeoJSON($10::text), 4326)), 3)) AS geom
), inserted AS (
    INSERT INTO play_area_preset (id, name, kind, country_code, version, source,
        license, attribution, providers, geom, display_geom)
    SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9::text[], geom,
        ST_Multi(ST_CollectionExtract(ST_MakeValid(ST_SimplifyPreserveTopology(geom, 0.01)), 3))
    FROM boundary WHERE NOT ST_IsEmpty(geom)
    ON CONFLICT (id) DO NOTHING
    RETURNING id
)
SELECT EXISTS (SELECT 1 FROM inserted) OR EXISTS (
    SELECT 1 FROM play_area_preset p, boundary b
    WHERE p.id = $1 AND p.name = $2 AND p.kind = $3 AND p.country_code = $4
        AND p.version = $5 AND p.source = $6 AND p.license = $7 AND p.attribution = $8
        AND p.providers = $9::text[] AND ST_Equals(p.geom, b.geom)
)`

	var matches bool
	if err := tx.QueryRow(ctx, query, id, name, source.Kind, source.CountryCode, source.Version,
		source.URL, source.License, source.Attribution, source.Providers, string(geometry)).Scan(&matches); err != nil {
		return fmt.Errorf("insert play area preset %s: %w", id, err)
	}

	if !matches {
		return fmt.Errorf("%w: preset %s is empty or differs from its immutable version", errInvalidPlayAreaCatalog, id)
	}

	return nil
}

// RefreshPlayAreaMemberships backfills missing memberships without importing CSV
// locations or changing existing preset versions. New location imports call the
// same SQL inside their import transaction.
func RefreshPlayAreaMemberships(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin play area membership backfill: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := refreshPlayAreaMemberships(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit play area membership backfill: %w", err)
	}

	return nil
}

func refreshPlayAreaMemberships(ctx context.Context, tx pgx.Tx) error {
	if err := lockPlayAreaCatalog(ctx, tx); err != nil {
		return err
	}

	const query = `
INSERT INTO play_area_preset_location (preset_id, location_id)
SELECT preset.id, location.id
FROM play_area_preset preset
JOIN panorama_location location ON location.provider::text = ANY(preset.providers)
    AND ST_Covers(preset.geom, location.geom)
ON CONFLICT (preset_id, location_id) DO NOTHING`

	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("backfill play area memberships: %w", err)
	}

	return nil
}

func lockPlayAreaCatalog(ctx context.Context, tx pgx.Tx) error {
	query := "SELECT pg_advisory_xact_lock(731904264)"

	if _, err := tx.Exec(ctx, query); err != nil {
		return fmt.Errorf("lock play area catalog maintenance: %w", err)
	}

	return nil
}
