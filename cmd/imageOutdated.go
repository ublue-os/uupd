package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv"
)

func ImageOutdated(cmd *cobra.Command, args []string) {
	systemUpdater, err := drv.SystemUpdater{}.New(drv.UpdaterInitConfiguration{})
	if err != nil {
		slog.Error("Failed getting system driver", slog.Any("error", err))
		return
	}

	println(systemUpdater.Outdated)
}
