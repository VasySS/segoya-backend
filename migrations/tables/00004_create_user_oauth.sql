-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_oauth (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES user_info(id),
    oauth_id TEXT NOT NULL,
    issuer TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE (oauth_id, issuer)
);

CREATE INDEX user_oauth_user_id_issuer_idx ON user_oauth (user_id, issuer);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_oauth;
-- +goose StatementEnd
