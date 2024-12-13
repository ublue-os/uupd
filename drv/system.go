package drv

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

type bootcStatus struct {
	Status struct {
		Booted struct {
			Incompatible bool `json:"incompatible"`
			Image        struct {
				Timestamp string `json:"timestamp"`
			} `json:"image"`
		} `json:"booted"`
		Staged struct {
			Incompatible bool `json:"incompatible"`
			Image        struct {
				Timestamp string `json:"timestamp"`
			} `json:"image"`
		}
	} `json:"status"`
}

// Workaround interface to decouple individual drivers
// (TODO: Remove this whenever rpm-ostree driver gets deprecated)
type SystemUpdateDriver interface {
	Steps() int
	Outdated() (bool, error)
	UpdateAvailable() (bool, error)
	Check() (bool, error)
	Update() (*[]CommandOutput, error)
}

type SystemUpdater struct {
	Config     DriverConfiguration
	BinaryPath string
}

func (dr SystemUpdater) Outdated() (bool, error) {
	if dr.Config.DryRun {
		return false, nil
	}
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	var timestamp time.Time
	cmd := exec.Command(dr.BinaryPath, "status", "--format=json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	var status bootcStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return false, err
	}
	timestamp, err = time.Parse(time.RFC3339Nano, status.Status.Booted.Image.Timestamp)
	if err != nil {
		return false, nil
	}
	return timestamp.Before(oneMonthAgo), nil
}

func (dr SystemUpdater) Update() (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}
	var cmd *exec.Cmd
	binaryPath := dr.BinaryPath
	cli := []string{binaryPath, "upgrade"}
	cmd = exec.Command(cli[0], cli[1:]...)
	out, err := cmd.CombinedOutput()
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("System update")
	}
	finalOutput = append(finalOutput, *tmpout)
	return &finalOutput, err
}

func (dr SystemUpdater) UpdateAvailable() (bool, error) {
	cmd := exec.Command(dr.BinaryPath, "upgrade", "--check")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, err
	}
	return !strings.Contains(string(out), "No changes in:"), nil
}

func (up SystemUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up SystemUpdater) New(config UpdaterInitConfiguration) (SystemUpdater, error) {
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "System Updates",
		Enabled:     !config.Ci,
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}

	if up.Config.DryRun {
		return up, nil
	}

	bootcBinaryPath, exists := up.Config.Environment["UUPD_BOOTC_BINARY"]
	if !exists || bootcBinaryPath == "" {
		up.BinaryPath = "/usr/bin/bootc"
	} else {
		up.BinaryPath = bootcBinaryPath
	}

	return up, nil
}

func (up SystemUpdater) Check() (bool, error) {
	if up.Config.DryRun {
		return true, nil
	}

	return up.UpdateAvailable()
}
