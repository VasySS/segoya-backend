-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_game (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    creator_id BIGINT NOT NULL,
    timer_seconds BIGINT NOT NULL,
    provider panorama_provider NOT NULL,
    movement_allowed BOOLEAN NOT NULL,
    rounds BIGINT NOT NULL,
    players BIGINT NOT NULL,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    FOREIGN KEY (creator_id) REFERENCES user_info(id)
);

CREATE INDEX IF NOT EXISTS multiplayer_game_creator_id_idx ON multiplayer_game(creator_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_game;
-- +goose StatementEnd
