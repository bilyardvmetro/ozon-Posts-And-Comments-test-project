package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var Log zerolog.Logger

func Init() {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	Log = zerolog.New(output).With().
		Timestamp().
		Caller().
		Logger()

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}
