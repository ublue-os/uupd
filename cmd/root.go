package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var (
	rootCmd = &cobra.Command{
		Use:   "uupd",
		Short: "uupd (Universal Update) is the successor to ublue-update, built for bootc",
		Run:   Update,
	}

	waitCmd = &cobra.Command{
		Use:   "wait",
		Short: "Waits for ostree sysroot to unlock",
		Run:   Wait,
	}

	updateCheckCmd = &cobra.Command{
		Use:   "update-check",
		Short: "Check for updates to the booted image",
		Run:   UpdateCheck,
	}

	hardwareCheckCmd = &cobra.Command{
		Use:   "hw-check",
		Short: "Run hardware checks",
		Run:   HwCheck,
	}

	imageOutdatedCmd = &cobra.Command{
		Use:   "is-img-outdated",
		Short: "Print 'true' or 'false' based on if the current booted image is over 1 month old",
		Run:   ImageOutdated,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(waitCmd)
	rootCmd.AddCommand(updateCheckCmd)
	rootCmd.AddCommand(hardwareCheckCmd)
	rootCmd.AddCommand(imageOutdatedCmd)
	rootCmd.Flags().BoolP("hw-check", "c", false, "run hardware check before running updates")
	rootCmd.Flags().BoolP("dry-run", "n", false, "Do a dry run (used for testing)")
}
