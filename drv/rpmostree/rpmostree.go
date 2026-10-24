package rpmostree

// Temporary: WILL get removed at some point.
// FIXME: Remove this on Spring 2025 when we all move to dnf5 and bootc ideally

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	. "github.com/ublue-os/uupd/drv/generic"
	appConfig "github.com/ublue-os/uupd/pkg/config"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

type rpmOstreeStatus struct {
	Deployments []struct {
		Timestamp int64          `json:"timestamp"`
		Meta      BaseCommitMeta `json:"base-commit-meta"`
		Reference string         `json:"container-image-reference"`
	} `json:"deployments"`
}

type BaseCommitMeta struct {
	Digest string `json:"ostree.manifest-digest"`
}

type skopeoInspect struct {
	Digest string `json:"Digest"`
}

type RpmOstreeUpdater struct {
	Config     DriverConfiguration
	BinaryPath string
	SkopeoPath string
}

func (up RpmOstreeUpdater) Outdated() (bool, error) {
	if up.Config.DryRun {
		return false, nil
	}
	var timestamp time.Time

	cmd := exec.Command(up.BinaryPath, "status", "--json", "--booted")
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
	oneMonthAgo := time.Now().AddDate(0, -1, 0).UTC()

	return timestamp.UTC().Before(oneMonthAgo), nil
}

type updateErrorHint struct {
	pattern string
	message string
}

var updateErrorHints = []updateErrorHint{
	{"no space left on device", "Ran out of disk space during the update. Free up storage and try again."},
	{"out of space", "Ran out of disk space during the update. Free up storage and try again."},
	{"disk full", "Disk is full. Free up storage and try again."},
	{"failed to fetch", "Failed to fetch update data. Check your network connection and try again."},
	{"failed to download", "Failed to download update data. Check your network connection and try again."},
	{"couldn't connect to", "Could not connect to the update server. Check your network connection and try again."},
	{"connection refused", "Connection refused by the update server. Check your network connection and try again."},
	{"connection timed out", "Connection timed out. Check your network connection and try again."},
	{"no route to host", "No route to the update server. Check your network connection and try again."},
	{"invalid checksum", "Invalid checksum while downloading. The update data may be corrupted; try again."},
	{"signature invalid", "Invalid signature. The update data may be corrupted or untrusted; try again."},
	{"failed to commit", "Failed to commit the new deployment. Check system health and try again."},
	{"failed to deploy", "Failed to deploy the new tree. Check system health and try again."},
}

// ClassifyUpdateError inspects the combined output of an rpm-ostree upgrade
// run and, when it matches a known common-failure pattern, returns a
// human-readable hint suitable for printing to the terminal.
func ClassifyUpdateError(output string) string {
	lower := strings.ToLower(output)
	for _, hint := range updateErrorHints {
		if strings.Contains(lower, hint.pattern) {
			return hint.message
		}
	}
	return ""
}

func (up RpmOstreeUpdater) Update(_tracker *percent.Incrementer) (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}
	binaryPath := up.BinaryPath

	cli := []string{binaryPath, "upgrade"}
	up.Config.Logger.Debug("Executing update", slog.Any("cli", cli))
	cmd := exec.Command(cli[0], cli[1:]...)
	out, err := session.RunLog(up.Config.Logger, slog.LevelDebug, cmd)

	tmpout := CommandOutput{}.New(out, err)
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	tmpout.Context = "System Update"
	finalOutput = append(finalOutput, *tmpout)

	if err != nil {
		if hint := ClassifyUpdateError(string(out)); hint != "" {
			up.Config.Logger.Error(hint)
		}
	}
	return &finalOutput, err
}

func (up RpmOstreeUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up RpmOstreeUpdater) New(config UpdaterInitConfiguration) (RpmOstreeUpdater, error) {
	conf := appConfig.Get().Modules.System
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "rpm-ostree",
		Enabled:     !config.Ci,
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.BinaryPath = conf.RpmOstreeBinary
	up.SkopeoPath = conf.SkopeoBinary

	return up, nil
}

func (up RpmOstreeUpdater) Check() (bool, error) {
	if up.Config.DryRun {
		return true, nil
	}

	cmd := exec.Command(up.BinaryPath, "upgrade", "--check", "--unchanged-exit-77")
	out, err := cmd.CombinedOutput()

	if err == nil {
		up.Config.Logger.Debug("Executed update check", slog.String("output", string(out)), slog.Bool("update", true))
		return true, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 77 {
		up.Config.Logger.Debug("rpm-ostree --check reported no updates, verifying via skopeo")
		return up.checkSkopeo()
	}

	return true, fmt.Errorf("rpm-ostree upgrade --check failed: %w", err)
}

func (up RpmOstreeUpdater) checkSkopeo() (bool, error) {
	cmd := exec.Command(up.BinaryPath, "status", "--json", "--booted")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("rpm-ostree exited with error: %w", err)
	}
	var status rpmOstreeStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return true, fmt.Errorf("unable to unmarshal rpm-ostree status: %w", err)
	}

	ref, err := expandReference(status.Deployments[0].Reference)
	if err != nil {
		return true, fmt.Errorf("couldn't expand container reference: %w", err)
	}
	cmd = exec.Command(up.SkopeoPath, "inspect", ref)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("running skopeo inspect failed: %w", err)
	}
	var inspect skopeoInspect
	err = json.Unmarshal(out, &inspect)
	if err != nil {
		return true, fmt.Errorf("couldn't unmarshal skopeo inspect: %w", err)
	}

	updateNecessary := inspect.Digest != status.Deployments[0].Meta.Digest
	up.Config.Logger.Debug("Executed skopeo fallback check", slog.Bool("update", updateNecessary))
	return updateNecessary, nil
}

func expandReference(s string) (string, error) {
	_, url, found := strings.Cut(s, ":")
	if !found {
		return "", fmt.Errorf("Cannot expand reference: Malformed container reference: %s", s)
	}
	protocol := "docker://"
	if !strings.Contains(url, protocol) {
		url = protocol + url
	}

	return url, nil
}
