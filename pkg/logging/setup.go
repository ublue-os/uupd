package logging

import (
	"log/slog"
	"os"
	"slices"
	"strings"
)

func StrToLogLevel(logLevel string) (slog.Leveler, error) {
	var logLevels = map[string]slog.Leveler{
		"debug": slog.LevelDebug,
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
	}
	var valid_stuff []string
	for key := range logLevels {
		valid_stuff = append(valid_stuff, key)
	}

	if !slices.Contains(valid_stuff, logLevel) {
		return nil, &InvalidLevelError{Message: strings.ToUpper(logLevel)}
	}

	return logLevels[logLevel], nil
}

func SetupAppLogger(writer *os.File, logLevel slog.Leveler, verbose bool) slog.Handler {
	if verbose {
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: logLevel,
		})
	}
	return NewUserHandler(&slog.HandlerOptions{Level: logLevel})
}
