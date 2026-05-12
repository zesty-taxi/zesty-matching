package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/zesty-taxi/zesty-matching/pkg/db"
	customOtel "github.com/zesty-taxi/zesty-matching/pkg/otel"
)

type Config struct {
	Port            int
	LogLevel        string
	DB              db.Config
	Redis           RedisConfig
	Kafka           KafkaConfig
	Telemetry       customOtel.Config
	LocationBaseURL string
	OSRMBaseURL     string
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type KafkaConfig struct {
	Brokers       []string
	ConsumerGroup string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	viper.AutomaticEnv()

	cfg := &Config{
		Port:     viper.GetInt("SERVICE_PORT"),
		LogLevel: viper.GetString("LOG_LEVEL"),
		DB: db.Config{
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetString("DB_PORT"),
			User:     viper.GetString("DB_USER"),
			Password: viper.GetString("DB_PASSWORD"),
			Name:     viper.GetString("DB_NAME"),
			SSLMode:  viper.GetString("DB_SSLMODE"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetInt("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		Kafka: KafkaConfig{
			Brokers:       parseBrokers(viper.GetString("KAFKA_BROKERS")),
			ConsumerGroup: viper.GetString("KAFKA_CONSUMER_GROUP"),
		},
		Telemetry: customOtel.Config{
			ServiceName:    viper.GetString("SERVICE_NAME"),
			ServiceVersion: viper.GetString("SERVICE_VERSION"),
			Environment:    viper.GetString("ENVIRONMENT"),
			OTLPEndpoint:   viper.GetString("OTLP_ENDPOINT"),
		},
		LocationBaseURL: viper.GetString("LOCATION_URL"),
		OSRMBaseURL:     viper.GetString("OSRM_URL"),
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port out of range (1-65535)")
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "debug"
	}

	if cfg.DB.Host == "" {
		return nil, fmt.Errorf("DB_HOST is required")
	}
	if cfg.DB.Port == "" {
		cfg.DB.Port = "5432"
	}
	if cfg.DB.SSLMode == "" {
		cfg.DB.SSLMode = "disable"
	}

	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}

	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"localhost:9092"}
	}
	if cfg.Kafka.ConsumerGroup == "" {
		cfg.Kafka.ConsumerGroup = "ride-service"
	}

	if cfg.Telemetry.ServiceName == "" {
		cfg.Telemetry.ServiceName = "ride-service"
	}
	if cfg.Telemetry.ServiceVersion == "" {
		cfg.Telemetry.ServiceVersion = "1.0.0"
	}
	if cfg.Telemetry.Environment == "" {
		cfg.Telemetry.Environment = "local"
	}
	if cfg.Telemetry.OTLPEndpoint == "" {
		cfg.Telemetry.OTLPEndpoint = "localhost:4318"
	}

	if cfg.OSRMBaseURL == "" {
		cfg.OSRMBaseURL = "http://localhost:5000"
	}
	if cfg.LocationBaseURL == "" {
		cfg.LocationBaseURL = "http://localhost:8004"
	}

	return cfg, nil
}

func (c *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func parseBrokers(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))

	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}

	return brokers
}
