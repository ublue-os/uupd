package logging

import (
	"io"
	"log/slog"
)

func NewMuteLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
