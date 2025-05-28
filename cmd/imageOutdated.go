package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/system"
)

func ImageOutdated(cmd *cobra.Command, args []string) {
	initConfiguration := generic.UpdaterInitConfiguration{}.New()
	initConfiguration.Ci = false
	initConfiguration.DryRun = false
	initConfiguration.Verbose = false

	mainSystemDriver, _, _, err := system.InitializeSystemDriver(*initConfiguration)
	if err != nil {
		slog.Error("Failed")
		return
	}

	systemOutdated, err := mainSystemDriver.Outdated()

	if err != nil {
		slog.Error("Failed checking if system is out of date")
	}

	slog.Info("Outdated Check", slog.Bool("image_outdated", systemOutdated))
	if systemOutdated {
		os.Exit(77)
	}
}
