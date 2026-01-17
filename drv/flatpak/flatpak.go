package flatpak

import (
	"log/slog"
	"os/exec"
	"strings"

	. "github.com/ublue-os/uupd/drv/generic"
	appConfig "github.com/ublue-os/uupd/pkg/config"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

type FlatpakUpdater struct {
	Config       DriverConfiguration
	binaryPath   string
	users        []session.User
	usersEnabled bool
}

func (up FlatpakUpdater) Steps() int {
	if up.Config.Enabled {
		var steps = 1
		if up.usersEnabled {
			steps += len(up.users)
		}
		return steps
	}
	return 0
}

func (up FlatpakUpdater) New(config UpdaterInitConfiguration) (FlatpakUpdater, error) {
	conf := appConfig.Get().Modules.Flatpak
	userdesc := "Apps for User:"
	up.Config = DriverConfiguration{
		Title:           "Flatpak",
		Description:     "System Apps",
		UserDescription: &userdesc,
		Enabled:         true,
		MultiUser:       true,
		DryRun:          config.DryRun,
		Environment:     config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))
	up.usersEnabled = false

	up.binaryPath = conf.BinaryPath

	return up, nil
}

func (up *FlatpakUpdater) SetUsers(users []session.User) {
	up.users = users
	up.usersEnabled = true
}

func (up FlatpakUpdater) Check() (bool, error) {
	return true, nil
}

func (up FlatpakUpdater) Update(tracker *percent.Incrementer) (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}

	if up.Config.DryRun {
		tracker.ReportStatusChange(up.Config.Title, up.Config.Description)
		tracker.IncrementSection(nil)

		var err error = nil
		for _, user := range up.users {
			tracker.IncrementSection(err)
			tracker.ReportStatusChange(up.Config.Title, *up.Config.UserDescription+" "+user.Name)
		}
		return &finalOutput, nil
	}

	tracker.ReportStatusChange(up.Config.Title, up.Config.Description)
	cli := []string{up.binaryPath, "update", "-y", "--noninteractive"}
	flatpakCmd := exec.Command(cli[0], cli[1:]...)
	out, err := session.RunLog(up.Config.Logger, slog.LevelDebug, flatpakCmd)
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Context = up.Config.Description
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	finalOutput = append(finalOutput, *tmpout)

	err = nil
	for _, user := range up.users {
		tracker.IncrementSection(err)
		context := *up.Config.UserDescription + " " + user.Name
		tracker.ReportStatusChange(up.Config.Title, context)
		cli := []string{up.binaryPath, "update", "-y"}
		out, err := session.RunUID(up.Config.Logger, slog.LevelDebug, user.UID, cli, nil)
		tmpout = CommandOutput{}.New(out, err)
		tmpout.Context = context
		tmpout.Cli = cli
		tmpout.Failure = err != nil
		finalOutput = append(finalOutput, *tmpout)
	}
	return &finalOutput, nil
}
