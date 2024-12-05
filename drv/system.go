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

type SystemDriver int

const (
	Bootc SystemDriver = iota
	RpmOstree
)

func (dr SystemDriver) Outdated() (bool, error) {
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	var timestamp time.Time
	switch dr {
	case Bootc:
		cmd := exec.Command("bootc", "status", "--format=json")
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
		cmd := exec.Command("rpm-ostree", "status", "--json", "--booted")
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
	switch dr {
	case Bootc:
		cmd = exec.Command("/usr/bin/bootc", "upgrade")
	case RpmOstree:
		cmd = exec.Command("/usr/bin/rpm-ostree", "upgrade")
	}
	out, err := cmd.CombinedOutput()
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("System update")
	}
	finalOutput = append(finalOutput, *tmpout)
	return &finalOutput, err
}

func (dr SystemDriver) UpdateAvailable() (bool, error) {
	switch dr {
	case Bootc:
		cmd := exec.Command("/usr/bin/bootc", "upgrade", "--check")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return true, err
		}
		return !strings.Contains(string(out), "No changes in:"), nil
	case RpmOstree:
		// This function may or may not be accurate, rpm-ostree updgrade --check has issues... https://github.com/coreos/rpm-ostree/issues/1579
		// Not worried because we will end up removing rpm-ostree from the equation soon
		cmd := exec.Command("/usr/bin/rpm-ostree", "upgrade", "--check")
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

func BootcCompatible() (bool, error) {
	cmd := exec.Command("bootc", "status", "--format=json")
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

func (up SystemUpdater) New(initconfig UpdaterInitConfiguration) (SystemUpdater, error) {
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "System Updates",
		Enabled:     !initconfig.Ci,
		DryRun:      initconfig.DryRun,
	}

	if up.Config.DryRun {
		up.Outdated = false
		return up, nil
	}

	isBootc, err := BootcCompatible()
	if err != nil {
		return up, err
	}
	if isBootc {
		up.SystemDriver = Bootc
	} else {
		up.SystemDriver = RpmOstree
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
	var final_output = []CommandOutput{}

	if up.Config.DryRun {
		return &final_output, nil
	}

	out, err := up.SystemDriver.Update()
	final_output = append(final_output, *out...)

	return &final_output, err
}
