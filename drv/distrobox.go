package drv

import (
	"fmt"

	"github.com/ublue-os/uupd/lib"
)

type DistroboxUpdater struct {
	Config       DriverConfiguration
	Tracker      *TrackerConfiguration
	users        []lib.User
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

func (up DistroboxUpdater) New(initconfig UpdaterInitConfiguration) (DistroboxUpdater, error) {
	userdesc := "Distroboxes for User:"
	up.Config = DriverConfiguration{
		Title:           "Distrobox",
		Description:     "Rootful Distroboxes",
		UserDescription: &userdesc,
		Enabled:         true,
		MultiUser:       true,
		DryRun:          initconfig.DryRun,
	}
	up.usersEnabled = false
	up.Tracker = nil

	return up, nil
}

func (up *DistroboxUpdater) SetUsers(users []lib.User) {
	up.users = users
	up.usersEnabled = true
}

func (up DistroboxUpdater) Check() (*[]CommandOutput, error) {
	return nil, nil
}

func (up *DistroboxUpdater) Update() (*[]CommandOutput, error) {
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

	// TODO: add env support for Flatpak and Distrobox updaters
	lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: up.Config.Description})
	out, err := lib.RunUID(0, []string{"/usr/bin/distrobox", "upgrade", "-a"}, nil)
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("System Distroboxes")
	}
	finalOutput = append(finalOutput, *tmpout)

	err = nil
	for _, user := range up.users {
		up.Tracker.Tracker.IncrementSection(err)
		lib.ChangeTrackerMessageFancy(*up.Tracker.Writer, up.Tracker.Tracker, up.Tracker.Progress, lib.TrackerMessage{Title: up.Config.Title, Description: *up.Config.UserDescription + " " + user.Name})
		out, err := lib.RunUID(user.UID, []string{"/usr/bin/distrobox", "upgrade", "-a"}, nil)
		tmpout = CommandOutput{}.New(out, err)
		if err != nil {
			tmpout.SetFailureContext(fmt.Sprintf("Distroboxes for User: %s", user.Name))
		}
		finalOutput = append(finalOutput, *tmpout)
	}
	return &finalOutput, nil
}
