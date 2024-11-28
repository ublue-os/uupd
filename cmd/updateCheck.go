package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/drv"
	"log"
)

func UpdateCheck(cmd *cobra.Command, args []string) {
	systemDriver, err := drv.GetSystemUpdateDriver()
	if err != nil {
		log.Fatalf("Failed to get system update driver: %v", err)
	}
	update, err := systemDriver.UpdateAvailable()
	if err != nil {
		log.Fatalf("Failed to check for updates: %v", err)
	}
	log.Printf("Update Available: %v", update)
}
