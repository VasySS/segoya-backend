-- +goose Up
-- +goose StatementBegin
ALTER TABLE singleplayer_game
    ADD COLUMN scoring_version SMALLINT CHECK (scoring_version IN (1, 2)),
    ADD COLUMN scoring_scale_meters DOUBLE PRECISION
        CHECK (scoring_scale_meters > 0 AND scoring_scale_meters < 'Infinity'::DOUBLE PRECISION),
    ADD CONSTRAINT singleplayer_game_scoring_check
        CHECK ((scoring_version IS NULL) = (scoring_scale_meters IS NULL));

ALTER TABLE multiplayer_game
    ADD COLUMN scoring_version SMALLINT CHECK (scoring_version IN (1, 2)),
    ADD COLUMN scoring_scale_meters DOUBLE PRECISION
        CHECK (scoring_scale_meters > 0 AND scoring_scale_meters < 'Infinity'::DOUBLE PRECISION),
    ADD CONSTRAINT multiplayer_game_scoring_check
        CHECK ((scoring_version IS NULL) = (scoring_scale_meters IS NULL));

COMMENT ON COLUMN singleplayer_game.scoring_version IS
    'Frozen curve: 1 is provider Gaussian, 2 is area exponential; NULL initializes on the next unfinished-game guess.';
COMMENT ON COLUMN singleplayer_game.scoring_scale_meters IS
    'Frozen distance scale in metres: Gaussian standard deviation for version 1, exponential decay length for version 2.';
COMMENT ON COLUMN multiplayer_game.scoring_version IS
    'Frozen curve: 1 is provider Gaussian, 2 is area exponential; NULL initializes on the next unfinished-game guess.';
COMMENT ON COLUMN multiplayer_game.scoring_scale_meters IS
    'Frozen distance scale in metres: Gaussian standard deviation for version 1, exponential decay length for version 2.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE multiplayer_game DROP COLUMN scoring_scale_meters, DROP COLUMN scoring_version;
ALTER TABLE singleplayer_game DROP COLUMN scoring_scale_meters, DROP COLUMN scoring_version;
-- +goose StatementEnd
