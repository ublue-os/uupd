package percent

import (
	"log/slog"
)

type TrackerMessage struct {
	Title       string
	Description string
}

func ReportStatusChange(tracker *Incrementer, message TrackerMessage) {
	slog.Info("Updating",
		slog.String("title", message.Title),
		slog.String("description", message.Description),
		slog.Int("progress", tracker.CurrentStep()),
		slog.Int("total", tracker.MaxIncrements),
	)
}

func ResetOscProgress() {
	// OSC escape sequence to reset all previous OSC progress hints to 0%.
	// Documentation is on https://conemu.github.io/en/AnsiEscapeCodes.html#ConEmu_specific_OSC
	print("\033]9;4;0\a")
}
