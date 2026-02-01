-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_info (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    name TEXT NOT NULL,
    avatar_hash TEXT DEFAULT NULL, 
    avatar_last_update TIMESTAMP DEFAULT NULL,
    register_date TIMESTAMP NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_info;
-- +goose StatementEnd
