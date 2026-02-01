-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_game (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    creator_id UUID NOT NULL REFERENCES user_info(id),
    timer_seconds BIGINT NOT NULL,
    provider panorama_provider NOT NULL,
    movement_allowed BOOLEAN NOT NULL,
    rounds BIGINT NOT NULL,
    players BIGINT NOT NULL,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS multiplayer_game_creator_id_idx ON multiplayer_game(creator_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_game;
-- +goose StatementEnd
