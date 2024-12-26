package percent

import (
	"fmt"
	"log/slog"
	"math"
)

func (tracker Incrementer) ReportStatusChange(title string, description string) {
	if tracker.OscEnabled {
		percentage := math.Round((float64(tracker.CurrentStep()) / float64(tracker.MaxIncrements)) * 100)
		fmt.Printf("\033]9;4;1;%d\a", int(percentage))
	}

	slog.Info("Updating",
		slog.String("title", title),
		slog.String("description", description),
		slog.Int("progress", tracker.CurrentStep()),
		slog.Int("total", tracker.MaxIncrements),
	)
}

func ResetOscProgress() {
	// OSC escape sequence to reset all previous OSC progress hints to 0%.
	// Documentation is on https://conemu.github.io/en/AnsiEscapeCodes.html#ConEmu_specific_OSC
	print("\033]9;4;0\a")
}
