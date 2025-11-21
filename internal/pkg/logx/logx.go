/*
Package logx provides a structured logging wrapper based on zerolog.

It is responsible for initializing the global logger, configuring the output format
(JSON or console) based on the environment, and providing unified helper functions
for logging levels like Info, Warn, Error, and Fatal.
*/
package logx

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitGlobalLogger initializes the global zerolog instance.
// It configures the log level and output format based on the isDevelopment parameter:
// Development: Debug level, uses ConsoleWriter (colored/human-readable format).
// Production: Info level, uses standard JSON format.
// All logs include a Unix timestamp and caller information.
func InitGlobalLogger(isDevelopment bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	if isDevelopment {
		logger = logger.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			NoColor:    false,
			TimeFormat: time.RFC3339,
		})
		logger = logger.Level(zerolog.DebugLevel)
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}

	log.Logger = logger.With().Caller().Logger()
}

// Logger returns a pointer to the global zerolog.Logger instance.
func Logger() *zerolog.Logger {
	return &log.Logger
}

// checkFields validates that the variadic fields parameter has an even number (key-value pairs).
// If the count is odd, it logs a warning and returns nil to prevent zerolog from panicking.
func checkFields(level string, fields []any) []any {
	if len(fields)%2 != 0 {
		Logger().Warn().
			Int("fields_count", len(fields)).
			Str("log_level", level).
			Msgf("Logx call (%s) received odd number of fields: %v. Fields ignored.", level, fields)
		return nil
	}
	return fields
}

// Info records a log message at the Info level.
// It accepts a formatted message string and optional key-value field list.
func Info(msg string, fields ...any) {
	fields = checkFields("Info", fields)

	Logger().Info().
		Fields(fields).
		CallerSkipFrame(1).
		Msg(msg)
}

// Warn records a log message at the Warn level.
// It accepts a formatted message string and optional key-value field list.
func Warn(msg string, fields ...any) {
	fields = checkFields("Warn", fields)

	Logger().Warn().
		Fields(fields).
		CallerSkipFrame(1).
		Msg(msg)
}

// Error records a log message at the Error level.
// It accepts an error object, a formatted message string, and an optional key-value field list.
func Error(err error, msg string, fields ...any) {
	fields = checkFields("Error", fields)

	Logger().Error().
		Err(err).
		Fields(fields).
		CallerSkipFrame(1).
		Msg(msg)
}

// Fatal records a log message at the Fatal level and then calls os.Exit(1) to terminate the program.
// It accepts an error object, a formatted message string, and an optional key-value field list.
func Fatal(err error, msg string, fields ...any) {
	fields = checkFields("Fatal", fields)

	Logger().Fatal().
		Err(err).
		Fields(fields).
		CallerSkipFrame(1).
		Msg(msg)
}
