package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ublue-os/uupd/pkg/config"
	appLogging "github.com/ublue-os/uupd/pkg/logging"
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
		RunE:   Update,
	}

	waitCmd = &cobra.Command{
		Use:    "wait",
		Short:  "Waits for ostree sysroot to unlock",
		PreRun: assertRoot,
		RunE:   Wait,
	}

	updateCheckCmd = &cobra.Command{
		Use:    "update-check",
		Short:  "Check for updates to the booted image, returns exit code 77 if update is not available",
		PreRun: assertRoot,
		RunE:   UpdateCheck,
	}

	hardwareCheckCmd = &cobra.Command{
		Use:    "hw-check",
		Short:  "Run hardware checks",
		PreRun: assertRoot,
		RunE:   HwCheck,
	}

	imageOutdatedCmd = &cobra.Command{
		Use:    "is-img-outdated",
		Short:  "Checks if the current booted image is over 1 month old, returns exit code 77 if true.",
		PreRun: assertRoot,
		RunE:   ImageOutdated,
	}

	fLogFile    string
	fLogLevel   string
	fNoLogging  bool
	fLogJson    bool
	fConfigPath string
)

func Execute() {
	if err := initPre(); err != nil {
		slog.Error("Unable to init", slog.Any("error", err))
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command failed!", slog.Any("error", err))
		os.Exit(1)
	}
}

func initPre() error {
	if err := config.InitConfig(fConfigPath); err != nil {
		return fmt.Errorf("Failed to init config: %v", err)
	}
	if err := initLogging(); err != nil {
		return fmt.Errorf("Failed to init logging: %v", err)
	}
	return nil
}

func initLogging() error {
	logWriter := os.Stdout
	if fLogFile != "" {
		abs, err := filepath.Abs(path.Clean(fLogFile))
		if err != nil {
			return err
		}
		logWriter, err = os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
		if err != nil {
			return err
		}
	}

	logLevel, err := appLogging.StrToLogLevel(fLogLevel)
	if err != nil {
		return err
	}

	main_app_logger := slog.New(appLogging.SetupAppLogger(logWriter, logLevel, fLogFile != "" || fLogJson))

	if fNoLogging {
		slog.SetDefault(appLogging.NewMuteLogger())
	} else {
		slog.SetDefault(main_app_logger)
	}
	return nil
}

func init() {

	rootCmd.AddCommand(waitCmd)
	rootCmd.AddCommand(updateCheckCmd)
	rootCmd.AddCommand(hardwareCheckCmd)
	rootCmd.AddCommand(imageOutdatedCmd)

	// config flags
	rootCmd.Flags().Bool("disable-module-system", false, "Disable the System module")
	rootCmd.Flags().Bool("disable-module-flatpak", false, "Disable the Flatpak module")
	rootCmd.Flags().Bool("disable-module-distrobox", false, "Disable the Distrobox update module")
	rootCmd.Flags().Bool("disable-module-brew", false, "Disable the Brew update module")

	rootCmd.PersistentFlags().BoolVar(&fLogJson, "json", false, "Print logs as json")
	rootCmd.PersistentFlags().StringVar(&fLogFile, "log-file", "-", "File where user-facing logs will be written to")
	rootCmd.PersistentFlags().StringVar(&fLogLevel, "log-level", "info", "Log level for user-facing logs")
	// misc flags

	rootCmd.Flags().BoolP("force", "f", false, "Force system update without update checks")
	rootCmd.Flags().BoolP("dry-run", "n", false, "Do a dry run")
	// rootCmd.Flags().Bool("auto", false, "Indicate that this is an automatic update")
	rootCmd.PersistentFlags().StringVar(&fConfigPath, "config", config.DEFAULT_PATH, "Config file path")

	rootCmd.Flags().BoolP("verbose", "v", false, "Display command outputs after run")

	rootCmd.Flags().Bool("ci", false, "Makes some modifications to behavior if is running in CI")
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))
	rootCmd.Flags().Bool("disable-progress", !isTerminal, "Disable the GUI progress indicator, automatically disabled when loglevel is debug or in JSON")
	rootCmd.Flags().Bool("apply", false, "Reboot if there's an update to the image")
	rootCmd.PersistentFlags().BoolVar(&fNoLogging, "quiet", false, "Make logs quiet")

	_ = viper.BindPFlag("modules.flatpak.disable", rootCmd.Flags().Lookup("disable-module-flatpak"))
	_ = viper.BindPFlag("modules.brew.disable", rootCmd.Flags().Lookup("disable-module-brew"))
	_ = viper.BindPFlag("modules.system.disable", rootCmd.Flags().Lookup("disable-module-system"))
	_ = viper.BindPFlag("modules.distrobox.disable", rootCmd.Flags().Lookup("disable-module-distrobox"))
	_ = viper.BindPFlag("checks.hardware.enable", rootCmd.Flags().Lookup("hw-check"))

	// _ = viper.BindPFlag("update.force", rootCmd.Flags().Lookup("force"))
	// _ = viper.BindPFlag("update.verbose", rootCmd.Flags().Lookup("verbose"))

	_ = viper.BindPFlag("logging.json", rootCmd.PersistentFlags().Lookup("json"))
	_ = viper.BindPFlag("logging.file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("logging.quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}
