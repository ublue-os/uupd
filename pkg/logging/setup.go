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

// CommandOutputLevel returns the level that command output should be logged at.
// JSON consumers rely on these records, so they are promoted to info level when
// the logger emits JSON (the default log level is info). Plain text logging
// keeps them at the requested level, usually debug.
func CommandOutputLevel(logger *slog.Logger, level slog.Level) slog.Level {
	if level == slog.LevelDebug && isJSONLogger(logger) {
		return slog.LevelInfo
	}
	return level
}

func isJSONLogger(logger *slog.Logger) bool {
	if logger == nil {
		return false
	}
	_, ok := logger.Handler().(*slog.JSONHandler)
	return ok
}

func SetupAppLogger(writer *os.File, logLevel slog.Leveler, verbose bool) slog.Handler {
	if verbose {
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: logLevel,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		})
	}
	return NewUserHandler(&slog.HandlerOptions{Level: logLevel})
}
