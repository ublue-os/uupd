package percent

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Time() time.Time         { return c.now }
func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func captureProgressLogs(t *testing.T, max int) (*bytes.Buffer, *Incrementer, *fakeClock) {
	t.Helper()
	buffer := &bytes.Buffer{}
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buffer, nil)))

	clock := &fakeClock{now: time.Unix(0, 0)}
	tracker := NewIncrementer(false, max)
	tracker.now = clock.Time
	return buffer, &tracker, clock
}

func decodeProgressRecords(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(buffer.Bytes()))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatalf("Failed decoding log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}

// A single bootc update can produce hundreds of progress events. None of them
// should become a log record unless something meaningful changed.
func TestReportStatusChangeThrottlesRepeatedUpdates(t *testing.T) {
	buffer, tracker, _ := captureProgressLogs(t, 3)

	logged := 0
	for i := 0; i < 500; i++ {
		tracker.SectionPercent(0)
		if tracker.ReportStatusChange("System", "Downloading") {
			logged++
		}
	}

	if logged != 1 {
		t.Fatalf("Expected 1 log record for 500 identical updates, got %d", logged)
	}
	records := decodeProgressRecords(t, buffer)
	if len(records) != 1 || records[0]["status"] != "starting" {
		t.Fatalf("Expected a single starting record, got %v", records)
	}
}

func TestReportStatusChangeLogsStageTransitions(t *testing.T) {
	buffer, tracker, _ := captureProgressLogs(t, 3)

	tracker.ReportStatusChange("System", "Downloading")
	tracker.ReportStatusChange("System", "Downloading")
	tracker.ReportStatusChange("System", "Importing")
	tracker.ReportStatusChange("System", "Importing")
	tracker.ReportStatusChange("System", "Deploying")

	records := decodeProgressRecords(t, buffer)
	if len(records) != 3 {
		t.Fatalf("Expected 3 records (one per stage), got %d: %v", len(records), records)
	}
	want := []string{"System module is starting.", "Updating System (Importing)", "Updating System (Deploying)"}
	for i, output := range want {
		if records[i]["output"] != output {
			t.Fatalf("Record %d: expected output %q, got %v", i, output, records[i]["output"])
		}
	}
}

func TestReportStatusChangeLogsModuleTransitions(t *testing.T) {
	buffer, tracker, _ := captureProgressLogs(t, 3)

	tracker.ReportStatusChange("System", "Bootc")
	tracker.ReportStatusChange("System", "Bootc")
	tracker.ReportStatusChange("Brew", "Brew")

	records := decodeProgressRecords(t, buffer)
	if len(records) != 2 {
		t.Fatalf("Expected 2 starting records, got %d: %v", len(records), records)
	}
	for _, record := range records {
		if record["status"] != "starting" {
			t.Fatalf("Expected a starting record, got %v", record)
		}
	}
}

func TestReportStatusChangeLogsAfterInterval(t *testing.T) {
	buffer, tracker, clock := captureProgressLogs(t, 3)

	tracker.ReportStatusChange("System", "Downloading")
	clock.advance(progressLogInterval - time.Second)
	if tracker.ReportStatusChange("System", "Downloading") {
		t.Fatal("Did not expect a record before the log interval elapsed")
	}
	clock.advance(time.Second)
	if !tracker.ReportStatusChange("System", "Downloading") {
		t.Fatal("Expected a record once the log interval elapsed")
	}

	if got := len(decodeProgressRecords(t, buffer)); got != 2 {
		t.Fatalf("Expected 2 records, got %d", got)
	}
}

func TestReportStatusChangeLogsAfterPercentChange(t *testing.T) {
	buffer, tracker, _ := captureProgressLogs(t, 3)

	tracker.ReportStatusChange("System", "Downloading")
	tracker.SectionPercent(5) // Overall progress ~2%, below the threshold.
	if tracker.ReportStatusChange("System", "Downloading") {
		t.Fatal("Did not expect a record for a small percent change")
	}
	tracker.SectionPercent(35) // Overall progress ~12%.
	if !tracker.ReportStatusChange("System", "Downloading") {
		t.Fatal("Expected a record after significant overall progress")
	}

	if got := len(decodeProgressRecords(t, buffer)); got != 2 {
		t.Fatalf("Expected 2 records, got %d", got)
	}
}
