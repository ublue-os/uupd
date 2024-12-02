package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ublue-os/uupd/checks"
	"github.com/ublue-os/uupd/drv"
	"github.com/ublue-os/uupd/lib"
)

type Failure struct {
	Err    error
	Output string
}

func Update(cmd *cobra.Command, args []string) {
	lock, err := lib.AcquireLock()
	if err != nil {
		log.Fatalf("%v, is uupd already running?", err)
	}
	systemDriver, err := drv.GetSystemUpdateDriver()
	if err != nil {
		log.Fatalf("Failed to get system update driver: %v", err)
	}
	outdated, err := systemDriver.ImageOutdated()
	if err != nil {
		log.Fatalf("Unable to determine if image is outdated: %v", err)
	}
	if outdated {
		lib.Notify("System Warning", "There hasn't been an update in over a month. Consider rebooting or running updates manually")
		log.Printf("There hasn't been an update in over a month. Consider rebooting or running updates manually")
	}

	hwCheck, err := cmd.Flags().GetBool("hw-check")
	if err != nil {
		log.Fatalf("Failed to get hw-check flag: %v", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		log.Fatalf("Failed to get dry-run flag: %v", err)
	}

	if hwCheck {
		err := checks.RunHwChecks()
		if err != nil {
			log.Fatalf("Hardware checks failed: %v", err)
		}
		log.Println("Hardware checks passed")
	}

	users, err := lib.ListUsers()
	if err != nil {
		log.Fatalf("Failed to list users")
	}

	// Check if system update is available
	log.Printf("Checking for system updates (%s)", systemDriver.Name)
	updateAvailable, err := systemDriver.UpdateAvailable()
	// ignore error on dry run
	if err != nil && !dryRun {
		log.Fatalf("Failed to check for image updates: %v", err)
	}
	log.Printf("System updates available: %t (%s)", updateAvailable, systemDriver.Name)
	// don't update system if there's a dry run
	updateAvailable = updateAvailable && !dryRun
	systemUpdate := 0
	if updateAvailable {
		systemUpdate = 1
	}

	// Check if brew is installed
	brewUid, brewErr := drv.GetBrewUID()
	brewUpdate := 0
	if brewErr == nil {
		brewUpdate = 1
	}

	totalSteps := brewUpdate + systemUpdate + 1 + len(users) + 1 + len(users) // system + Brew + Flatpak (users + root) + Distrobox (users + root)
	currentStep := 0
	failures := make(map[string]Failure)

	if updateAvailable {
		currentStep++
		log.Printf("[%d/%d] Updating System (%s)", currentStep, totalSteps, systemDriver.Name)
		out, err := systemDriver.Update()
		if err != nil {
			failures[systemDriver.Name] = Failure{
				err,
				string(out),
			}
		}
	}

	if brewUpdate == 1 {
		currentStep++
		log.Printf("[%d/%d] Updating CLI Apps (Brew)", currentStep, totalSteps)
		out, err := drv.BrewUpdate(brewUid)
		if err != nil {
			failures["Brew"] = Failure{
				err,
				string(out),
			}
		}
	}

	// Run flatpak updates
	currentStep++
	log.Printf("[%d/%d] Updating System Apps (Flatpak)", currentStep, totalSteps)
	flatpakCmd := exec.Command("/usr/bin/flatpak", "update", "-y")
	out, err := flatpakCmd.CombinedOutput()
	if err != nil {
		failures["Flatpak"] = Failure{
			err,
			string(out),
		}
	}
	for _, user := range users {
		currentStep++
		log.Printf("[%d/%d] Updating Apps for User: %s (Flatpak)", currentStep, totalSteps, user.Name)
		out, err := lib.RunUID(user.UID, []string{"/usr/bin/flatpak", "update", "-y"}, nil)
		if err != nil {
			failures[fmt.Sprintf("Flatpak User: %s", user.Name)] = Failure{
				err,
				string(out),
			}
		}
	}

	// Run distrobox updates
	currentStep++
	log.Printf("[%d/%d] Updating System Distroboxes", currentStep, totalSteps)
	// distrobox doesn't support sudo, run with systemd-run
	out, err = lib.RunUID(0, []string{"/usr/bin/distrobox", "upgrade", "-a"}, nil)
	if err != nil {
		failures["Distrobox"] = Failure{
			err,
			string(out),
		}
	}
	for _, user := range users {
		currentStep++
		log.Printf("[%d/%d] Updating Distroboxes for User: %s", currentStep, totalSteps, user.Name)
		out, err := lib.RunUID(user.UID, []string{"/usr/bin/distrobox", "upgrade", "-a"}, nil)
		if err != nil {
			failures[fmt.Sprintf("Distrobox User: %s", user.Name)] = Failure{
				err,
				string(out),
			}
		}
	}

	if len(failures) > 0 {
		failedSystemsList := make([]string, 0, len(failures))
		for systemName := range failures {
			failedSystemsList = append(failedSystemsList, systemName)
		}
		failedSystemsStr := strings.Join(failedSystemsList, ", ")
		lib.Notify("Updates failed", fmt.Sprintf("uupd failed to update: %s, consider seeing logs with `journalctl -exu uupd.service`", failedSystemsStr))

		log.Printf("Updates Completed with Failures:")
		for name, fail := range failures {
			indentedOutput := "\t |  "
			lines := strings.Split(fail.Output, "\n")
			for i, line := range lines {
				if i > 0 {
					indentedOutput += "\n\t |  "
				}
				indentedOutput += line
			}
			log.Printf("---> %s \n\t | Failure error: %v \n\t | Command Output: \n%s", name, fail.Err, indentedOutput)
		}
		return
	}
	log.Printf("Updates Completed")
	lib.ReleaseLock(lock)
}
