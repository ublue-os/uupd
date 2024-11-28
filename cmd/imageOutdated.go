package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv"
)

func ImageOutdated(cmd *cobra.Command, args []string) {
	systemDriver, err := drv.GetSystemUpdateDriver()
	if err != nil {
		log.Fatalf("Failed to get system update driver: %v", err)
	}
	outdated, err := systemDriver.ImageOutdated()
	if err != nil {
		log.Fatalf("Cannot determine if image is outdated: %v", err)
	}
	log.Printf("%t", outdated)
}
