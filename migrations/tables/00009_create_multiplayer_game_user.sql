-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_game_user (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES user_info(id),
    game_id UUID NOT NULL REFERENCES multiplayer_game(id),
    created_at TIMESTAMP NOT NULL,
    UNIQUE (user_id, game_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_game_user;
-- +goose StatementEnd
