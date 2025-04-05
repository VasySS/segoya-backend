-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_oauth (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    user_id BIGINT NOT NULL,
    oauth_id VARCHAR NOT NULL,
    issuer VARCHAR NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user_info(id),
    UNIQUE (oauth_id, issuer)
);

CREATE INDEX user_oauth_user_id_issuer_idx ON user_oauth (user_id, issuer);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_oauth;
-- +goose StatementEnd
