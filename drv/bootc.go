package drv

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// implementation of bootc and rpm-ostree commands (rpm-ostree support will be removed in the future)

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

type rpmOstreeStatus struct {
	Deployments []struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"deployments"`
}

func BootcCompat() (bool, error) {
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

func IsBootcImageOutdated() (bool, error) {
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
	timestamp, err := time.Parse(time.RFC3339Nano, status.Status.Booted.Image.Timestamp)
	if err != nil {
		return false, nil
	}
	oneMonthAgo := time.Now().UTC().AddDate(0, -1, 0)

	return timestamp.Before(oneMonthAgo), nil
}

func BootcUpdate() ([]byte, error) {
	cmd := exec.Command("/usr/bin/bootc", "upgrade")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func CheckForBootcImageUpdate() (bool, error) {
	cmd := exec.Command("/usr/bin/bootc", "upgrade", "--check")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, err
	}
	return !strings.Contains(string(out), "No changes in:"), nil
}

func IsRpmOstreeImageOutdated() (bool, error) {
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
	timestamp := time.Unix(status.Deployments[0].Timestamp, 0).UTC()
	oneMonthAgo := time.Now().AddDate(0, -1, 0)

	return timestamp.Before(oneMonthAgo), nil
}

func RpmOstreeUpdate() ([]byte, error) {
	cmd := exec.Command("/usr/bin/rpm-ostree", "upgrade")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func CheckForRpmOstreeImageUpdate() (bool, error) {
	// This function may or may not be accurate, rpm-ostree updgrade --check has issues... https://github.com/coreos/rpm-ostree/issues/1579
	// Not worried because we will end up removing rpm-ostree from the equation soon
	cmd := exec.Command("/usr/bin/rpm-ostree", "upgrade", "--check")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, err
	}
	return strings.Contains(string(out), "AvailableUpdate"), nil
}

// Generalize bootc and rpm-ostree drivers into this struct as system updaters
type SystemUpdateDriver struct {
	ImageOutdated   func() (bool, error)
	Update          func() ([]byte, error)
	UpdateAvailable func() (bool, error)
	Name            string
}

func GetSystemUpdateDriver() (SystemUpdateDriver, error) {
	useBootc, err := BootcCompat()
	if err != nil {
		// bootc isn't on the current system if there's an error
		return SystemUpdateDriver{
			IsRpmOstreeImageOutdated,
			RpmOstreeUpdate,
			CheckForRpmOstreeImageUpdate,
			"rpm-ostree",
		}, nil
	}
	if useBootc {
		return SystemUpdateDriver{
			IsBootcImageOutdated,
			BootcUpdate,
			CheckForBootcImageUpdate,
			"Bootc",
		}, nil
	}
	return SystemUpdateDriver{
		IsRpmOstreeImageOutdated,
		RpmOstreeUpdate,
		CheckForRpmOstreeImageUpdate,
		"rpm-ostree",
	}, nil
}
