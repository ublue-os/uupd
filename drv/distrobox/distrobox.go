package distrobox

import (
	"log/slog"
	"os"
	"strings"

	. "github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

type DistroboxUpdater struct {
	Config       DriverConfiguration
	binaryPath   string
	users        []session.User
	usersEnabled bool
}

func (up DistroboxUpdater) Steps() int {
	if up.Config.Enabled {
		var steps = 1
		if up.usersEnabled {
			steps += len(up.users)
		}
		return steps
	}
	return 0
}

func (up DistroboxUpdater) New(config UpdaterInitConfiguration) (DistroboxUpdater, error) {
	userdesc := "Distroboxes for User:"
	up.Config = DriverConfiguration{
		Title:           "Distrobox",
		Description:     "Rootful Distroboxes",
		UserDescription: &userdesc,
		Enabled:         true,
		MultiUser:       true,
		DryRun:          config.DryRun,
		Environment:     config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.usersEnabled = false

	up.binaryPath = EnvOrFallback(up.Config.Environment, "UUPD_DISTROBOX_BINARY", "/usr/bin/distrobox")

	if up.Config.DryRun {
		return up, nil
	}

	inf, err := os.Stat(up.binaryPath)
	if err != nil {
		return up, err
	}
	// check if file is executable using bitmask
	up.Config.Enabled = inf.Mode()&0111 != 0

	return up, nil
}

func (up *DistroboxUpdater) SetUsers(users []session.User) {
	up.users = users
	up.usersEnabled = true
}

func (up DistroboxUpdater) Check() (bool, error) {
	return true, nil
}

func (up DistroboxUpdater) Update(tracker *percent.Incrementer) (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}

	if up.Config.DryRun {
		tracker.IncrementSection(nil)
		tracker.ReportStatusChange(up.Config.Title, up.Config.Description)

		var err error = nil
		for _, user := range up.users {
			tracker.IncrementSection(err)
			tracker.ReportStatusChange(up.Config.Title, *up.Config.UserDescription+" "+user.Name)
		}
		return &finalOutput, nil
	}

	tracker.ReportStatusChange(up.Config.Title, up.Config.Description)
	cli := []string{up.binaryPath, "upgrade", "-a"}
	out, err := session.RunUID(up.Config.Logger, slog.LevelDebug, 0, cli, nil)
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Context = up.Config.Description
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	finalOutput = append(finalOutput, *tmpout)

	err = nil
	for _, user := range up.users {
		tracker.IncrementSection(err)
		context := *up.Config.UserDescription + " " + user.Name
		tracker.ReportStatusChange(up.Config.Title, *up.Config.UserDescription+" "+user.Name)
		cli := []string{up.binaryPath, "upgrade", "-a"}
		out, err := session.RunUID(up.Config.Logger, slog.LevelDebug, user.UID, cli, nil)
		tmpout = CommandOutput{}.New(out, err)
		tmpout.Context = context
		tmpout.Cli = cli
		tmpout.Failure = err != nil
		finalOutput = append(finalOutput, *tmpout)
	}
	return &finalOutput, nil
}
