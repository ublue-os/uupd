package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv"
)

func UpdateCheck(cmd *cobra.Command, args []string) {
	systemUpdater, err := drv.SystemUpdater{}.New(drv.UpdaterInitConfiguration{})
	if err != nil {
		slog.Error("Failed getting system driver", slog.Any("error", err))
		return
	}
	updateAvailable, err := systemUpdater.Check()
	if err != nil {
		slog.Error("Failed checking for updates", slog.Any("error", err))
		return
	}
	if updateAvailable {
		slog.Info("Update Available")
	} else {
		slog.Info("No updates available")
	}
}
