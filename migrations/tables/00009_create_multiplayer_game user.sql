-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_game_user (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id BIGINT NOT NULL,
    game_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_info(id),
    FOREIGN KEY (game_id) REFERENCES multiplayer_game(id),
    UNIQUE (user_id, game_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_game_user;
-- +goose StatementEnd
