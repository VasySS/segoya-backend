// Package data contains logic to run data migrations on Postgres (panorama locations from CSV).
package data

import (
	"context"
	"embed"
	"encoding/csv"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// All files located in data folder
//
//go:embed *
var dataFiles embed.FS

// Migration contains data migration information.
type Migration struct {
	TempTableName      string
	CreateTableSQLPath string
	InsertSQLPath      string
	CSVPath            string
}

//nolint:gochecknoglobals
var migrations = []Migration{
	{
		TempTableName:      "yandex_streetview_temp",
		CreateTableSQLPath: "yandex/00001_create_temptable_streetview.sql",
		InsertSQLPath:      "yandex/00002_insert_csv_streetview.sql",
		CSVPath:            "yandex/00001_yandex_streetview_locations.csv",
	},
	{
		TempTableName:      "yandex_airview_temp",
		CreateTableSQLPath: "yandex/00003_create_temptable_airview.sql",
		InsertSQLPath:      "yandex/00004_insert_csv_airview.sql",
		CSVPath:            "yandex/00001_yandex_airview_locations.csv",
	},
	{
		TempTableName:      "google_temp",
		CreateTableSQLPath: "google/00001_create_temptable_streetview.sql",
		InsertSQLPath:      "google/00002_insert_csv_streetview.sql",
		CSVPath:            "google/00001_google_locations.csv",
	},
	{
		TempTableName:      "seznam_temp",
		CreateTableSQLPath: "seznam/00001_create_temptable_streetview.sql",
		InsertSQLPath:      "seznam/00002_insert_csv_streetview.sql",
		CSVPath:            "seznam/00001_seznam_locations.csv",
	},
}

// MigrateAllCSVData adds all panorama data to the Postgres database from CSV files.
func MigrateAllCSVData(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, migration := range migrations {
		slog.Info("running data migration", slog.String("name", migration.CreateTableSQLPath))

		if err := processDataMigration(ctx, tx, migration); err != nil {
			return err
		}

		slog.Info("data migration completed", slog.String("name", migration.CreateTableSQLPath))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func processDataMigration(
	ctx context.Context,
	conn pgx.Tx,
	migration Migration,
) error {
	createTempTableSQL, err := dataFiles.ReadFile(migration.CreateTableSQLPath)
	if err != nil {
		return fmt.Errorf("failed to read temp table SQL: %w", err)
	}

	// Execute CREATE TEMP TABLE
	if _, err := conn.Exec(ctx, string(createTempTableSQL)); err != nil {
		return fmt.Errorf("failed to create temp table: %w", err)
	}

	rows, csvHeaders, err := parseCSVRows(migration)
	if err != nil {
		return err
	}

	// Copy data from CSV to temp table
	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{migration.TempTableName},
		csvHeaders,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("failed to copy CSV data: %w", err)
	}

	insertSQL, err := dataFiles.ReadFile(migration.InsertSQLPath)
	if err != nil {
		return fmt.Errorf("failed to read insert SQL: %w", err)
	}

	// Execute INSERT INTO ... SELECT to add data to the main table
	if _, err := conn.Exec(ctx, string(insertSQL)); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}

	return nil
}

func parseCSVRows(migration Migration) ([][]any, []string, error) {
	// Read embedded CSV data
	csvFile, err := dataFiles.Open(migration.CSVPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CSV: %w", err)
	}

	defer func() {
		_ = csvFile.Close()
	}()

	reader := csv.NewReader(csvFile)

	csvHeaders, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Read and parse CSV rows
	var rows [][]any

	for {
		record, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}

			return nil, nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		rowData := make([]any, len(csvHeaders))
		for i := range csvHeaders {
			rowData[i] = record[i]
		}

		rows = append(rows, rowData)
	}

	return rows, csvHeaders, nil
}
