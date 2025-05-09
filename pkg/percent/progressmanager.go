package percent

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/text"
	"log/slog"
	"math"
	"time"
)

type Incrementer struct {
	DoneIncrements    int
	MaxIncrements     int
	IncrementProgress float64
	// Controls the OSC progress escape code, as well as the actual cmdline progressbar
	OscEnabled     bool
	ProgressWriter progress.Writer
	Tracker        *progress.Tracker
}

// type StepTracker struct {
// 	pw      *progress.Writer
// 	tracker progress.Tracker
// }

var CuteColors = progress.StyleColors{
	Message: text.Colors{text.FgWhite},
	Error:   text.Colors{text.FgRed},
	Percent: text.Colors{text.FgHiBlue},
	Pinned:  text.Colors{text.BgHiBlack, text.FgWhite, text.Bold},
	Stats:   text.Colors{text.FgHiBlack},
	Time:    text.Colors{text.FgBlue},
	Tracker: text.Colors{text.FgHiBlue},
	Value:   text.Colors{text.FgBlue},
	Speed:   text.Colors{text.FgBlue},
}

func NewProgressWriter() progress.Writer {
	pw := progress.NewWriter()
	pw.SetTrackerLength(25)
	pw.Style().Visibility.TrackerOverall = true
	pw.Style().Visibility.Time = true
	pw.Style().Visibility.Tracker = true
	pw.Style().Visibility.Value = true
	pw.SetMessageLength(32)
	pw.SetSortBy(progress.SortByPercentDsc)
	pw.SetStyle(progress.StyleBlocks)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * 100)
	pw.Style().Options.PercentFormat = "%4.1f%%"
	pw.Style().Colors = CuteColors
	return pw
}

func (it *Incrementer) ReportStatusChange(title string, description string) {
	if !it.OscEnabled {
		slog.Info("Updating",
			slog.String("title", title),
			slog.String("description", description),
			slog.Int("progress", it.CurrentStep()),
			slog.Int("total", it.MaxIncrements),
			slog.Float64("step_progress", it.IncrementProgress),
			slog.Float64("overall", it.OverallPercent()),
		)
	}

	percentage := it.OverallPercent()
	fmt.Printf("\033]9;4;1;%d\a", int(percentage))
	finalMessage := fmt.Sprintf("Updating %s (%s)", title, description)
	it.ProgressWriter.SetMessageLength(len(finalMessage))
	it.Tracker.UpdateMessage(finalMessage)
	it.Tracker.SetValue(int64(it.OverallPercent()))
}

func ResetOscProgress() {
	// OSC escape sequence to reset all previous OSC progress hints to 0%.
	// Documentation is on https://conemu.github.io/en/AnsiEscapeCodes.html#ConEmu_specific_OSC
	print("\033]9;4;0\a")
}

func NewIncrementer(oscEnabled bool, max int) Incrementer {
	var pw progress.Writer
	var tracker progress.Tracker
	// susRef := &tracker
	if oscEnabled {
		pw = NewProgressWriter()
		tracker = progress.Tracker{
			Message: "Updating",
			Units:   progress.UnitsDefault,
			Total:   100,
		}
		pw.AppendTracker(&tracker)
	}
	return Incrementer{
		0,
		max,
		0.0,
		oscEnabled,
		pw,
		&tracker,
	}
}

func (it *Incrementer) IncrementSection(err error) {
	it.IncrementProgress = 0.0
	if int64(it.DoneIncrements)+int64(1) > int64(it.MaxIncrements) {
		return
	}
	it.DoneIncrements += 1
}

func (it *Incrementer) OverallPercent() float64 {
	steps := ((float64(it.CurrentStep()) + it.IncrementProgress) / float64(it.MaxIncrements)) * 100.0
	return math.Round(steps)
}

func (it *Incrementer) SectionPercent(percent float64) {
	it.IncrementProgress = percent
}

func (it *Incrementer) CurrentStep() int {
	return it.DoneIncrements
}
