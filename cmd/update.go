package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"github.com/ublue-os/uupd/drv/brew"
	"github.com/ublue-os/uupd/drv/distrobox"
	"github.com/ublue-os/uupd/drv/flatpak"
	drv "github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/system"

	"github.com/ublue-os/uupd/pkg/filelock"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

func Update(cmd *cobra.Command, args []string) {

	meteredIgnore, err := cmd.Flags().GetBool("ignore-metered-connection")
	if err != nil {
		slog.Error("Failed to get ignore-metered-connection flag", "error", err)
		return
	}

	meteredConn, err := session.CheckMeteredConn()
	if err != nil {
		slog.Error("There was an error getting Metered Network status", slog.Any("error", err))
	}
	if meteredConn == true && meteredIgnore == false {
		slog.Error("Metered Network Connection detected. Update will be cancelled. To update on Metered Connections, run with --ignore-metered-connection flag")
		return
	}

	lockfile, err := filelock.OpenLockfile(filelock.GetDefaultLockfile())
	if err != nil {
		slog.Error("Failed creating and opening lockfile. Is uupd already running?", slog.Any("error", err))
		return
	}
	defer func(lockfile *os.File) {
		err := filelock.ReleaseLock(lockfile)
		if err != nil {
			slog.Error("Failed releasing lock", slog.Any("error", err))
		}
	}(lockfile)
	if err := filelock.AcquireLock(lockfile, filelock.TimeoutConfig{Tries: 5}); err != nil {
		slog.Error(fmt.Sprintf("%v, is uupd already running?", err))
		return
	}

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
	jsonLog, err := cmd.Flags().GetBool("json")
	if err != nil {
		slog.Error("Failed to get json flag", "error", err)
		return
	}
	disableProgress, err := cmd.Flags().GetBool("disable-progress")
	if err != nil {
		slog.Error("Failed to get disable-progress flag", "error", err)
		return
	}
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		slog.Error("Failed to get log-level flag", "error", err)
		return
	}
	// We DONT want to display the progress bar when we have JSON logs or when are logs are cluttered/debug (prints out command output)
	disableProgress = disableProgress || jsonLog || (logLevel != "info")
	applySystem, err := cmd.Flags().GetBool("apply")
	if err != nil {
		slog.Error("Failed to get apply flag", "error", err)
		return
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		slog.Error("Failed to get force flag", "error", err)
		return
	}
	disableModuleSystem, err := cmd.Flags().GetBool("disable-module-system")
	if err != nil {
		slog.Error("Failed to get disable-module-system flag", "error", err)
		return
	}
	disableModuleFlatpak, err := cmd.Flags().GetBool("disable-module-flatpak")
	if err != nil {
		slog.Error("Failed to get disable-module-flatpak flag", "error", err)
		return
	}
	disableModuleBrew, err := cmd.Flags().GetBool("disable-module-brew")
	if err != nil {
		slog.Error("Failed to get disable-module-brew flag", "error", err)
		return
	}
	disableModuleDistrobox, err := cmd.Flags().GetBool("disable-module-distrobox")
	if err != nil {
		slog.Error("Failed to get disable-module-distrobox flag", "error", err)
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

	users, err := session.ListUsers()
	if err != nil {
		slog.Error("Failed to list users", "users", users)
		return
	}

	initConfiguration := drv.UpdaterInitConfiguration{}.New()
	_, exists := os.LookupEnv("CI")
	initConfiguration.Ci = exists
	initConfiguration.DryRun = dryRun
	initConfiguration.Verbose = verboseRun

	brewUpdater, err := brew.BrewUpdater{}.New(*initConfiguration)
	if err != nil {
		brewUpdater.Config.Enabled = false
		slog.Debug("Brew driver failed to initialize", slog.Any("error", err))
	}
	brewUpdater.Config.Enabled = !disableModuleBrew

	flatpakUpdater, err := flatpak.FlatpakUpdater{}.New(*initConfiguration)
	if err != nil {
		flatpakUpdater.Config.Enabled = false
		slog.Debug("Flatpak driver failed to initialize", slog.Any("error", err))
	}
	flatpakUpdater.Config.Enabled = !disableModuleFlatpak
	flatpakUpdater.SetUsers(users)

	distroboxUpdater, err := distrobox.DistroboxUpdater{}.New(*initConfiguration)
	if err != nil {
		distroboxUpdater.Config.Enabled = false
		slog.Debug("Distrobox driver failed to initialize", slog.Any("error", err))
	}
	distroboxUpdater.Config.Enabled = !disableModuleDistrobox
	distroboxUpdater.SetUsers(users)

	mainSystemDriver, mainSystemDriverConfig, _, _ := system.InitializeSystemDriver(*initConfiguration)

	enableUpd, err := true, nil
	// if there's no force flag, check for updates
	if !force {
		enableUpd, err = mainSystemDriver.Check()
	}
	if err != nil {
		slog.Error("Failed checking for updates")
	}
	mainSystemDriverConfig.Enabled = mainSystemDriverConfig.Enabled && enableUpd && !disableModuleSystem

	slog.Debug("System Updater module status", slog.Bool("enabled", mainSystemDriverConfig.Enabled))

	totalSteps := brewUpdater.Steps() + flatpakUpdater.Steps() + distroboxUpdater.Steps()
	if mainSystemDriverConfig.Enabled {
		totalSteps += mainSystemDriver.Steps()
	}

	// -1 because 0 index
	tracker := percent.NewIncrementer(!disableProgress, totalSteps-1)
	if !disableProgress {
		percent.ResetOscProgress()
		go tracker.ProgressWriter.Render()
	}

	var outputs = []drv.CommandOutput{}

	systemOutdated, err := mainSystemDriver.Outdated()
	if err != nil {
		slog.Error("Failed checking if system is out of date")
	}
	if systemOutdated {
		const OUTDATED_WARNING = "There hasn't been an update in over a month. Consider rebooting or running updates manually"
		err := session.Notify(users, "System Warning", OUTDATED_WARNING, "critical")
		if err != nil {
			slog.Error("Failed showing warning notification")
		}
		slog.Warn(OUTDATED_WARNING)
	}

	// This section is ugly but we cant really do much about it.
	// Using interfaces doesn't preserve the "Config" struct state and I dont know any other way to make this work without cursed workarounds.

	if mainSystemDriverConfig.Enabled {
		slog.Debug(fmt.Sprintf("%s module", mainSystemDriverConfig.Title), slog.String("module_name", mainSystemDriverConfig.Title), slog.Any("module_configuration", mainSystemDriverConfig))
		tracker.ReportStatusChange(mainSystemDriverConfig.Title, mainSystemDriverConfig.Description)
		var out *[]drv.CommandOutput
		// Pass in the tracker manually because setting it in the config is less possible
		out, err = mainSystemDriver.Update(&tracker)
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if brewUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", brewUpdater.Config.Title), slog.String("module_name", brewUpdater.Config.Title), slog.Any("module_configuration", brewUpdater.Config))
		tracker.ReportStatusChange(brewUpdater.Config.Title, brewUpdater.Config.Description)
		var out *[]drv.CommandOutput
		out, err = brewUpdater.Update(&tracker)
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if flatpakUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", flatpakUpdater.Config.Title), slog.String("module_name", flatpakUpdater.Config.Title), slog.Any("module_configuration", flatpakUpdater.Config))
		var out *[]drv.CommandOutput
		out, err = flatpakUpdater.Update(&tracker)
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if distroboxUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", distroboxUpdater.Config.Title), slog.String("module_name", distroboxUpdater.Config.Title), slog.Any("module_configuration", distroboxUpdater.Config))
		var out *[]drv.CommandOutput
		out, err = distroboxUpdater.Update(&tracker)
		outputs = append(outputs, *out...)
		tracker.IncrementSection(err)
	}

	if !disableProgress {
		tracker.ProgressWriter.Stop()
		time.Sleep(time.Millisecond * 100)
		percent.ResetOscProgress()
	}
	if verboseRun {
		slog.Info("Verbose run requested")

		for _, output := range outputs {
			slog.Info(output.Context, slog.Any("output", output))
		}
	}

	var failures = []drv.CommandOutput{}
	var contexts = []string{}
	for _, output := range outputs {
		if output.Failure {
			failures = append(failures, output)
			contexts = append(contexts, output.Context)
		}
	}

	if len(failures) > 0 {
		slog.Warn("Exited with failed updates.")

		for _, output := range failures {
			slog.Info(output.Context, slog.Any("output", output))
		}
		// session.Notify(users, "Some System Updates Failed", fmt.Sprintf("Systems Failed: %s", strings.Join(contexts, ", ")), "critical")

		return
	}

	slog.Info("Updates Completed Successfully")
	if applySystem && mainSystemDriverConfig.Enabled {
		slog.Info("Applying System Update")
		cmd := exec.Command("/usr/bin/systemctl", "reboot")
		err := cmd.Run()
		if err != nil {
			slog.Error("Failed rebooting machine for updates", slog.Any("error", err))
		}
	}
}
