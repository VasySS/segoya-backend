-- +goose Up
-- +goose StatementBegin
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'panorama_provider') THEN
        CREATE TYPE panorama_provider AS ENUM ('google', 'yandex', 'yandex_air', 'seznam');
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TYPE IF EXISTS panorama_provider;
-- +goose StatementEnd
