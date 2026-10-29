package system_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/system"
	appLogging "github.com/ublue-os/uupd/pkg/logging"
)

func InitBaseConfig() system.SystemUpdater {
	var initConfiguration = generic.UpdaterInitConfiguration{
		DryRun:      true,
		Ci:          false,
		Verbose:     false,
		Environment: nil,
		Logger:      appLogging.NewMuteLogger(),
	}
	driv, _ := system.SystemUpdater{}.New(initConfiguration)
	return driv
}

func TestProperSteps(t *testing.T) {
	systemUpdater := InitBaseConfig()
	systemUpdater.Config.Enabled = false

	if systemUpdater.Steps() != 0 {
		t.Fatalf("Expected no steps when module is disabled")
	}

	systemUpdater.Config.Enabled = true
	if systemUpdater.Steps() == 0 {
		t.Fatalf("Expected steps to be added")
	}
}

func TestUpdateDryRunDoesNotExecuteBootc(t *testing.T) {
	systemUpdater := InitBaseConfig()
	// Executing this fake updater would fail. A dry run must not execute it.
	systemUpdater.BinaryPath = writeFakeBinary(t, "unexpected execution", 77)
	output, err := systemUpdater.Update(nil)
	if err != nil {
		t.Fatalf("Dry run executed bootc: %v", err)
	}
	if output == nil || len(*output) != 0 {
		t.Fatalf("Dry run should return empty command output, got %v", output)
	}
}

func writeFakeBinary(t *testing.T, output string, code int) string {
	t.Helper()
	f, err := os.CreateTemp("", "fake-binary-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	script := "#!/bin/sh\n"
	if output != "" {
		script += fmt.Sprintf("cat <<'EOF'\n%s\nEOF\n", output)
	}
	script += fmt.Sprintf("exit %d\n", code)

	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestCheckDryRun(t *testing.T) {
	updater := InitBaseConfig()
	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error in dry-run, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected update to be available in dry-run mode")
	}
}

func TestCheckNoUpdates(t *testing.T) {
	binary := writeFakeBinary(t, "No changes in: docker://localhost/test", 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = binary

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if updateAvailable {
		t.Fatalf("Expected no updates, got updateAvailable=true")
	}
}

func TestCheckUpdatesAvailable(t *testing.T) {
	binary := writeFakeBinary(t, "Update available for: docker://localhost/test", 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = binary

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected updates to be available")
	}
}

func TestCheckErrorReturnsTrue(t *testing.T) {
	binary := writeFakeBinary(t, "", 1)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = binary

	updateAvailable, err := updater.Check()
	if err == nil {
		t.Fatalf("Expected error when bootc command fails")
	}
	if !updateAvailable {
		t.Fatalf("Expected updateAvailable=true on error (safe default)")
	}
}
