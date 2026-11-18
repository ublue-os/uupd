package firmware_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ublue-os/uupd/drv/firmware"
	"github.com/ublue-os/uupd/drv/generic"
	appConfig "github.com/ublue-os/uupd/pkg/config"
	appLogging "github.com/ublue-os/uupd/pkg/logging"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

const updatesJSON = `[
	{
		"DeviceId": "device-1",
		"Name": "System BIOS",
		"Version": "1.2.3",
		"Update": {
			"UpdateId": "update-1",
			"Version": "1.2.4"
		}
	},
	{
		"DeviceId": "device-2",
		"Name": "SSD",
		"Version": "2.0"
	}
]`

func TestParseUpdates(t *testing.T) {
	updates, err := firmware.ParseUpdates([]byte(updatesJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	update := updates[0]
	if update.DeviceID != "device-1" || update.Name != "System BIOS" {
		t.Fatalf("unexpected device: %+v", update)
	}
	if update.CurrentVersion != "1.2.3" || update.NewVersion != "1.2.4" {
		t.Fatalf("unexpected versions: %+v", update)
	}
}

func TestParseUpdatesFallsBackToUpdateName(t *testing.T) {
	data := []byte(`[{"DeviceId":"d","Update":{"UpdateId":"u","Name":"2.0"}}]`)
	updates, err := firmware.ParseUpdates(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 || updates[0].NewVersion != "2.0" {
		t.Fatalf("expected fallback to update name, got %+v", updates)
	}
}

func TestParseUpdatesNoUpdates(t *testing.T) {
	for _, data := range []string{
		`[]`,
		`[{"DeviceId":"d","Name":"No update"}]`,
		`[{"DeviceId":"d","Update":{}}]`,
	} {
		updates, err := firmware.ParseUpdates([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", data, err)
		}
		if len(updates) != 0 {
			t.Fatalf("expected no updates for %q, got %+v", data, updates)
		}
	}
}

func TestParseUpdatesInvalidJSON(t *testing.T) {
	if _, err := firmware.ParseUpdates([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestFormatUpdates(t *testing.T) {
	updates, _ := firmware.ParseUpdates([]byte(updatesJSON))
	body := firmware.FormatUpdates(updates)
	for _, want := range []string{"System BIOS", "1.2.3 -> 1.2.4", "fwupdmgr update"} {
		if !strings.Contains(body, want) {
			t.Fatalf("notification body %q does not contain %q", body, want)
		}
	}
	if strings.Count(body, "firmware updates available") != 0 {
		t.Fatalf("expected singular notification for one update, got %q", body)
	}
}

func TestNewDefaultsToDisabled(t *testing.T) {
	initConfig := generic.UpdaterInitConfiguration{
		DryRun: true,
		Logger: appLogging.NewMuteLogger(),
	}
	updater, err := firmware.FirmwareUpdater{}.New(initConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updater.Config.Enabled {
		t.Fatal("firmware module must be disabled by default")
	}
	if updater.Steps() != 0 {
		t.Fatal("disabled firmware module must not add steps")
	}
}

func TestSetPolicy(t *testing.T) {
	initConfig := generic.UpdaterInitConfiguration{
		DryRun: true,
		Logger: appLogging.NewMuteLogger(),
	}
	updater, err := firmware.FirmwareUpdater{}.New(initConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updater.SetPolicy(appConfig.FirmwarePolicyNotifyOnly)
	if !updater.Config.Enabled || updater.Steps() != 1 {
		t.Fatal("notify-only policy must enable the module")
	}

	updater.SetPolicy(appConfig.FirmwarePolicyDisabled)
	if updater.Config.Enabled || updater.Steps() != 0 {
		t.Fatal("disabled policy must disable the module")
	}
}

// withFwupdmgr points the firmware module at a fake fwupdmgr script that
// prints the supplied output, then restores the real configuration.
func withFwupdmgr(t *testing.T, output string) firmware.FirmwareUpdater {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fwupdmgr")
	script := "#!/bin/sh\nprintf '%s' '" + strings.ReplaceAll(output, "'", "'\\''") + "'\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed writing fake fwupdmgr: %v", err)
	}

	t.Setenv("UUPD_FIRMWDMGR_BINARY", path)
	t.Cleanup(func() { _ = appConfig.InitConfig(appConfig.DEFAULT_PATH) })
	if err := appConfig.InitConfig(appConfig.DEFAULT_PATH); err != nil {
		t.Fatalf("unable to init config: %v", err)
	}

	initConfig := generic.UpdaterInitConfiguration{
		DryRun: false,
		Logger: appLogging.NewMuteLogger(),
	}
	updater, err := firmware.FirmwareUpdater{}.New(initConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updater.SetPolicy(appConfig.FirmwarePolicyNotifyOnly)
	return updater
}

func TestUpdateWithFakeFwupdmgr(t *testing.T) {
	updater := withFwupdmgr(t, updatesJSON)
	tracker := percent.NewIncrementer(false, 1)

	outputs, err := updater.Update(&tracker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*outputs) != 1 || !strings.Contains((*outputs)[0].Stdout, "System BIOS") {
		t.Fatalf("expected firmware update details, got %#v", *outputs)
	}

	available, err := updater.Check()
	if err != nil || !available {
		t.Fatalf("expected Check to report available updates, got available=%v err=%v", available, err)
	}
}

func TestUpdateNoUpdatesWithFakeFwupdmgr(t *testing.T) {
	updater := withFwupdmgr(t, `[]`)
	tracker := percent.NewIncrementer(false, 1)

	outputs, err := updater.Update(&tracker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*outputs) != 1 || !strings.Contains((*outputs)[0].Stdout, "No firmware updates available") {
		t.Fatalf("expected a no-updates status, got %#v", *outputs)
	}
}

func TestUpdateFwupdmgrFailureIsBestEffort(t *testing.T) {
	initConfig := generic.UpdaterInitConfiguration{
		DryRun: false,
		Logger: appLogging.NewMuteLogger(),
	}
	updater, err := firmware.FirmwareUpdater{}.New(initConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updater.SetPolicy(appConfig.FirmwarePolicyNotifyOnly)
	updater.SetBinaryPath("/nonexistent/fwupdmgr")

	tracker := percent.NewIncrementer(false, 1)
	outputs, err := updater.Update(&tracker)
	if err != nil {
		t.Fatalf("fwupdmgr failure must not fail the module: %v", err)
	}
	if len(*outputs) != 1 || (*outputs)[0].Failure {
		t.Fatalf("expected a non-failing status output, got %#v", *outputs)
	}
}

func TestSetUsers(t *testing.T) {
	initConfig := generic.UpdaterInitConfiguration{
		DryRun: true,
		Logger: appLogging.NewMuteLogger(),
	}
	updater, err := firmware.FirmwareUpdater{}.New(initConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updater.SetUsers([]session.User{{UID: 1000, Name: "test"}})
	if updater.Steps() != 0 {
		t.Fatal("users should not add steps for the firmware module")
	}
}
