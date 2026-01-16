package cmd

import (
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/pkg/filelock"
)

func Wait(cmd *cobra.Command, args []string) error {
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
			file.Close() //nolint:errcheck
			slog.Info("Waiting for lockfile", slog.String("path", lockFilePath))
		} else {
			file.Close() //nolint:errcheck
			break
		}
	}
	slog.Info("Done Waiting!")
	return nil
}
