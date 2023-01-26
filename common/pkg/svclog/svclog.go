package svclog

import (
	"github.com/rs/zerolog"
	"os"
	"time"
)

func Service(log zerolog.Logger, srv string) zerolog.Logger {
	return log.With().Str("service", srv).Logger()
}

func NewLogger() zerolog.Logger {
	w := zerolog.ConsoleWriter{Out: os.Stderr,
		TimeFormat: time.StampNano}
	return zerolog.New(w).Level(zerolog.DebugLevel).With().Timestamp().Logger()
}
