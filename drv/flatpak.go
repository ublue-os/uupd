package drv

import (
	"os/exec"

	"github.com/ublue-os/uupd/lib"
)

type FlatpakUpdater struct {
	Config       DriverConfiguration
	Tracker      *TrackerConfiguration
	binaryPath   string
	users        []lib.User
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
	userdesc := "Apps for User:"
	up.Config = DriverConfiguration{
		Title:           "Flatpak",
		Description:     "System Apps",
		UserDescription: &userdesc,
		Enabled:         true,
		MultiUser:       true,
		DryRun:          config.DryRun,
	}
	up.usersEnabled = false
	up.Tracker = nil

	binaryPath, empty := config.Environment["UUPD_FLATPAK_BINARY"]
	if empty || binaryPath == "" {
		up.binaryPath = "/usr/bin/flatpak"
	} else {
		up.binaryPath = binaryPath
	}

	return up, nil
}

func (up *FlatpakUpdater) SetUsers(users []lib.User) {
	up.users = users
	up.usersEnabled = true
}

func (up FlatpakUpdater) Check() (*[]CommandOutput, error) {
	return nil, nil
}

func (up FlatpakUpdater) Update() (*[]CommandOutput, error) {
	var finalOutput = []CommandOutput{}

	if up.Config.DryRun {
		lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: up.Config.Description})
		up.Tracker.Tracker.IncrementSection(nil)

		var err error = nil
		for _, user := range up.users {
			up.Tracker.Tracker.IncrementSection(err)
			lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: *up.Config.UserDescription + " " + user.Name})
		}
		return &finalOutput, nil
	}

	lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: up.Config.Description})
	cli := []string{up.binaryPath, "update", "-y"}
	flatpakCmd := exec.Command(cli[0], cli[1:]...)
	out, err := flatpakCmd.CombinedOutput()
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Context = up.Config.Description
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	finalOutput = append(finalOutput, *tmpout)

	err = nil
	for _, user := range up.users {
		up.Tracker.Tracker.IncrementSection(err)
		context := *up.Config.UserDescription + " " + user.Name
		lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: context})
		cli := []string{up.binaryPath, "update", "-y"}
		out, err := lib.RunUID(user.UID, cli, nil)
		tmpout = CommandOutput{}.New(out, err)
		tmpout.Context = context
		tmpout.Cli = cli
		tmpout.Failure = err != nil
		finalOutput = append(finalOutput, *tmpout)
	}
	return &finalOutput, nil
}
