-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS postgis;

ALTER TABLE panorama_location
    ADD COLUMN geom geometry(Point, 4326)
        GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(lng, lat), 4326)) STORED;

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

CREATE INDEX play_area_preset_geom_idx ON play_area_preset USING GIST (geom);
CREATE INDEX play_area_preset_providers_idx ON play_area_preset USING GIN (providers);

CREATE TABLE play_area_preset_location (
    preset_id UUID NOT NULL REFERENCES play_area_preset(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES panorama_location(id) ON DELETE CASCADE,
    PRIMARY KEY (preset_id, location_id)
);

CREATE INDEX play_area_preset_location_location_id_idx
    ON play_area_preset_location (location_id, preset_id);

CREATE TABLE saved_play_area (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    owner_id UUID NOT NULL REFERENCES user_info(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 100),
    polygon JSONB NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'unlisted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (polygon->>'type' = 'Polygon'),
    CHECK (ST_IsValid(ST_SetSRID(ST_GeomFromGeoJSON(polygon), 4326)))
);

CREATE INDEX saved_play_area_owner_created_idx ON saved_play_area (owner_id, created_at DESC, id DESC);

ALTER TABLE singleplayer_game ADD COLUMN play_area JSONB NOT NULL DEFAULT '{"type":"all"}';
ALTER TABLE multiplayer_game ADD COLUMN play_area JSONB NOT NULL DEFAULT '{"type":"all"}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE multiplayer_game DROP COLUMN play_area;
ALTER TABLE singleplayer_game DROP COLUMN play_area;
DROP TABLE saved_play_area;
DROP TABLE play_area_preset_location;
DROP TABLE play_area_preset;
DROP INDEX panorama_location_geom_idx;
ALTER TABLE panorama_location DROP COLUMN geom;
-- Preserve PostGIS: it may be used independently of these tables.
-- +goose StatementEnd
