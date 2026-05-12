package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func SetupGlobalLogger(serviceName string, level string) {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	log.Logger = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Caller().
		Str("service", serviceName).
		Logger()
}
