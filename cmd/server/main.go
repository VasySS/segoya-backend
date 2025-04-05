// Package main is the entry point for the application.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/VasySS/segoya-backend/internal/app"
	"github.com/VasySS/segoya-backend/internal/config"
)

func main() {
	conf := config.MustInit()

	setupLogger(conf.ENV.Mode)

	slog.Info("starting app", slog.String("mode", conf.ENV.Mode))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, conf); err != nil {
		slog.Error("error running server", slog.Any("error", err))
	}
}

func setupLogger(envMode string) {
	var slogLogger *slog.Logger

	if envMode == "production" {
		slogLogger = slog.New(
			slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}),
		)
	} else {
		slogLogger = slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}),
		)
	}

	slog.SetDefault(slogLogger)
}
