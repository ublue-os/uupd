package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"log"
)

func HwCheck(cmd *cobra.Command, args []string) {
	// (some hardware checks require dbus access)
	err := checks.RunHwChecks()
	if err != nil {
		log.Fatalf("Hardware checks failed: %v", err)
	}
	log.Println("Hardware checks passed")
}
