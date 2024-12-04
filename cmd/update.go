package cmd

import (
	"fmt"
	"log"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"github.com/ublue-os/uupd/drv"
	"github.com/ublue-os/uupd/lib"
	"gopkg.in/yaml.v3"
)

func Update(cmd *cobra.Command, args []string) {
	lock, err := lib.AcquireLock()
	if err != nil {
		log.Fatalf("%v, is uupd already running?", err)
	}
	defer lib.ReleaseLock(lock)

	hwCheck, err := cmd.Flags().GetBool("hw-check")
	if err != nil {
		log.Fatalf("Failed to get hw-check flag: %v", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		log.Fatalf("Failed to get dry-run flag: %v", err)
	}
	verboseRun, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		log.Fatalf("Failed to get verbose flag: %v", err)
	}

	if hwCheck {
		err := checks.RunHwChecks()
		if err != nil {
			log.Fatalf("Hardware checks failed: %v", err)
		}
		log.Println("Hardware checks passed")
	}

	users, err := lib.ListUsers()
	if err != nil {
		log.Fatalf("Failed to list users")
	}

	systemUpdater, err := drv.SystemUpdater{}.New(dryRun)
	if err != nil {
		systemUpdater.Config.Enabled = false
	} else {
		systemUpdater.Check()
	}

	brewUpdater, err := drv.BrewUpdater{}.New(dryRun)
	if err != nil {
		brewUpdater.Config.Enabled = false
	}

	flatpakUpdater, err := drv.FlatpakUpdater{}.New(dryRun)
	if err != nil {
		flatpakUpdater.Config.Enabled = false
	}
	flatpakUpdater.SetUsers(users)

	distroboxUpdater, err := drv.DistroboxUpdater{}.New(dryRun)
	if err != nil {
		distroboxUpdater.Config.Enabled = false
	}
	distroboxUpdater.SetUsers(users)

	totalSteps := brewUpdater.Steps() + systemUpdater.Steps() + flatpakUpdater.Steps() + distroboxUpdater.Steps()
	pw := lib.NewProgressWriter()
	pw.SetNumTrackersExpected(1)
	pw.SetAutoStop(false)

	progressEnabled, err := cmd.Flags().GetBool("no-progress")
	if err != nil {
		log.Fatalf("Failed to get no-progress flag: %v", err)
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
		lib.Notify("System Warning", "There hasn't been an update in over a month. Consider rebooting or running updates manually")
		log.Printf("There hasn't been an update in over a month. Consider rebooting or running updates manually")
	}

	if systemUpdater.Config.Enabled {
		lib.ChangeTrackerMessageFancy(pw, tracker, progressEnabled, fmt.Sprintf("Updating %s (%s)", systemUpdater.Config.Description, systemUpdater.Config.Title))
		out, err := systemUpdater.Update()
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if brewUpdater.Config.Enabled {
		lib.ChangeTrackerMessageFancy(pw, tracker, progressEnabled, fmt.Sprintf("Updating %s (%s)", brewUpdater.Config.Description, brewUpdater.Config.Title))
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
		log.Println("Verbose run requested")

		b, err := yaml.Marshal(outputs)
		if err != nil {
			log.Fatalf("Failure!")
			return
		}
		log.Printf("%s", string(b))
		return
	}

	var failures = []drv.CommandOutput{}
	for _, output := range outputs {
		if output.Failure {
			failures = append(failures, output)
		}
	}

	if len(failures) > 0 {
		log.Println("Exited with failed updates.\nFailures found:")

		b, err := yaml.Marshal(failures)
		if err != nil {
			log.Fatalf("Failure!")
			return
		}
		log.Printf("%s", string(b))

		return
	}

	log.Printf("Updates Completed")
}
