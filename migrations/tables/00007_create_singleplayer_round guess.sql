-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS singleplayer_round_guess (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    round_id BIGINT NOT NULL UNIQUE,
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    score BIGINT NOT NULL,
    distance_miss_meters BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (round_id) REFERENCES singleplayer_round(id)
);

CREATE INDEX singleplayer_round_guess_round_id_idx ON singleplayer_round_guess (round_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS singleplayer_round_guess;
-- +goose StatementEnd
