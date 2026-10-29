package logging

import (
	"io"
	"log/slog"
	"testing"
)

func TestCommandOutputLevel(t *testing.T) {
	t.Parallel()

	jsonLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	textLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cases := []struct {
		description string
		logger      *slog.Logger
		level       slog.Level
		expected    slog.Level
	}{
		{"json logger promotes debug output", jsonLogger, slog.LevelDebug, slog.LevelInfo},
		{"json logger keeps info output", jsonLogger, slog.LevelInfo, slog.LevelInfo},
		{"text logger keeps debug output", textLogger, slog.LevelDebug, slog.LevelDebug},
		{"text logger keeps info output", textLogger, slog.LevelInfo, slog.LevelInfo},
		{"nil logger keeps debug output", nil, slog.LevelDebug, slog.LevelDebug},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			if got := CommandOutputLevel(testCase.logger, testCase.level); got != testCase.expected {
				t.Fatalf("CommandOutputLevel() = %v, expected %v", got, testCase.expected)
			}
		})
	}
}
