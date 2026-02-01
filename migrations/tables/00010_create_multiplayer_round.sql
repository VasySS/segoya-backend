-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_round (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    game_id UUID NOT NULL REFERENCES multiplayer_game(id),
    location_id UUID NOT NULL REFERENCES panorama_location(id),
    round_num BIGINT NOT NULL,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    UNIQUE (game_id, round_num)
);

CREATE INDEX multiplayer_round_location_id_idx ON multiplayer_round (location_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_round;
-- +goose StatementEnd
