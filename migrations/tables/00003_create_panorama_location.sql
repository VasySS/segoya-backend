-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS panorama_location (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    streetview_id TEXT, 
    provider panorama_provider NOT NULL,
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL
);

CREATE INDEX panorama_location_provider_idx ON panorama_location (provider);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS panorama_location;
-- +goose StatementEnd
