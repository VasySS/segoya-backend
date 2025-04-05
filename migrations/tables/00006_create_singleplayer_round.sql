-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS singleplayer_round (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    game_id BIGINT NOT NULL,
    location_id BIGINT NOT NULL,
    round_num BIGINT NOT NULL,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP NOT NULL, 
    ended_at TIMESTAMP,
    FOREIGN KEY (game_id) REFERENCES singleplayer_game(id),
    FOREIGN KEY (location_id) REFERENCES panorama_location(id),
    UNIQUE (game_id, round_num)
);

CREATE INDEX singleplayer_round_location_id_idx ON singleplayer_round (location_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS singleplayer_round;
-- +goose StatementEnd
