// Package app contains logic for running the application.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/VasySS/segoya-backend/internal/config"
	httpController "github.com/VasySS/segoya-backend/internal/controller/http"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/cloudflare"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	valkeyRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/valkey"
	"github.com/VasySS/segoya-backend/internal/infrastructure/token"
	"github.com/VasySS/segoya-backend/internal/infrastructure/transport/melody"
	"github.com/VasySS/segoya-backend/internal/usecase/auth"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/panorama"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/user"
	"github.com/VasySS/segoya-backend/pkg/captcha"
	"github.com/VasySS/segoya-backend/pkg/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Run creates all needed usecases and starts the application.
func Run(ctx context.Context, conf config.Config) error {
	closer := NewCloser()

	if err := setGlobalTracer(ctx, conf.ENV.JaegerURL); err != nil {
		return err
	}

	valkeyRepo, err := newValkeyRepo(ctx, closer, conf.ENV.ValkeyURL)
	if err != nil {
		return err
	}

	pgConnectionURL := fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		conf.ENV.PostgresUser,
		conf.ENV.PostgresPassword,
		conf.ENV.PostgresHost,
		conf.ENV.PostgresDatabase,
	)

	pgRepo, err := newPgRepo(ctx, closer, pgConnectionURL)
	if err != nil {
		return err
	}

	cloudflareS3, err := cloudflare.New(ctx, cloudflare.NewConfig(conf))
	if err != nil {
		return fmt.Errorf("failed to create cloudflare repo: %w", err)
	}

	cryptoService := crypto.NewService()
	captchaService := captcha.NewCloudflareService(
		conf.HTTPClient,
		conf.ENV.FrontendURL.String(),
		conf.ENV.CaptchaSecretKey,
	)
	tokenService := token.NewService(
		conf.ENV.JWTSecretKey,
		conf.Limits.AccessTokenTTL,
		conf.Limits.RefreshTokenTTL,
	)

	lobbyWebSocketService := melody.NewWebSocketService()
	closer.AddWithError(lobbyWebSocketService.Close)

	multiplayerWebSocketService := melody.NewWebSocketService()
	closer.AddWithError(multiplayerWebSocketService.Close)

	authUsecase := auth.NewUsecase(auth.NewConfig(conf), cryptoService, tokenService, pgRepo, valkeyRepo)
	userUsecase := user.NewUsecase(user.NewConfig(conf), pgRepo, cloudflareS3)

	panoramaUsecase := panorama.NewUsecase(panorama.NewConfig(conf), pgRepo)
	singleplayerUsecase := singleplayer.NewUsecase(singleplayer.NewConfig(conf), pgRepo, panoramaUsecase)
	multiplayerUsecase := multiplayer.NewUsecase(multiplayer.NewConfig(conf), pgRepo, panoramaUsecase)
	lobbyUsecase := lobby.NewUsecase(lobby.NewConfig(conf), cryptoService, pgRepo, valkeyRepo, multiplayerUsecase)

	r := httpController.NewRouter(
		conf,
		cryptoService,
		tokenService,
		captchaService,
		lobbyWebSocketService,
		multiplayerWebSocketService,
		authUsecase,
		userUsecase,
		lobbyUsecase,
		singleplayerUsecase,
		multiplayerUsecase,
	)

	go startHTTP(closer, r)

	<-ctx.Done()
	slog.Info("stopping server...")

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if err := closer.Close(closeCtx); err != nil { //nolint:contextcheck
		return err
	}

	return nil
}

func startHTTP(
	closer *Closer,
	r http.Handler,
) {
	srv := &http.Server{
		Addr:         ":4174",
		Handler:      r,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
		IdleTimeout:  time.Second * 10,
	}

	slog.Info("starting http server", slog.String("addr", srv.Addr))
	closer.AddWithCtx(srv.Shutdown)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("failed to start http server: %v", err)
	}
}

func newPgRepo(ctx context.Context, closer *Closer, connectionURL string) (*postgres.Repository, error) {
	pool, err := pgxpool.New(ctx, connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	slog.Info("postgres connected")
	closer.Add(pool.Close)

	txManager := postgres.NewTxManager(pool)
	pgRepo := postgres.New(txManager)

	return pgRepo, nil
}

func newValkeyRepo(ctx context.Context, closer *Closer, valkeyURL string) (*valkeyRepo.Repository, error) {
	valkeyClient, err := valkey.NewClient(valkey.MustParseURL(valkeyURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	pingCmd := valkeyClient.B().Ping().Build()
	if err := valkeyClient.Do(ctx, pingCmd).Error(); err != nil {
		return nil, fmt.Errorf("failed to ping valkey: %w", err)
	}

	slog.Info("valkey connected")
	closer.Add(valkeyClient.Close)

	return valkeyRepo.New(valkeyClient), nil
}

func setGlobalTracer(ctx context.Context, jaegerURL string) error {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("segoya-backend"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(jaegerURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create jaeger exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// metricExporter, err := otlpmetrichttp.New(ctx,
	// 	otlpmetrichttp.WithEndpointURL(jaegerURL),
	// )
	// if err != nil {
	// 	return fmt.Errorf("failed to create jaeger metric exporter: %w", err)
	// }

	// mp := metric.NewMeterProvider(
	// 	metric.WithResource(res),
	// 	metric.WithReader(metric.NewPeriodicReader(
	// 		metricExporter,
	// 		metric.WithInterval(10*time.Second),
	// 	)),
	// )
	// otel.SetMeterProvider(mp)

	return nil
}
