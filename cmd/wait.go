package cmd

import (
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/pkg/filelock"
)

func Wait(cmd *cobra.Command, args []string) {
	// TODO: rely on bootc to do transaction wait
	lockFilePath := "/sysroot/ostree/lock"

	for {

		time.Sleep(2 * time.Second)
		file, err := os.Open(lockFilePath)
		if err != nil {
			// file must not exist
			break
		}

		if filelock.IsFileLocked(file) {
			file.Close()
			log.Printf("Waiting for lockfile: %s", lockFilePath)
		} else {
			file.Close()
			break
		}
	}
	log.Printf("Done Waiting!")
}
