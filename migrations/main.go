// package main contains the logic for running migrations on Postgres from CLI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/jackc/pgx/v5/pgxpool"

	dataMigrations "github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
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
	up-with-data        Migrate the DB to the most recent version available and apply data migrations
	data-only           Run only data migrations
	play-areas-verify              Validate the bundled boundary catalog (no DB required)
	play-areas-import              Import bundled boundaries and backfill memberships
	play-areas-backfill            Backfill memberships without reimporting panorama CSVs
	down                Roll back the version by 1
	down-to VERSION     Roll back to the specified VERSION
	version             Print the current version
	status              Print the status of the current DB
	`
)

const (
	upFlag                = "up"
	upToFlag              = "up-to"
	upWithDataFlag        = "up-with-data"
	dataOnlyFlag          = "data-only"
	downFlag              = "down"
	downToFlag            = "down-to"
	versionFlag           = "version"
	statusFlag            = "status"
	playAreasVerifyFlag   = "play-areas-verify"
	playAreasImportFlag   = "play-areas-import"
	playAreasBackfillFlag = "play-areas-backfill"
)

var errInvalidMigrationCommand = errors.New("invalid migration command")

func flagsUsage() {
	fmt.Println(flagUsagePrefix) //nolint:forbidigo
	flags.PrintDefaults()
	fmt.Println(flagsUsageCommands) //nolint:forbidigo
}

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", slog.Any("error", err))
		os.Exit(1)
	}
}

//nolint:cyclop // Keep command dispatch and sequential migration stages explicit.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command, commandArgs, err := parseEnvAndCommand()
	if err != nil {
		return err
	}

	if command == "" {
		return nil
	}

	if command == playAreasVerifyFlag {
		return migrationResult("verify play area sources", dataMigrations.VerifyPlayAreaSources(ctx))
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s",
		parsedENV.PostgresUser,
		parsedENV.PostgresPassword,
		parsedENV.PostgresHost,
		parsedENV.PostgresDatabase,
	)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if command == playAreasImportFlag {
		return migrationResult("import play areas", dataMigrations.ImportPlayAreas(ctx, pool))
	}

	if command == playAreasBackfillFlag {
		return migrationResult("backfill play areas", dataMigrations.RefreshPlayAreaMemberships(ctx, pool))
	}

	if command != dataOnlyFlag {
		slog.Info("creating tables in the database")

		gooseCommand := command
		if gooseCommand == upWithDataFlag {
			gooseCommand = upFlag
		}

		if err := tables.RunGooseMigrations(ctx, pool, gooseCommand, commandArgs...); err != nil {
			return fmt.Errorf("migrate tables: %w", err)
		}
	}

	if command == upWithDataFlag || command == dataOnlyFlag {
		slog.Info("adding data to the tables")

		if err := dataMigrations.MigrateAllCSVData(ctx, pool); err != nil {
			return fmt.Errorf("import panorama locations: %w", err)
		}
	}

	return nil
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

	if err := validateCommand(command, commandArgs); err != nil {
		return "", nil, err
	}

	if command == playAreasVerifyFlag {
		return command, commandArgs, nil
	}

	if err := cleanenv.ReadConfig(".env", &parsedENV); err != nil {
		slog.Info("failed to read .env, using environment variables")
	}

	if err := cleanenv.ReadEnv(&parsedENV); err != nil {
		slog.Error("failed to read environment variables", slog.Any("error", err))
		return "", nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	return command, commandArgs, nil
}

func validateCommand(command string, args []string) error {
	wantArguments := 0

	switch command {
	case upFlag, upWithDataFlag, dataOnlyFlag, downFlag, versionFlag, statusFlag,
		playAreasBackfillFlag, playAreasVerifyFlag, playAreasImportFlag:
	case upToFlag, downToFlag:
		wantArguments = 1
	default:
		return fmt.Errorf("%w: unknown migration command %q", errInvalidMigrationCommand, command)
	}

	if len(args) != wantArguments {
		return fmt.Errorf("%w: command %s requires %d arguments", errInvalidMigrationCommand, command, wantArguments)
	}

	return nil
}

func migrationResult(action string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	return nil
}
