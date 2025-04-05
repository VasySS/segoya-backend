-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_round_user (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    round_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    score BIGINT NOT NULL,
    distance_miss_meters BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (round_id) REFERENCES multiplayer_round(id),
    FOREIGN KEY (user_id) REFERENCES user_info(id),
    UNIQUE (round_id, user_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_round_user;
-- +goose StatementEnd
