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
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

type rpmOstreeStatus struct {
	Deployments []struct {
		Timestamp int64  `json:"timestamp"`
		Digest    string `json:"ostree.manifest-digest"`
		Reference string `json:"container-image-reference"`
	} `json:"deployments"`
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
	return &finalOutput, err
}

func (up RpmOstreeUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up RpmOstreeUpdater) New(config UpdaterInitConfiguration) (RpmOstreeUpdater, error) {
	up.Config = DriverConfiguration{
		Title:       "System",
		Description: "rpm-ostree",
		Enabled:     !config.Ci,
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.BinaryPath = EnvOrFallback(up.Config.Environment, "UUPD_RPMOSTREE_BINARY", "/usr/bin/rpm-ostree")
	up.SkopeoPath = EnvOrFallback(up.Config.Environment, "UUPD_SKOPEO_BINARY", "/usr/bin/skopeo")

	return up, nil
}

func (up RpmOstreeUpdater) Check() (bool, error) {
	if up.Config.DryRun {
		return true, nil
	}

	cmd := exec.Command(up.BinaryPath, "status", "--json", "--booted")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("rpm-ostree exited with error: %v", err)
	}
	var status rpmOstreeStatus
	err = json.Unmarshal(out, &status)
	if err != nil {
		return true, fmt.Errorf("unable to unmarshal rpm-ostree status: %v", err)
	}

	ref, err := expandReference(status.Deployments[0].Reference)
	if err != nil {
		return true, fmt.Errorf("couldn't expand container reference: %v", err)
	}
	cmd = exec.Command(up.SkopeoPath, "inspect", ref)
	out, err = cmd.CombinedOutput()
	var inspect skopeoInspect
	err = json.Unmarshal(out, &inspect)
	if err != nil {
		return true, fmt.Errorf("Couldn't unmarshal skopeo inspect: %v", err)
	}

	updateNecessary := inspect.Digest != status.Deployments[0].Digest
	up.Config.Logger.Debug("Executed update check", slog.String("output", string(out)), slog.Bool("update", updateNecessary))
	return updateNecessary, nil
}

func expandReference(s string) (string, error) {
	// ref := strings.SplitN(s, ":", 2)
	_, url, found := strings.Cut(s, ":")
	if !found {
		return "", fmt.Errorf("Cannot expand reference: Malformed container reference: %s", s)
	}
	protocol := "docker://"
	// if the url doesn't have to the protocol (implicit)
	if !strings.Contains(url, protocol) {
		url = protocol + url
	}

	return url, nil
}
