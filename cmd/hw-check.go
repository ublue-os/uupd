package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
)

func HwCheck(cmd *cobra.Command, args []string) error {
	// (some hardware checks require dbus access)
	err := checks.RunHwChecks()
	if err != nil {
		slog.Error("Hardware checks failed: %v", err)
		return err
	}
	slog.Info("Hardware checks passed")

	return nil
}
