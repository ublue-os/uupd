package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"github.com/ublue-os/uupd/drv/brew"
	"github.com/ublue-os/uupd/drv/distrobox"
	"github.com/ublue-os/uupd/drv/flatpak"
	drv "github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/system"

	"github.com/ublue-os/uupd/pkg/config"
	"github.com/ublue-os/uupd/pkg/filelock"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

func runModule(
	title string,
	tracker *percent.Incrementer,
	outputs *[]drv.CommandOutput,
	update func(*percent.Incrementer) (*[]drv.CommandOutput, error),
) error {
	module := strings.ToLower(title)

	out, err := update(tracker)
	for i := range *out {
		(*out)[i].Module = module
	}
	*outputs = append(*outputs, *out...)

	exitCode := 0
	for _, output := range *out {
		if output.ExitCode != 0 {
			exitCode = output.ExitCode
			break
		}
	}
	if err != nil && exitCode == 0 {
		exitCode = 1
	}

	if !tracker.ProgressEnabled {
		message := fmt.Sprintf("%s module has completed.", title)
		log := slog.Info
		if exitCode != 0 {
			message = fmt.Sprintf("%s module has failed.", title)
			log = slog.Error
		}
		log(message,
			slog.String("module", module),
			slog.String("status", "finished"),
			slog.Int("exit_code", exitCode),
			slog.String("output", message),
		)
	}

	tracker.IncrementSection(err)
	return err
}

func logModuleFailure(output drv.CommandOutput) {
	slog.Error("module_fail",
		slog.String("module", output.Module),
		slog.String("context", output.Context),
		slog.String("status", "failed"),
		slog.Int("exit_code", output.ExitCode),
		slog.String("output", output.Stdout),
		slog.String("cli", strings.Join(output.Cli, " ")),
	)
}

func Update(cmd *cobra.Command, args []string) error {
	conf := config.Get()
	modules := conf.Modules

	lockfile, err := filelock.OpenLockfile(filelock.GetDefaultLockfile())
	if err != nil {
		slog.Error("Failed creating and opening lockfile. Is uupd already running?", slog.Any("error", err))
		return err
	}
	defer func(lockfile *os.File) {
		err := filelock.ReleaseLock(lockfile)
		if err != nil {
			slog.Error("Failed releasing lock", slog.Any("error", err))
		}
	}(lockfile)

	if err := filelock.AcquireLock(lockfile, filelock.TimeoutConfig{Tries: 5}); err != nil {
		slog.Error(fmt.Sprintf("%v, is uupd already running?", err))
		return err
	}

	hwCheck := conf.Checks.Hardware.Enable
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		slog.Error("Failed to get dry-run flag", "error", err)
		return err
	}
	verboseRun, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		slog.Error("Failed to get verbose flag", "error", err)
		return err
	}
	jsonLog := fLogJson
	logLevel := fLogLevel
	// We DONT want to display the progress bar when we have JSON logs or when are logs are cluttered/debug (prints out command output)
	disableProgress := jsonLog || (logLevel != "info")
	applySystem, err := cmd.Flags().GetBool("apply")
	if err != nil {
		slog.Error("Failed to get apply flag", "error", err)
		return err
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		slog.Error("Failed to get force flag", "error", err)
		return err
	}

	disableModuleSystem := modules.System.Disable
	disableModuleFlatpak := modules.Flatpak.Disable
	disableModuleBrew := modules.Brew.Disable
	disableModuleDistrobox := modules.Distrobox.Disable

	if hwCheck {
		err := checks.RunHwChecks()
		if err != nil {
			slog.Error("Hardware checks failed", "error", err)
			return err
		}
		slog.Info("Hardware checks passed")
	}

	users, err := session.ListUsers()
	if err != nil {
		slog.Error("Failed to list users", "users", users)
		return err
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

	tracker := percent.NewIncrementer(!disableProgress, totalSteps)
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

	if mainSystemDriverConfig.Enabled {
		slog.Debug(fmt.Sprintf("%s module", mainSystemDriverConfig.Title), slog.String("module", strings.ToLower(mainSystemDriverConfig.Title)), slog.Any("module_configuration", mainSystemDriverConfig))
		tracker.ReportStatusChange(mainSystemDriverConfig.Title, mainSystemDriverConfig.Description)
		_ = runModule(mainSystemDriverConfig.Title, &tracker, &outputs, mainSystemDriver.Update)
	}

	if brewUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", brewUpdater.Config.Title), slog.String("module", strings.ToLower(brewUpdater.Config.Title)), slog.Any("module_configuration", brewUpdater.Config))
		tracker.ReportStatusChange(brewUpdater.Config.Title, brewUpdater.Config.Description)
		_ = runModule(brewUpdater.Config.Title, &tracker, &outputs, brewUpdater.Update)
	}

	if flatpakUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", flatpakUpdater.Config.Title), slog.String("module", strings.ToLower(flatpakUpdater.Config.Title)), slog.Any("module_configuration", flatpakUpdater.Config))
		_ = runModule(flatpakUpdater.Config.Title, &tracker, &outputs, flatpakUpdater.Update)
	}

	if distroboxUpdater.Config.Enabled {
		slog.Debug(fmt.Sprintf("%s module", distroboxUpdater.Config.Title), slog.String("module", strings.ToLower(distroboxUpdater.Config.Title)), slog.Any("module_configuration", distroboxUpdater.Config))
		_ = runModule(distroboxUpdater.Config.Title, &tracker, &outputs, distroboxUpdater.Update)
	}

	if !disableProgress {
		tracker.ProgressWriter.Stop()
		time.Sleep(time.Millisecond * 100)
		percent.ResetOscProgress()
	}
	if verboseRun {
		slog.Info("Verbose run requested")

		for _, output := range outputs {
			slog.Info(output.Context,
				slog.String("module", output.Module),
				slog.String("context", output.Context),
				slog.String("status", "finished"),
				slog.Int("exit_code", output.ExitCode),
				slog.String("output", output.Stdout),
				slog.String("cli", strings.Join(output.Cli, " ")),
			)
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
			logModuleFailure(output)
		}
		_ = session.Notify(users, "Some System Updates Failed", fmt.Sprintf("Systems Failed: %s", strings.Join(contexts, ", ")), "critical")

		slog.Error("Updates finished with errors!")
		return fmt.Errorf("%d update module(s) failed: %s", len(failures), strings.Join(contexts, ", "))
	}

	slog.Info("Updates Completed Successfully")
	if applySystem && mainSystemDriverConfig.Enabled {
		slog.Info("Applying System Update")
		cmd := exec.Command("/usr/bin/systemctl", "reboot")
		err := cmd.Run()
		if err != nil {
			slog.Error("Failed rebooting machine for updates", slog.Any("error", err))
			return err
		}
	}
	return nil
}
