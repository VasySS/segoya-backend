//nolint:testpackage // Exercise private streaming parser and pinned manifest validation.
package data

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
)

//nolint:cyclop // Check every source metadata invariant in the pinned catalog.
func TestPlayAreaManifest(t *testing.T) {
	t.Parallel()

	manifest, err := readPlayAreaManifest()
	if err != nil {
		t.Fatal(err)
	}

	countries, russianRegions, czechRegions := 0, 0, 0

	for _, source := range manifest.Sources {
		switch {
		case source.Format == "geoboundaries" && source.Kind == "country":
			countries += source.FeatureCount

			if len(source.Providers) != 1 || source.Providers[0] != "google" {
				t.Fatalf("country has wrong providers: %s", source.File)
			}
		case source.Format == "geoboundaries" && source.CountryCode == "RUS":
			russianRegions += source.FeatureCount
		case source.Format == "nuts" && source.CountryCode == "CZE":
			czechRegions += source.FeatureCount
		default:
			t.Fatalf("unexpected source: %s", source.File)
		}

		if strings.Contains(source.URL, "/current/") || strings.Contains(source.URL, "simplified") ||
			(source.Format == "geoboundaries" && !strings.Contains(source.URL, "/9469f09592ced973a3448cf66b6100b741b64c0d/")) {
			t.Fatalf("source is not pinned full-resolution geometry: %s", source.URL)
		}
	}

	if countries != 230 || russianRegions != 83 || czechRegions != 14 {
		t.Fatalf("catalog coverage: countries=%d, Russian regions=%d, Czech regions=%d",
			countries, russianRegions, czechRegions)
	}
}

func TestReadPlayAreaFeatures(t *testing.T) {
	t.Parallel()

	fixture, err := dataFiles.ReadFile("playareas/testdata/regions.geojson")
	if err != nil {
		t.Fatal(err)
	}

	var features []playAreaFeature

	if err := readPlayAreaFeatures(strings.NewReader(string(fixture)), func(feature playAreaFeature) error {
		features = append(features, feature)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(features) != 2 || features[0].Properties["shapeName"] != "West" ||
		!strings.Contains(string(features[0].Geometry), "MultiPolygon") {
		t.Fatalf("unexpected parsed features: %+v", features)
	}

	for _, invalid := range []string{
		`[]`, `{"type":"Polygon","features":[]}`, `{"type":"FeatureCollection"}`,
		`{"type":"FeatureCollection","features":{}}`,
		`{"type":"FeatureCollection","features":[{"type":"Polygon"}]}`,
		`{"type":"FeatureCollection","features":[],"features":[]}`,
		`{"type":"FeatureCollection","features":[]} true`,
	} {
		if err := readPlayAreaFeatures(strings.NewReader(invalid), func(playAreaFeature) error { return nil }); err == nil {
			t.Errorf("accepted invalid collection: %s", invalid)
		}
	}

	sentinel := errors.New("visitor failed")
	if err := readPlayAreaFeatures(strings.NewReader(string(fixture)), func(playAreaFeature) error {
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("visitor error lost: %v", err)
	}
}

//nolint:cyclop // Exercise identity, versioning, and provider source cases together.
func TestPlayAreaFeatureIdentity(t *testing.T) {
	t.Parallel()

	source := playAreaSource{Format: "geoboundaries", Kind: "region", Version: "fixture-v1"}
	feature := playAreaFeature{
		Properties: map[string]any{"shapeID": "west", "shapeName": "West"},
		Geometry:   json.RawMessage(`{"type":"MultiPolygon","coordinates":[]}`),
	}

	first, name, include, err := playAreaFeatureIdentity(source, feature)
	if err != nil || !include || name != "West" {
		t.Fatalf("unexpected identity: %s %s %t %v", first, name, include, err)
	}

	repeated, _, _, err := playAreaFeatureIdentity(source, feature)
	if err != nil || repeated != first {
		t.Fatalf("unstable preset identity: %s %s %v", first, repeated, err)
	}

	source.Version = "fixture-v2"

	versioned, _, _, err := playAreaFeatureIdentity(source, feature)
	if err != nil || versioned == first {
		t.Fatalf("preset versions share identity: %s %v", versioned, err)
	}

	source.Format = "nuts"

	feature.Properties = map[string]any{"CNTR_CODE": "DE", "NUTS_ID": "DE123", "NAME_LATN": "Other"}

	if _, _, include, err := playAreaFeatureIdentity(source, feature); err != nil || include {
		t.Fatalf("included non-Czech NUTS region: %t %v", include, err)
	}

	feature.Properties = map[string]any{"CNTR_CODE": "CZ", "NUTS_ID": "CZ010", "NAME_LATN": "Praha"}
	if _, name, include, err := playAreaFeatureIdentity(source, feature); err != nil || !include || name != "Praha" {
		t.Fatalf("excluded Czech NUTS region: %s %t %v", name, include, err)
	}
}

func TestBundledPlayAreaSources(t *testing.T) {
	t.Parallel()

	if err := VerifyPlayAreaSources(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestReadVerifiedPlayAreaSource(t *testing.T) {
	t.Parallel()

	fixture, err := dataFiles.ReadFile("playareas/testdata/regions.geojson")
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(fixture)
	source := playAreaSource{
		File: "regions.geojson", Size: int64(len(fixture)), SHA256: hex.EncodeToString(hash[:]),
		Format: "geoboundaries", Kind: "region", Version: "fixture-v1", FeatureCount: 2,
	}

	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(fixture); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		content    []byte
		checksum   string
		valid      bool
		wantVisits int
	}{
		{name: "compressed source", content: compressed.Bytes(), checksum: source.SHA256, valid: true, wantVisits: 2},
		{name: "wrong checksum", content: compressed.Bytes(), checksum: strings.Repeat("0", 64)},
		{name: "truncated gzip", content: compressed.Bytes()[:compressed.Len()-4], checksum: source.SHA256},
		{name: "uncompressed source", content: fixture, checksum: source.SHA256},
		{name: "missing source", checksum: source.SHA256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sources := fstest.MapFS{}
			if tc.content != nil {
				sources[source.File+".gz"] = &fstest.MapFile{Data: tc.content}
			}

			selected := source
			selected.SHA256 = tc.checksum
			visits := 0

			err := readVerifiedPlayAreaSource(sources, selected, func(uuid.UUID, string, json.RawMessage) error {
				visits++
				return nil
			})
			if (err == nil) != tc.valid {
				t.Fatalf("read source: %v, expected valid=%t", err, tc.valid)
			}

			if visits != tc.wantVisits {
				t.Fatalf("expected %d verified features, got %d", tc.wantVisits, visits)
			}
		})
	}
}

func TestVerifyPlayAreaSourcesCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := VerifyPlayAreaSources(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled verification, got %v", err)
	}
}
