// package main contains the logic for running migrations on Postgres from CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	dataMigrations "github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

type env struct {
	PostgresUser     string `env:"PG_USER" env-required:"true"`
	PostgresPassword string `env:"PG_PASS" env-required:"true"`
	PostgresHost     string `env:"PG_HOST" env-required:"true"`
	PostgresDatabase string `env:"PG_DB"   env-required:"true"`
}

var (
	//nolint:gochecknoglobals
	parsedENV env
	//nolint:gochecknoglobals
	flags = flag.NewFlagSet("migrate", flag.ExitOnError)
	//nolint:gochecknoglobals
	flagUsagePrefix = `
Usage: migrate COMMAND
Example: migrate up`
	//nolint:gochecknoglobals
	flagsUsageCommands = `
Commands:
	up                  Migrate the DB to the most recent version available
	up-to VERSION       Migrate the DB to a specific VERSION
	up-with-data        Migrate the DB to the most recent version available with data migrations
	data-only           Run only data migrations
	down                Roll back the version by 1
	down-to VERSION     Roll back to the specified VERSION
	version             Print the current version
	status              Print the status of the current DB
	`
)

const (
	upFlag         = "up"
	upToFlag       = "up-to"
	upWithDataFlag = "up-with-data"
	dataOnlyFlag   = "data-only"
	downFlag       = "down"
	downToFlag     = "down-to"
	versionFlag    = "version"
	statusFlag     = "status"
)

func flagsUsage() {
	fmt.Println(flagUsagePrefix) //nolint:forbidigo
	flags.PrintDefaults()
	fmt.Println(flagsUsageCommands) //nolint:forbidigo
}

func main() {
	ctx := context.Background()

	command, commandArgs, err := parseEnvAndCommand()
	if err != nil {
		slog.Error("failed to parse env and command", slog.Any("error", err))
		return
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s",
		parsedENV.PostgresUser, parsedENV.PostgresPassword, parsedENV.PostgresHost, parsedENV.PostgresDatabase)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		slog.Error("unable to connect to database", slog.Any("error", err))
		return
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to ping database", slog.Any("error", err))
		return
	}

	if command != dataOnlyFlag {
		slog.Info("creating tables in the database")

		gooseCommand := command
		if gooseCommand == upWithDataFlag {
			gooseCommand = upFlag
		}

		if err := tables.RunGooseMigrations(ctx, pool, gooseCommand, commandArgs...); err != nil {
			slog.Error("unable to migrate data", slog.Any("error", err))
			return
		}
	}

	if command == upWithDataFlag || command == dataOnlyFlag {
		slog.Info("adding data to the tables")

		if err := dataMigrations.MigrateAllCSVData(ctx, pool); err != nil {
			slog.Error("unable to migrate data", slog.Any("error", err))
			return
		}
	}
}

func parseEnvAndCommand() (string, []string, error) {
	flags.Usage = flagsUsage
	if err := flags.Parse(os.Args[1:]); err != nil {
		return "", nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	flagArgs := flags.Args()
	if len(flagArgs) == 0 || flagArgs[0] == "-h" || flagArgs[0] == "--help" {
		flags.Usage()
		return "", nil, nil
	}

	command := flagArgs[0]
	commandArgs := flagArgs[1:]

	if err := cleanenv.ReadConfig(".env", &parsedENV); err != nil {
		slog.Info("failed to read .env, using environment variables")
	}

	if err := cleanenv.ReadEnv(&parsedENV); err != nil {
		slog.Error("failed to read environment variables", slog.Any("error", err))
		return "", nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	return command, commandArgs, nil
}
