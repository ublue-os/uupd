package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"github.com/ublue-os/uupd/drv"
	"github.com/ublue-os/uupd/lib"
)

func Update(cmd *cobra.Command, args []string) {
	lock, err := lib.AcquireLock()
	if err != nil {
		slog.Error(fmt.Sprintf("%v, is uupd already running?", err))
		return
	}
	defer func() {
		err := lib.ReleaseLock(lock)
		if err != nil {
			slog.Error("Failed releasing lock")
		}
	}()

	hwCheck, err := cmd.Flags().GetBool("hw-check")
	if err != nil {
		slog.Error("Failed to get hw-check flag", "error", err)
		return
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		slog.Error("Failed to get dry-run flag", "error", err)
		return
	}
	verboseRun, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		slog.Error("Failed to get verbose flag", "error", err)
		return
	}

	if hwCheck {
		err := checks.RunHwChecks()
		if err != nil {
			slog.Error("Hardware checks failed", "error", err)
			return
		}
		slog.Info("Hardware checks passed")
	}

	users, err := lib.ListUsers()
	if err != nil {
		slog.Error("Failed to list users", "users", users)
		return
	}

	initConfiguration := drv.UpdaterInitConfiguration{}.New()
	_, empty := os.LookupEnv("CI")
	initConfiguration.Ci = !empty
	initConfiguration.DryRun = dryRun
	initConfiguration.Verbose = verboseRun

	systemUpdater, err := drv.SystemUpdater{}.New(*initConfiguration)
	if err != nil {
		systemUpdater.Config.Enabled = false
	} else {
		enableUpd, err := systemUpdater.Check()
		if err != nil {
			slog.Error("Failed checking for updates")
		}
		systemUpdater.Config.Enabled = enableUpd
	}

	brewUpdater, err := drv.BrewUpdater{}.New(*initConfiguration)
	brewUpdater.Config.Enabled = err == nil

	flatpakUpdater, err := drv.FlatpakUpdater{}.New(*initConfiguration)
	flatpakUpdater.Config.Enabled = err == nil
	flatpakUpdater.SetUsers(users)

	distroboxUpdater, err := drv.DistroboxUpdater{}.New(*initConfiguration)
	distroboxUpdater.Config.Enabled = err == nil
	distroboxUpdater.SetUsers(users)

	totalSteps := brewUpdater.Steps() + systemUpdater.Steps() + flatpakUpdater.Steps() + distroboxUpdater.Steps()
	pw := lib.NewProgressWriter()
	pw.SetNumTrackersExpected(1)
	pw.SetAutoStop(false)

	progressEnabled, err := cmd.Flags().GetBool("no-progress")
	if err != nil {
		slog.Error("Failed to get no-progress flag", "error", err)
		return
	}
	// Move this to its actual boolean value (~no-progress)
	progressEnabled = !progressEnabled

	if progressEnabled {
		go pw.Render()
	}

	// -1 because 0 index
	tracker := lib.NewIncrementTracker(&progress.Tracker{Message: "Updating", Units: progress.UnitsDefault, Total: int64(totalSteps - 1)}, totalSteps-1)
	pw.AppendTracker(tracker.Tracker)

	var trackerConfig = &drv.TrackerConfiguration{
		Tracker:  tracker,
		Writer:   &pw,
		Progress: progressEnabled,
	}
	flatpakUpdater.Tracker = trackerConfig
	distroboxUpdater.Tracker = trackerConfig

	var outputs = []drv.CommandOutput{}

	if systemUpdater.Outdated {
		const OUTDATED_WARNING = "There hasn't been an update in over a month. Consider rebooting or running updates manually"
		err := lib.Notify("System Warning", OUTDATED_WARNING)
		if err != nil {
			slog.Error("Failed showing warning notification")
		}
		slog.Warn(OUTDATED_WARNING)
	}

	if systemUpdater.Config.Enabled {
		lib.ChangeTrackerMessageFancy(pw, tracker, progressEnabled, lib.TrackerMessage{Title: systemUpdater.Config.Title, Description: systemUpdater.Config.Description})
		out, err := systemUpdater.Update()
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if brewUpdater.Config.Enabled {
		lib.ChangeTrackerMessageFancy(pw, tracker, progressEnabled, lib.TrackerMessage{Title: brewUpdater.Config.Title, Description: brewUpdater.Config.Description})
		out, err := brewUpdater.Update()
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if flatpakUpdater.Config.Enabled {
		out, err := flatpakUpdater.Update()
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if distroboxUpdater.Config.Enabled {
		out, err := distroboxUpdater.Update()
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	pw.Stop()
	if verboseRun {
		slog.Info("Verbose run requested")

		for _, output := range outputs {
			slog.Info("CommandOutput", slog.String("context", output.Context), slog.String("stdout", output.Stdout), slog.Any("stderr", output.Stderr))
		}

		return
	}

	var failures = []drv.CommandOutput{}
	for _, output := range outputs {
		if output.Failure {
			failures = append(failures, output)
		}
	}

	if len(failures) > 0 {
		slog.Warn("Exited with failed updates.")

		for _, output := range failures {
			slog.Info("CommandOutput", slog.String("context", output.Context), slog.String("stdout", output.Stdout), slog.Any("stderr", output.Stderr))
		}

		return
	}

	slog.Info("Updates Completed Successfully")
}
