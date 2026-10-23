package cmd

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os/exec"
	"testing"

	"github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/pkg/percent"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buffer := &bytes.Buffer{}
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buffer, nil)))
	return buffer
}

func decodeLogs(t *testing.T, buffer *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(buffer)
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatalf("Failed decoding log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func findRecord(records []map[string]any, status string) map[string]any {
	for _, record := range records {
		if record["status"] == status {
			return record
		}
	}
	return nil
}

func TestRunModuleStructuredOutput(t *testing.T) {
	buffer := captureLogs(t)
	tracker := percent.NewIncrementer(false, 1)
	outputs := []generic.CommandOutput{}

	update := func(*percent.Incrementer) (*[]generic.CommandOutput, error) {
		commandOutput := generic.CommandOutput{}.New([]byte("command output"), nil)
		commandOutput.Context = "Test Update"
		commandOutput.Cli = []string{"test", "update"}
		out := []generic.CommandOutput{*commandOutput}
		return &out, nil
	}

	tracker.ReportStatusChange("Test", "Test description")
	tracker.ReportStatusChange("Test", "Still testing")
	if err := runModule("Test", &tracker, &outputs, update); err != nil {
		t.Fatalf("runModule returned an error: %v", err)
	}
	if len(outputs) != 1 || outputs[0].Module != "test" {
		t.Fatalf("Expected output tagged with module \"test\", got %#v", outputs)
	}

	records := decodeLogs(t, buffer)
	for _, record := range records {
		if record["module"] != "test" {
			t.Fatalf("Expected module \"test\" on every record, got %v", record)
		}
	}

	starting := findRecord(records, "starting")
	if starting == nil {
		t.Fatalf("Expected a starting record, got %v", records)
	}
	if findRecord(records, "running") == nil {
		t.Fatalf("Expected a running record, got %v", records)
	}
	finished := findRecord(records, "finished")
	if finished == nil {
		t.Fatalf("Expected a finished record, got %v", records)
	}
	if finished["exit_code"] != float64(0) {
		t.Fatalf("Expected exit code 0, got %v", finished["exit_code"])
	}
}

func TestRunModuleReportsCommandExitCode(t *testing.T) {
	buffer := captureLogs(t)
	tracker := percent.NewIncrementer(false, 1)
	outputs := []generic.CommandOutput{}

	update := func(*percent.Incrementer) (*[]generic.CommandOutput, error) {
		err := exec.Command("/bin/sh", "-c", "exit 3").Run()
		commandOutput := generic.CommandOutput{}.New([]byte("failed output"), err)
		commandOutput.Context = "Test Update"
		commandOutput.Cli = []string{"test", "update"}
		out := []generic.CommandOutput{*commandOutput}
		return &out, err
	}

	if err := runModule("Test", &tracker, &outputs, update); err == nil {
		t.Fatal("Expected runModule to return the command error")
	}

	finished := findRecord(decodeLogs(t, buffer), "finished")
	if finished == nil {
		t.Fatal("Expected a finished record")
	}
	if finished["exit_code"] != float64(3) {
		t.Fatalf("Expected exit code 3, got %v", finished["exit_code"])
	}
	if len(outputs) != 1 || outputs[0].ExitCode != 3 {
		t.Fatalf("Expected output with exit code 3, got %#v", outputs)
	}
}
