-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS singleplayer_game (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES user_info(id),
    rounds BIGINT NOT NULL,
    provider panorama_provider NOT NULL,
    movement_allowed BOOLEAN NOT NULL,
    timer_seconds BIGINT NOT NULL,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP
);

CREATE INDEX singleplayer_game_user_id_created_at_idx ON singleplayer_game (user_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS singleplayer_game;
-- +goose StatementEnd
