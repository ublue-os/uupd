// Package firmware implements the optional firmware module. It uses fwupd to
// detect available firmware updates and notifies logged-in users about them.
//
// The module never installs firmware on its own: per the design agreed in
// ublue-os/uupd#119, only notifications are supported.
package firmware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	. "github.com/ublue-os/uupd/drv/generic"
	appConfig "github.com/ublue-os/uupd/pkg/config"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

// FirmwareUpdate describes a single device with an available firmware update.
type FirmwareUpdate struct {
	DeviceID       string
	Name           string
	CurrentVersion string
	NewVersion     string
}

// fwupdDevice mirrors the subset of the JSON emitted by
// `fwupdmgr get-updates --json` that we care about. A device exposes an
// "Update" object only when an update is available for it.
type fwupdDevice struct {
	DeviceID string `json:"DeviceId"`
	Name     string `json:"Name"`
	Version  string `json:"Version"`
	Update   *struct {
		UpdateID string `json:"UpdateId"`
		Name     string `json:"Name"`
		Version  string `json:"Version"`
	} `json:"Update"`
}

// ParseUpdates parses the JSON output of `fwupdmgr get-updates --json` and
// returns the devices that have an update available.
func ParseUpdates(data []byte) ([]FirmwareUpdate, error) {
	var devices []fwupdDevice
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil, fmt.Errorf("failed parsing fwupd output: %w", err)
	}

	updates := make([]FirmwareUpdate, 0)
	for _, device := range devices {
		if device.Update == nil || device.Update.UpdateID == "" {
			continue
		}

		newVersion := device.Update.Version
		if newVersion == "" {
			newVersion = device.Update.Name
		}

		updates = append(updates, FirmwareUpdate{
			DeviceID:       device.DeviceID,
			Name:           device.Name,
			CurrentVersion: device.Version,
			NewVersion:     newVersion,
		})
	}
	return updates, nil
}

// FormatUpdates renders a short, human-readable summary of the available
// firmware updates suitable for a desktop notification.
func FormatUpdates(updates []FirmwareUpdate) string {
	parts := make([]string, 0, len(updates))
	for _, update := range updates {
		name := update.Name
		if name == "" {
			name = update.DeviceID
		}

		version := ""
		switch {
		case update.CurrentVersion != "" && update.NewVersion != "":
			version = fmt.Sprintf(" (%s -> %s)", update.CurrentVersion, update.NewVersion)
		case update.NewVersion != "":
			version = fmt.Sprintf(" (%s)", update.NewVersion)
		}
		parts = append(parts, name+version)
	}

	if len(parts) == 1 {
		return "Firmware update available for " + parts[0] + ". Run 'fwupdmgr update' to install it."
	}
	return fmt.Sprintf("%d firmware updates available: %s. Run 'fwupdmgr update' to install them.", len(parts), strings.Join(parts, ", "))
}

// FirmwareUpdater checks for firmware updates and notifies users. It is
// disabled unless the configured policy is "notify-only".
type FirmwareUpdater struct {
	Config     DriverConfiguration
	binaryPath string
	policy     string
	users      []session.User
}

func (up FirmwareUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up FirmwareUpdater) New(config UpdaterInitConfiguration) (FirmwareUpdater, error) {
	conf := appConfig.Get().Modules.Firmware

	up.Config = DriverConfiguration{
		Title:       "Firmware",
		Description: "Firmware Update Notification",
		Enabled:     appConfig.FirmwarePolicyEnabled(conf.Policy),
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.binaryPath = conf.BinaryPath
	up.policy = appConfig.NormalizeFirmwarePolicy(conf.Policy)

	return up, nil
}

// SetPolicy overrides the policy read from the configuration (used by the
// --policy-module-firmware and --disable-module-firmware CLI flags).
func (up *FirmwareUpdater) SetPolicy(policy string) {
	up.policy = appConfig.NormalizeFirmwarePolicy(policy)
	up.Config.Enabled = appConfig.FirmwarePolicyEnabled(policy)
}

func (up *FirmwareUpdater) SetUsers(users []session.User) {
	up.users = users
}

// SetBinaryPath overrides the path to the fwupdmgr binary.
func (up *FirmwareUpdater) SetBinaryPath(path string) {
	up.binaryPath = path
}

// Check reports whether firmware updates are available. The notification
// logic lives in Update so the module fits the standard module runner.
func (up FirmwareUpdater) Check() (bool, error) {
	updates, err := up.getUpdates()
	if err != nil {
		return false, err
	}
	return len(updates) > 0, nil
}

// getUpdates runs fwupdmgr and parses its output. stderr is captured
// separately so it cannot corrupt the JSON on stdout.
func (up FirmwareUpdater) getUpdates() ([]FirmwareUpdate, error) {
	cli := []string{up.binaryPath, "get-updates", "--json"}
	cmd := exec.Command(cli[0], cli[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		up.Config.Logger.Debug("fwupdmgr get-updates failed",
			slog.Any("error", err),
			slog.String("stderr", stderr.String()),
		)
		return nil, fmt.Errorf("fwupdmgr get-updates failed: %w", err)
	}

	if out := strings.TrimSpace(stdout.String()); out != "" {
		up.Config.Logger.Debug("fwupdmgr output", slog.String("output", out))
	}

	return ParseUpdates(stdout.Bytes())
}

func (up FirmwareUpdater) Update(_tracker *percent.Incrementer) (*[]CommandOutput, error) {
	finalOutput := []CommandOutput{}
	cli := []string{up.binaryPath, "get-updates", "--json"}

	if up.Config.DryRun {
		return &finalOutput, nil
	}

	updates, err := up.getUpdates()
	output := CommandOutput{
		Context: "Firmware Update Check",
		Cli:     cli,
		Module:  "firmware",
	}
	if err != nil {
		// A firmware check is best-effort: it must not fail the whole update.
		output.Stdout = err.Error()
		up.Config.Logger.Warn("Failed checking for firmware updates", slog.Any("error", err))
		finalOutput = append(finalOutput, output)
		return &finalOutput, nil
	}

	if len(updates) == 0 {
		up.Config.Logger.Debug("No firmware updates available")
		output.Stdout = "No firmware updates available"
		finalOutput = append(finalOutput, output)
		return &finalOutput, nil
	}

	body := FormatUpdates(updates)
	up.Config.Logger.Info("Firmware updates available",
		slog.Int("count", len(updates)),
		slog.String("devices", body),
		slog.String("policy", up.policy),
	)

	if appConfig.FirmwarePolicyEnabled(up.policy) {
		if err := session.Notify(up.users, "Firmware updates available", body, "normal"); err != nil {
			up.Config.Logger.Warn("Failed notifying users about firmware updates", slog.Any("error", err))
		}
	}

	output.Stdout = body
	finalOutput = append(finalOutput, output)
	return &finalOutput, nil
}
