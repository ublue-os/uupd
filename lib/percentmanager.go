package lib

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/text"
)

type Incrementer struct {
	doneIncrements int
	MaxIncrements  int
}

type IncrementTracker struct {
	Tracker     *progress.Tracker
	incrementer *Incrementer
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

func setColorsAccent(colors *progress.StyleColors, accent string) {
	highlightColor := text.Colors{text.FgHiBlue}
	lowColor := text.Colors{text.FgBlue}

	switch accent {
	case "blue":
		highlightColor = text.Colors{text.FgHiBlue}
		lowColor = text.Colors{text.FgBlue}
	case "teal":
		highlightColor = text.Colors{text.FgHiCyan}
		lowColor = text.Colors{text.FgCyan}
	case "green":
		highlightColor = text.Colors{text.FgHiGreen}
		lowColor = text.Colors{text.FgGreen}
	case "yellow":
		highlightColor = text.Colors{text.FgHiYellow}
		lowColor = text.Colors{text.FgYellow}
	case "orange":
		highlightColor = text.Colors{text.FgHiYellow}
		lowColor = text.Colors{text.FgYellow}
	case "red":
		highlightColor = text.Colors{text.FgHiRed}
		lowColor = text.Colors{text.FgRed}
	case "pink":
		highlightColor = text.Colors{text.FgHiMagenta}
		lowColor = text.Colors{text.FgMagenta}
	case "purple":
		highlightColor = text.Colors{text.FgHiMagenta}
		lowColor = text.Colors{text.FgMagenta}
	case "slate":
		highlightColor = text.Colors{text.FgHiWhite}
		lowColor = text.Colors{text.FgWhite}
	}

	colors.Percent = highlightColor
	colors.Tracker = highlightColor
	colors.Time = lowColor
	colors.Value = lowColor
	colors.Speed = lowColor
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

	colorsSet := CuteColors

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
		cli := []string{"gsettings", "get", "org.gnome.desktop.interface", "accent-color"}
		out, err := RunUID(targetUser, cli, nil)
		accent := strings.TrimSpace(strings.ReplaceAll(string(out), "'", ""))
		if err == nil {
			setColorsAccent(&colorsSet, accent)
		}
	}
	pw.Style().Colors = colorsSet
	return pw
}

func NewIncrementTracker(tracker *progress.Tracker, max_increments int) *IncrementTracker {
	return &IncrementTracker{
		Tracker:     tracker,
		incrementer: &Incrementer{MaxIncrements: max_increments},
	}
}

type TrackerMessage struct {
	Title       string
	Description string
}

func ChangeTrackerMessageFancy(writer progress.Writer, tracker *IncrementTracker, progress bool, message TrackerMessage) {
	if !progress {
		slog.Info("Updating",
			slog.String("title", message.Title),
			slog.String("description", message.Description),
			slog.Int64("progress", tracker.Tracker.Value()),
			slog.Int64("total", tracker.Tracker.Total),
		)
		return
	}
	percentage := math.Round((float64(tracker.Tracker.Value()) / float64(tracker.Tracker.Total)) * 100)
	fmt.Printf("\033]9;4;1;%d\a", int(percentage))
	finalMessage := fmt.Sprintf("Updating %s (%s)", message.Description, message.Title)
	writer.SetMessageLength(len(finalMessage))
	tracker.Tracker.UpdateMessage(finalMessage)
}

func (it *IncrementTracker) IncrementSection(err error) {
	var increment_step float64
	if it.incrementer.doneIncrements == 0 {
		increment_step = 1
	} else {
		increment_step = float64(it.Tracker.Total / int64(it.incrementer.MaxIncrements))
	}
	if err == nil {
		it.Tracker.Increment(int64(increment_step))
	} else {
		it.Tracker.IncrementWithError(int64(increment_step))
	}
	it.incrementer.doneIncrements++
}

func (it *IncrementTracker) CurrentStep() int {
	return it.incrementer.doneIncrements
}

func ResetOscProgress() {
	// OSC escape sequence to reset all previous OSC progress hints to 0%.
	// Documentation is on https://conemu.github.io/en/AnsiEscapeCodes.html#ConEmu_specific_OSC
	print("\033]9;4;0\a")
}
