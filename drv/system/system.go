package system

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/rpmostree"
	"github.com/ublue-os/uupd/pkg/percent"
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
	Check() (bool, error)
	Update(tracker *percent.Incrementer) (*[]CommandOutput, error)
}

type SystemUpdater struct {
	Config     DriverConfiguration
	BinaryPath string
}

// Bootc Progress
type StageInfo struct {
	Text   string
	Start  int
	Length int
}

var PROGRESS_STAGES = map[string]StageInfo{
	"pulling":   {"Downloading", 0, 80},
	"importing": {"Importing", 80, 10},
	"staging":   {"Deploying", 90, 10},
	"unknown":   {"Loading", 100, 0},
}

type BootcProgress struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Task        string `json:"task"`
	Steps       int    `json:"steps"`
	StepsTotal  int    `json:"stepsTotal"`
	Bytes       int    `json:"bytes"`
	BytesTotal  int    `json:"bytesTotal"`
}

func (up SystemUpdater) Outdated() (bool, error) {
	if up.Config.DryRun {
		return false, nil
	}

	cmd := exec.Command(up.BinaryPath, "status", "--format=json")
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
	oneMonthAgo := time.Now().AddDate(0, -1, 0).UTC()

	return timestamp.UTC().Before(oneMonthAgo), nil
}

func (up SystemUpdater) Update(tracker *percent.Incrementer) (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}
	var cmd *exec.Cmd
	binaryPath := up.BinaryPath

	cli := []string{binaryPath, "upgrade", "--quiet", "--progress-fd", "3"}

	up.Config.Logger.Debug("Executing update", slog.Any("cli", cli))

	cmd = exec.Command(cli[0], cli[1:]...)

	r, w, err := os.Pipe()
	if err != nil {
		return &finalOutput, err
	}

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.ExtraFiles = []*os.File{w}

	err = cmd.Start()
	if err != nil {
		return &finalOutput, err
	}

	scanner := bufio.NewScanner(r)
	go bootcScan(scanner, tracker, up.Config.Logger, slog.LevelDebug)
	err = cmd.Wait()

	tmpout := CommandOutput{}.New(errb.Bytes(), err)
	tmpout.Failure = err != nil
	tmpout.Context = "System Update"
	finalOutput = append(finalOutput, *tmpout)
	return &finalOutput, err
}

func bootcScan(scanner *bufio.Scanner, tracker *percent.Incrementer, logger *slog.Logger, level slog.Level) {
	for scanner.Scan() {

		logger.Log(context.TODO(), level, scanner.Text())
		var progress BootcProgress

		err := json.Unmarshal(scanner.Bytes(), &progress)

		if err != nil {
			continue
		}
		logger.Log(context.TODO(), level, "scanned progress", slog.Any("struct", progress))

		stageInfo, exists := PROGRESS_STAGES[progress.Task]

		if !exists {
			stageInfo = PROGRESS_STAGES["unknown"]
		}

		switch progress.Type {
		case "ProgressSteps":
			curr := progress.Steps
			total := progress.StepsTotal
			value := float64(stageInfo.Start) + math.Min(float64(stageInfo.Length), float64(curr)/float64(total+1)*float64(stageInfo.Length))
			tracker.SectionPercent(value)
			tracker.ReportStatusChange("System", stageInfo.Text)

		case "ProgressBytes":
			curr := progress.Bytes
			total := progress.BytesTotal
			value := float64(stageInfo.Start) + math.Min(float64(stageInfo.Length), float64(curr)/float64(total)*float64(stageInfo.Length))
			tracker.SectionPercent(value)
			tracker.ReportStatusChange("System", stageInfo.Text)
		default:
			continue
		}
	}
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
		Description: "Bootc",
		Enabled:     !config.Ci,
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.BinaryPath = EnvOrFallback(config.Environment, "UUPD_BOOTC_BINARY", "/usr/bin/bootc")

	return up, nil
}

func (up SystemUpdater) Check() (bool, error) {
	if up.Config.DryRun {
		return true, nil
	}

	cmd := exec.Command(up.BinaryPath, "upgrade", "--check")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, err
	}

	updateNecessary := !strings.Contains(string(out), "No changes in:")
	up.Config.Logger.Debug("Executed update check", slog.String("output", string(out)), slog.Bool("update", updateNecessary))
	return updateNecessary, nil
}

func BootcCompatible(binaryPath string) bool {
	cmd := exec.Command(binaryPath, "status", "--format=json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	var status bootcStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return false
	}
	return !(status.Status.Booted.Incompatible || status.Status.Staged.Incompatible)
}

func InitializeSystemDriver(initConfiguration UpdaterInitConfiguration) (SystemUpdateDriver, DriverConfiguration, bool, error) {

	rpmOstreeUpdater, err := rpmostree.RpmOstreeUpdater{}.New(initConfiguration)

	systemUpdater, err := SystemUpdater{}.New(initConfiguration)

	isBootc := BootcCompatible(systemUpdater.BinaryPath)

	if !isBootc {
		slog.Debug("Using rpm-ostree fallback as system driver")
	}

	// The system driver to be applied needs to have the correct "enabled" value since it will NOT update from here onwards.
	systemUpdater.Config.Enabled = systemUpdater.Config.Enabled && isBootc

	rpmOstreeUpdater.Config.Enabled = rpmOstreeUpdater.Config.Enabled && !isBootc

	// var finalConfig DriverConfiguration
	var mainSystemDriver SystemUpdateDriver
	if isBootc {
		mainSystemDriver = systemUpdater
		return mainSystemDriver, systemUpdater.Config, isBootc, err
	}
	mainSystemDriver = rpmOstreeUpdater
	return mainSystemDriver, rpmOstreeUpdater.Config, isBootc, err

}
