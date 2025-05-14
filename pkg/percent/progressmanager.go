package percent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/ublue-os/uupd/pkg/session"
)

type Incrementer struct {
	DoneIncrements int
	MaxIncrements  int
	// Controls the OSC progress escape code, as well as the actual cmdline progressbar
	ProgressEnabled bool
	ProgressWriter  progress.Writer
	PTracker        StepTracker
}

type StepTracker struct {
	Progress float64
	Tracker  *progress.Tracker
}

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
	pw.Style().Visibility.Time = false
	pw.Style().Visibility.Tracker = true
	pw.Style().Visibility.Value = true
	pw.SetMessageLength(32)
	pw.SetSortBy(progress.SortByNone)
	pw.SetStyle(progress.StyleBlocks)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * 200)
	pw.Style().Options.PercentFormat = "%4.1f%%"
	pw.Style().Colors = CuteColors
	pw.SetAutoStop(false)

	var targetUser int
	baseUser, exists := os.LookupEnv("SUDO_UID")
	if !exists || baseUser == "" {
		targetUser = 0
	} else {
		var err error
		targetUser, err = strconv.Atoi(baseUser)
		if err != nil {
			slog.Error("Failed parsing provided user as UID", slog.String("user_value", baseUser))
			return pw
		}
	}
	if targetUser != 0 {
		var accentColorSet progress.StyleColors
		// Get accent color: https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Settings.html
		cmd := exec.Command("busctl", fmt.Sprintf("--machine=%d@", targetUser), "--user", "--json=short", "call", "org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop", "org.freedesktop.portal.Settings", "ReadOne", "ss", "org.freedesktop.appearance", "accent-color")
		out, err := session.RunLog(nil, slog.LevelDebug, cmd)
		if err != nil {
			// Erroring out here would be kinda silly because sometimes the xdg portal just doesn't exist on certain desktops, uncomment line below for debugging
			// slog.Error("Failed to get accent color", slog.Any("err", err))
			return pw
		}
		var accent Accent
		err = json.Unmarshal(out, &accent)
		if err != nil {
			slog.Error("Failed to unmarshal accent color data", slog.String("raw_data", string(out)), slog.Any("error", err))
			return pw
		}
		if len(accent.Data) == 0 {
			slog.Error("Accent data is empty")
			return pw
		}
		rawColor := accent.Data[0].Data
		highlightColor, lowColor := findClosestColor(rawColor)
		validHighlightColor := text.Colors{highlightColor}
		validLowColor := text.Colors{lowColor}
		accentColorSet.Percent = validHighlightColor
		accentColorSet.Tracker = validHighlightColor
		accentColorSet.Time = validLowColor
		accentColorSet.Value = validLowColor
		accentColorSet.Speed = validLowColor
		pw.Style().Colors = accentColorSet
	}
	return pw
}

func (it *Incrementer) ReportStatusChange(title string, description string) {
	if !it.ProgressEnabled {
		slog.Info("Updating",
			slog.String("title", title),
			slog.String("description", description),
			slog.Int("progress", it.CurrentStep()),
			slog.Int("total", it.MaxIncrements),
			slog.Float64("step_progress", it.PTracker.Progress),
			slog.Float64("overall", it.OverallPercent()),
		)
		return
	}
	// Only System updates have proper progress reporting
	if title == "System" {
		it.PTracker.Tracker.UpdateTotal(100)
	}
	percentage := it.OverallPercent()

	// OSC escape sequence to up the overall percentage
	fmt.Printf("\033]9;4;1;%d\a", int(percentage))

	finalMessage := fmt.Sprintf("Updating %s (%s) Step: [%d/%d]", title, description, it.CurrentStep()+1, it.MaxIncrements+1)

	it.ProgressWriter.SetMessageLength(len(finalMessage))
	it.PTracker.Tracker.UpdateMessage(finalMessage)
	it.PTracker.Tracker.SetValue(int64(it.PTracker.Progress))
}

func ResetOscProgress() {
	// OSC escape sequence to reset all previous OSC progress hints to 0%.
	// Documentation is on https://conemu.github.io/en/AnsiEscapeCodes.html#ConEmu_specific_OSC
	print("\033]9;4;0\a")
}

func newTracker(progressEnabled bool) StepTracker {
	var tracker progress.Tracker
	if progressEnabled {
		tracker = progress.Tracker{
			Message: "Updating",
			Units:   progress.UnitsDefault,
			Total:   0,
		}
	}
	return StepTracker{
		Progress: 0.0,
		Tracker:  &tracker,
	}
}

func NewIncrementer(progressEnabled bool, max int) Incrementer {
	var pw progress.Writer
	tracker := newTracker(progressEnabled)
	if progressEnabled {
		pw = NewProgressWriter()
		pw.AppendTracker(tracker.Tracker)
	}
	return Incrementer{
		0,
		max,
		progressEnabled,
		pw,
		tracker,
	}
}

func (it *Incrementer) IncrementSection(err error) {
	if it.ProgressEnabled {
		if err != nil {
			it.PTracker.Tracker.MarkAsErrored()
		} else {
			it.PTracker.Tracker.MarkAsDone()
		}
	}

	if int64(it.DoneIncrements+1) > int64(it.MaxIncrements) {
		return
	}
	it.DoneIncrements += 1

	if it.ProgressEnabled {
		it.PTracker = newTracker(it.ProgressEnabled)
		it.ProgressWriter.AppendTracker(it.PTracker.Tracker)
	}
}

func (it *Incrementer) OverallPercent() float64 {
	steps := ((float64(it.CurrentStep()+1) + it.PTracker.Progress/100.0) / float64(it.MaxIncrements+1)) * 100.0
	return math.Round(steps)
}

func (it *Incrementer) SectionPercent(percent float64) {
	it.PTracker.Progress = percent
}

func (it *Incrementer) CurrentStep() int {
	return it.DoneIncrements
}
