-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS multiplayer_round_user (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    round_id UUID NOT NULL REFERENCES multiplayer_round(id),
    user_id UUID NOT NULL REFERENCES user_info(id),
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    score BIGINT NOT NULL,
    distance_miss_meters BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE (round_id, user_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS multiplayer_round_user;
-- +goose StatementEnd
