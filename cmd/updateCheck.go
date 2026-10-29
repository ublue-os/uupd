package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/system"
)

func UpdateCheck(cmd *cobra.Command, args []string) error {
	initConfiguration := generic.UpdaterInitConfiguration{}.New()
	initConfiguration.Ci = false
	initConfiguration.DryRun = false
	initConfiguration.Verbose = false

	mainSystemDriver, _, _, err := system.InitializeSystemDriver(*initConfiguration)
	if err != nil {
		slog.Error("Failed")
		return err
	}

	updateAvailable, err := mainSystemDriver.Check()
	if err != nil {
		slog.Error("Failed checking for updates", slog.Any("error", err))
		return err
	}
	slog.Info("Update Check", slog.Bool("update_available", updateAvailable))
	if !updateAvailable {
		os.Exit(77)
	}
	return nil
}
