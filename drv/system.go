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

// TODO: Temporary: WILL get removed at some point.
type rpmOstreeStatus struct {
	Deployments []struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"deployments"`
}

type SystemUpdater struct {
	Config          DriverConfiguration
	SystemDriver    SystemDriver
	Outdated        bool
	UpdateAvailable bool
}

type SystemVariation int

const (
	Bootc SystemVariation = iota
	RpmOstree
)

type SystemDriver struct {
	Variation           SystemVariation
	bootcBinaryPath     string
	rpmOstreeBinaryPath string
}

func (dr SystemDriver) Outdated() (bool, error) {
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	var timestamp time.Time
	switch dr.Variation {
	case Bootc:
		cmd := exec.Command(dr.bootcBinaryPath, "status", "--format=json")
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
	case RpmOstree:
		cmd := exec.Command(dr.rpmOstreeBinaryPath, "status", "--json", "--booted")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, err
		}
		var status rpmOstreeStatus
		err = json.Unmarshal(out, &status)
		if err != nil {
			return false, err
		}
		timestamp = time.Unix(status.Deployments[0].Timestamp, 0).UTC()
	}

	return timestamp.Before(oneMonthAgo), nil
}

func (dr SystemDriver) Update() (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}
	var cmd *exec.Cmd = nil
	var binaryPath string
	switch dr.Variation {
	case Bootc:
		binaryPath = dr.bootcBinaryPath
	case RpmOstree:
		binaryPath = dr.rpmOstreeBinaryPath
	}
	cli := []string{binaryPath, "upgrade"}
	cmd = exec.Command(cli[0], cli[1:]...)
	out, err := cmd.CombinedOutput()
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	tmpout.Context = "System Update"
	finalOutput = append(finalOutput, *tmpout)
	return &finalOutput, err
}

func (dr SystemDriver) UpdateAvailable() (bool, error) {
	switch dr.Variation {
	case Bootc:
		cmd := exec.Command(dr.bootcBinaryPath, "upgrade", "--check")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return true, err
		}
		return !strings.Contains(string(out), "No changes in:"), nil
	case RpmOstree:
		// This function may or may not be accurate, rpm-ostree updgrade --check has issues... https://github.com/coreos/rpm-ostree/issues/1579
		// Not worried because we will end up removing rpm-ostree from the equation soon
		cmd := exec.Command(dr.rpmOstreeBinaryPath, "upgrade", "--check")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return true, err
		}
		return strings.Contains(string(out), "AvailableUpdate"), nil
	}

	return false, nil
}

func (up SystemUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (dr SystemDriver) BootcCompatible() (bool, error) {
	cmd := exec.Command(dr.bootcBinaryPath, "status", "--format=json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	var status bootcStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return false, nil
	}
	return !(status.Status.Booted.Incompatible || status.Status.Staged.Incompatible), nil
}

func (up SystemUpdater) New(config UpdaterInitConfiguration) (SystemUpdater, error) {
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "System Updates",
		Enabled:     !config.Ci,
		DryRun:      config.DryRun,
	}

	if up.Config.DryRun {
		up.Outdated = false
		return up, nil
	}

	up.SystemDriver = SystemDriver{}
	bootcBinaryPath, exists := config.Environment["UUPD_BOOTC_BINARY"]
	if !exists || bootcBinaryPath == "" {
		up.SystemDriver.bootcBinaryPath = "/usr/bin/bootc"
	} else {
		up.SystemDriver.bootcBinaryPath = bootcBinaryPath
	}
	rpmOstreeBinaryPath, exists := config.Environment["UUPD_RPMOSTREE_BINARY"]
	if !exists || rpmOstreeBinaryPath == "" {
		up.SystemDriver.rpmOstreeBinaryPath = "/usr/bin/rpm-ostree"
	} else {
		up.SystemDriver.rpmOstreeBinaryPath = rpmOstreeBinaryPath
	}

	isBootc, err := up.SystemDriver.BootcCompatible()
	if err != nil {
		return up, err
	}
	if isBootc {
		up.SystemDriver.Variation = Bootc
	} else {
		up.SystemDriver.Variation = RpmOstree
	}

	outdated, err := up.SystemDriver.Outdated()
	if err != nil {
		return up, err
	}
	up.Outdated = outdated

	return up, nil
}

func (up *SystemUpdater) Check() (bool, error) {
	if up.Config.DryRun {
		return true, nil
	}

	updateAvailable, err := up.SystemDriver.UpdateAvailable()
	return updateAvailable, err
}

func (up SystemUpdater) Update() (*[]CommandOutput, error) {
	if up.Config.DryRun {
		return &[]CommandOutput{}, nil
	}

	out, err := up.SystemDriver.Update()
	return out, err
}
