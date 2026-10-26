package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestLogModuleFailureIncludesContext(t *testing.T) {
	buffer := captureLogs(t)

	logModuleFailure(generic.CommandOutput{
		Module:   "flatpak",
		Context:  "Apps for User: alice",
		Failure:  true,
		ExitCode: 1,
		Stdout:   "error: Unable to update application",
		Cli:      []string{"/usr/bin/flatpak", "update", "-y", "--noninteractive"},
	})

	records := decodeLogs(t, buffer)
	if len(records) != 1 {
		t.Fatalf("Expected one failure record, got %v", records)
	}

	record := records[0]
	if record["module"] != "flatpak" {
		t.Fatalf("Expected module \"flatpak\", got %v", record)
	}
	if record["context"] != "Apps for User: alice" {
		t.Fatalf("Expected the context to identify the failing update, got %v", record)
	}
	if record["status"] != "failed" {
		t.Fatalf("Expected status \"failed\", got %v", record)
	}
	if record["exit_code"] != float64(1) {
		t.Fatalf("Expected exit code 1, got %v", record)
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

type fakeSystemDriver struct {
	checkResult bool
	checkErr    error
	checkCalls  int
}

func (driver *fakeSystemDriver) Steps() int {
	return 1
}

func (driver *fakeSystemDriver) Outdated() (bool, error) {
	return false, nil
}

func (driver *fakeSystemDriver) Check() (bool, error) {
	driver.checkCalls++
	return driver.checkResult, driver.checkErr
}

func (driver *fakeSystemDriver) Update(*percent.Incrementer) (*[]generic.CommandOutput, error) {
	return &[]generic.CommandOutput{}, nil
}

func TestPlanSystemModuleRunsWhenCheckReportsNoUpdate(t *testing.T) {
	driver := &fakeSystemDriver{checkResult: false}

	run, updateAvailable := planSystemModule(true, false, driver, false)

	if !run {
		t.Fatal("Expected the system module to run even when the update check reports no updates")
	}
	if updateAvailable {
		t.Fatal("Expected update_available to reflect the negative check result")
	}
	if driver.checkCalls != 1 {
		t.Fatalf("Expected the update check to run once, got %d", driver.checkCalls)
	}
}

func TestPlanSystemModuleRespectsDisabledModule(t *testing.T) {
	driver := &fakeSystemDriver{checkResult: true}

	run, _ := planSystemModule(true, true, driver, false)

	if run {
		t.Fatal("Expected the system module to be skipped when it is disabled")
	}
	if driver.checkCalls != 0 {
		t.Fatalf("Expected no update check when the module is disabled, got %d", driver.checkCalls)
	}
}

func TestPlanSystemModuleRespectsDisabledDriver(t *testing.T) {
	driver := &fakeSystemDriver{checkResult: true}

	run, _ := planSystemModule(false, false, driver, false)

	if run {
		t.Fatal("Expected the system module to be skipped when the driver is disabled")
	}
	if driver.checkCalls != 0 {
		t.Fatalf("Expected no update check when the driver is disabled, got %d", driver.checkCalls)
	}
}

func TestPlanSystemModuleCheckErrorStillRuns(t *testing.T) {
	buffer := captureLogs(t)
	driver := &fakeSystemDriver{checkErr: errors.New("check failed")}

	run, updateAvailable := planSystemModule(true, false, driver, false)

	if !run {
		t.Fatal("Expected the system module to run after a failed update check")
	}
	if !updateAvailable {
		t.Fatal("Expected a failed update check to assume an update may be available")
	}
	records := decodeLogs(t, buffer)
	if len(records) != 1 || records[0]["level"] != "WARN" {
		t.Fatalf("Expected a warning about the failed check, got %v", records)
	}
}

func TestPlanSystemModuleForceSkipsCheck(t *testing.T) {
	driver := &fakeSystemDriver{checkResult: false}

	run, updateAvailable := planSystemModule(true, false, driver, true)

	if !run || !updateAvailable {
		t.Fatalf("Expected forced system updates to run with update_available=true, got run=%v update_available=%v", run, updateAvailable)
	}
	if driver.checkCalls != 0 {
		t.Fatalf("Expected --force to skip the update check, got %d calls", driver.checkCalls)
	}
}
