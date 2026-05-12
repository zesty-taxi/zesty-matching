package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zesty-taxi/zesty-matching/internal/adapters/kafka_producer"
	locationclient "github.com/zesty-taxi/zesty-matching/internal/adapters/location"
	redisclient "github.com/zesty-taxi/zesty-matching/internal/adapters/redis"
	"github.com/zesty-taxi/zesty-matching/internal/config"
	"github.com/zesty-taxi/zesty-matching/internal/controller"
	"github.com/zesty-taxi/zesty-matching/internal/usecase"
	"github.com/zesty-taxi/zesty-matching/pkg/logger"
	"github.com/zesty-taxi/zesty-matching/pkg/metrics"
	customotel "github.com/zesty-taxi/zesty-matching/pkg/otel"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("config error")
	}

	logger.SetupGlobalLogger(cfg.Telemetry.ServiceName, cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// OTel — трейсинг + метрики
	_, otelShutdown, err := customotel.Init(ctx, cfg.Telemetry)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init otel")
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("otel shutdown error")
		}
	}()

	// Prometheus метрики
	m, err := metrics.New(cfg.Telemetry.ServiceName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create metrics")
	}

	// Redis
	cache, err := redisclient.NewClient(
		cfg.Redis.Address(),
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}
	defer func() {
		if err := cache.Close(); err != nil {
			log.Error().Err(err).Msg("redis close error")
		}
	}()

	// Location Service HTTP клиент
	locationClient := locationclient.NewClient(cfg.LocationBaseURL)

	// Kafka Producer
	kafkaProducer := kafka_producer.NewProducer(cfg.Kafka.Brokers)
	defer kafkaProducer.Close()

	// UseCase
	uc := usecase.New(cache, locationClient, kafkaProducer)

	// Kafka Consumer
	consumer := controller.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, uc, kafkaProducer)
	consumer.Start(ctx)
	defer consumer.Close()

	// HTTP сервер
	handler := controller.New()
	httpServer := http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler.LoadRoutes(*cfg, m),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().
			Str("port", strconv.Itoa(cfg.Port)).
			Str("env", cfg.Telemetry.Environment).
			Msg("matching service started")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server error")
		}
	}()

	// Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-signalCh

	log.Info().Str("signal", sig.String()).Msg("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("http shutdown error")
	}

	log.Info().Msg("bye bye!")
}
