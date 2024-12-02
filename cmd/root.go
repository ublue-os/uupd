package cmd

import (
	"fmt"
	"log"
	"os"
	"os/user"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func assertRoot(cmd *cobra.Command, args []string) {
	currentUser, err := user.Current()

	if err != nil {
		log.Fatalf("Error fetching current user: %v", err)
	}
	if currentUser.Uid != "0" {
		log.Fatalf("uupd needs to be invoked as root.")
	}
}

var (
	rootCmd = &cobra.Command{
		Use:    "uupd",
		Short:  "uupd (Universal Update) is the successor to ublue-update, built for bootc",
		PreRun: assertRoot,
		Run:    Update,
	}

	waitCmd = &cobra.Command{
		Use:    "wait",
		Short:  "Waits for ostree sysroot to unlock",
		PreRun: assertRoot,
		Run:    Wait,
	}

	updateCheckCmd = &cobra.Command{
		Use:    "update-check",
		Short:  "Check for updates to the booted image",
		PreRun: assertRoot,
		Run:    UpdateCheck,
	}

	hardwareCheckCmd = &cobra.Command{
		Use:    "hw-check",
		Short:  "Run hardware checks",
		PreRun: assertRoot,
		Run:    HwCheck,
	}

	imageOutdatedCmd = &cobra.Command{
		Use:    "is-img-outdated",
		Short:  "Print 'true' or 'false' based on if the current booted image is over 1 month old",
		PreRun: assertRoot,
		Run:    ImageOutdated,
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
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	rootCmd.Flags().BoolP("no-progress", "p", !isTerminal, "Do not show progress bars")
	rootCmd.Flags().BoolP("hw-check", "c", false, "Run hardware check before running updates")
	rootCmd.Flags().BoolP("dry-run", "n", false, "Do a dry run (used for testing)")
}
