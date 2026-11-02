-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS postgis;

ALTER TABLE panorama_location
    ADD COLUMN geom geometry(Point, 4326)
        GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(lng, lat), 4326)) STORED;

COMMENT ON COLUMN panorama_location.geom IS
    'Stored WGS84 point generated from lng and lat; X is longitude and Y is latitude.';

CREATE INDEX panorama_location_geom_idx ON panorama_location USING GIST (geom);

CREATE TABLE play_area_preset (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('country', 'region')),
    country_code TEXT NOT NULL,
    version TEXT NOT NULL,
    source TEXT NOT NULL,
    license TEXT NOT NULL,
    attribution TEXT NOT NULL,
    providers TEXT[] NOT NULL,
    geom geometry(MultiPolygon, 4326) NOT NULL,
    display_geom geometry(MultiPolygon, 4326) NOT NULL,
    CHECK (ST_IsValid(geom) AND NOT ST_IsEmpty(geom)),
    CHECK (ST_IsValid(display_geom) AND NOT ST_IsEmpty(display_geom))
);

COMMENT ON TABLE play_area_preset IS
    'Versioned country and region boundaries used to restrict round starting locations.';
COMMENT ON COLUMN play_area_preset.id IS
    'Stable UUID derived from the pinned source version and source feature identifier.';
COMMENT ON COLUMN play_area_preset.name IS
    'Display name of the country or region.';
COMMENT ON COLUMN play_area_preset.kind IS
    'Administrative area category: country or region.';
COMMENT ON COLUMN play_area_preset.country_code IS
    'Catalog country or territory code; identifies the parent country for regional presets.';
COMMENT ON COLUMN play_area_preset.version IS
    'Pinned boundary-source version used to preserve the meaning of existing preset references.';
COMMENT ON COLUMN play_area_preset.source IS
    'Pinned upstream URL identifying the boundary dataset.';
COMMENT ON COLUMN play_area_preset.license IS
    'Original source license retained with the boundary.';
COMMENT ON COLUMN play_area_preset.attribution IS
    'Source acknowledgements and copyright attribution to expose with the preset.';
COMMENT ON COLUMN play_area_preset.providers IS
    'Eligible panorama providers; location counts are calculated separately for each provider.';
COMMENT ON COLUMN play_area_preset.geom IS
    'Full-resolution WGS84 boundary used for membership, counts, and round selection; includes boundary points.';
COMMENT ON COLUMN play_area_preset.display_geom IS
    'Simplified, validity-repaired WGS84 outline for map display only; never used for location eligibility.';

CREATE INDEX play_area_preset_geom_idx ON play_area_preset USING GIST (geom);
CREATE INDEX play_area_preset_providers_idx ON play_area_preset USING GIN (providers);

CREATE TABLE play_area_preset_location (
    preset_id UUID NOT NULL REFERENCES play_area_preset(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES panorama_location(id) ON DELETE CASCADE,
    PRIMARY KEY (preset_id, location_id)
);

COMMENT ON TABLE play_area_preset_location IS
    'Precomputed provider-compatible memberships, including boundary points; overlapping presets can share locations.';
COMMENT ON COLUMN play_area_preset_location.preset_id IS
    'Preset whose full-resolution boundary covers the location.';
COMMENT ON COLUMN play_area_preset_location.location_id IS
    'Stored panorama location belonging to the preset; unique within each preset.';

CREATE INDEX play_area_preset_location_location_id_idx
    ON play_area_preset_location (location_id, preset_id);

CREATE TABLE saved_play_area (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    owner_id UUID NOT NULL REFERENCES user_info(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
    polygon geometry(Polygon, 4326) NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'unlisted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (ST_IsValid(polygon) AND NOT ST_IsEmpty(polygon))
);

COMMENT ON TABLE saved_play_area IS
    'User-owned named custom polygons; applied lobby and game settings retain independent polygon snapshots.';
COMMENT ON COLUMN saved_play_area.id IS
    'Saved-area identifier used to apply the area or share an unlisted area by link.';
COMMENT ON COLUMN saved_play_area.owner_id IS
    'Owning user; only the owner may edit or delete the saved area.';
COMMENT ON COLUMN saved_play_area.name IS
    'Display name with a trimmed length of 1-100 characters.';
COMMENT ON COLUMN saved_play_area.polygon IS
    'Provider-independent WGS84 polygon; X is longitude and Y is latitude. The API accepts one ring without holes.';
COMMENT ON COLUMN saved_play_area.visibility IS
    'Private permits owner access; unlisted also permits other authenticated users to read and use the area by ID.';
COMMENT ON COLUMN saved_play_area.created_at IS
    'Time the saved area was created.';
COMMENT ON COLUMN saved_play_area.updated_at IS
    'Time the saved area was last updated.';

CREATE INDEX saved_play_area_owner_created_idx ON saved_play_area (owner_id, created_at DESC, id DESC);

ALTER TABLE singleplayer_game
    ADD COLUMN play_area_preset_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN play_area_polygon geometry(Polygon, 4326),
    ADD CONSTRAINT singleplayer_game_play_area_check
        CHECK (cardinality(play_area_preset_ids) = 0 OR play_area_polygon IS NULL),
    ADD CONSTRAINT singleplayer_game_play_area_polygon_check
        CHECK (ST_IsValid(play_area_polygon) AND NOT ST_IsEmpty(play_area_polygon));

ALTER TABLE multiplayer_game
    ADD COLUMN play_area_preset_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN play_area_polygon geometry(Polygon, 4326),
    ADD CONSTRAINT multiplayer_game_play_area_check
        CHECK (cardinality(play_area_preset_ids) = 0 OR play_area_polygon IS NULL),
    ADD CONSTRAINT multiplayer_game_play_area_polygon_check
        CHECK (ST_IsValid(play_area_polygon) AND NOT ST_IsEmpty(play_area_polygon));

COMMENT ON COLUMN singleplayer_game.play_area_preset_ids IS
    'Immutable preset references for round starts; empty with no polygon means the whole provider.';
COMMENT ON COLUMN singleplayer_game.play_area_polygon IS
    'Independent WGS84 polygon snapshot for round starts; mutually exclusive with preset references.';
COMMENT ON COLUMN multiplayer_game.play_area_preset_ids IS
    'Immutable preset references for round starts; empty with no polygon means the whole provider.';
COMMENT ON COLUMN multiplayer_game.play_area_polygon IS
    'Independent WGS84 polygon snapshot for round starts; mutually exclusive with preset references.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE multiplayer_game DROP COLUMN play_area_polygon, DROP COLUMN play_area_preset_ids;
ALTER TABLE singleplayer_game DROP COLUMN play_area_polygon, DROP COLUMN play_area_preset_ids;
DROP TABLE saved_play_area;
DROP TABLE play_area_preset_location;
DROP TABLE play_area_preset;
DROP INDEX panorama_location_geom_idx;
ALTER TABLE panorama_location DROP COLUMN geom;
-- Preserve PostGIS: it may be used independently of these tables.
-- +goose StatementEnd
