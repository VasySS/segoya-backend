-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
REINDEX (VERBOSE) DATABASE segoya_data;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'DOWN MIGRATION NOT SUPPORTED' AS notice;
-- +goose StatementEnd