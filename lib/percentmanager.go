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
	percentage := math.Round((float64(tracker.Tracker.Value()) / float64(tracker.Tracker.Total)) * 100)
	fmt.Printf("\033]9;4;1;%d\a", int(percentage))
	if !progress {
		slog.Info("Updating",
			slog.String("title", message.Title),
			slog.String("description", message.Description),
			slog.Int64("progress", tracker.Tracker.Value()),
			slog.Int64("total", tracker.Tracker.Total),
		)
		return
	}
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
