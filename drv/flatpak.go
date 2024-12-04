package drv

import (
	"fmt"
	"os/exec"

	"github.com/ublue-os/uupd/lib"
)

type FlatpakUpdater struct {
	Config       DriverConfiguration
	Tracker      *TrackerConfiguration
	users        []lib.User
	usersEnabled bool
}

func (up FlatpakUpdater) Steps() int {
	return (1 + len(up.users))
}

func (up FlatpakUpdater) New(dryrun bool) (FlatpakUpdater, error) {
	userdesc := "Apps for User:"
	up.Config = DriverConfiguration{
		Title:           "Flatpak",
		Description:     "System Apps",
		UserDescription: &userdesc,
		Enabled:         true,
		MultiUser:       true,
		DryRun:          dryrun,
	}
	up.usersEnabled = false
	up.Tracker = nil

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
		lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, fmt.Sprintf("Updating %s (%s)", up.Config.Description, up.Config.Title))
		up.Tracker.Tracker.IncrementSection(nil)

		var err error = nil
		for _, user := range up.users {
			up.Tracker.Tracker.IncrementSection(err)
			lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, fmt.Sprintf("Updating %s %s (%s)", *up.Config.UserDescription, user.Name, up.Config.Title))
		}
		return &finalOutput, nil
	}

	lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, fmt.Sprintf("Updating %s (%s)", up.Config.Description, up.Config.Title))
	flatpakCmd := exec.Command("/usr/bin/flatpak", "update", "-y")
	out, err := flatpakCmd.CombinedOutput()
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("Flatpak System Apps")
	}
	finalOutput = append(finalOutput, *tmpout)

	err = nil
	for _, user := range up.users {
		up.Tracker.Tracker.IncrementSection(err)
		lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, fmt.Sprintf("Updating %s %s (%s)", *up.Config.UserDescription, user.Name, up.Config.Title))
		out, err := lib.RunUID(user.UID, []string{"/usr/bin/flatpak", "update", "-y"}, nil)
		tmpout = CommandOutput{}.New(out, err)
		if err != nil {
			tmpout.SetFailureContext(fmt.Sprintf("Flatpak User: %s", user.Name))
		}
		finalOutput = append(finalOutput, *tmpout)
	}
	return &finalOutput, nil
}
