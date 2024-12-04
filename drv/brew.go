package drv

import (
	"fmt"
	"github.com/ublue-os/uupd/lib"
	"os"
	"syscall"
)

func (up BrewUpdater) GetBrewUID() (int, error) {
	inf, err := os.Stat(up.Environment["HOMEBREW_PREFIX"])
	if err != nil {
		return -1, err
	}

	if !inf.IsDir() {
		return -1, fmt.Errorf("Brew prefix: %v, is not a dir.", up.Environment["HOMEBREW_PREFIX"])
	}
	stat, ok := inf.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, fmt.Errorf("Unable to retriev UID info for %v", up.Environment["HOMEBREW_PREFIX"])
	}
	return int(stat.Uid), nil
}

func (up BrewUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up BrewUpdater) Check() (*[]CommandOutput, error) {
	// TODO: implement
	return nil, nil
}

func (up BrewUpdater) Update() (*[]CommandOutput, error) {
	var final_output = []CommandOutput{}

	if up.Config.DryRun {
		return &final_output, nil
	}

	out, err := lib.RunUID(up.BaseUser, []string{up.Environment["HOMEBREW_PATH"], "update"}, up.Environment)
	tmpout := CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("Brew update")
		final_output = append(final_output, *tmpout)
		return &final_output, err
	}

	out, err = lib.RunUID(up.BaseUser, []string{up.Environment["HOMEBREW_PATH"], "upgrade"}, up.Environment)
	tmpout = CommandOutput{}.New(out, err)
	if err != nil {
		tmpout.SetFailureContext("Brew upgrade")
	}
	final_output = append(final_output, *tmpout)
	return &final_output, err
}

type BrewUpdater struct {
	Config      DriverConfiguration
	BaseUser    int
	Environment map[string]string
}

func (up BrewUpdater) New(config UpdaterInitConfiguration) (BrewUpdater, error) {
	brewPrefix, empty := os.LookupEnv("HOMEBREW_PREFIX")
	if empty || brewPrefix == "" {
		brewPrefix = "/home/linuxbrew/.linuxbrew"
	}
	brewRepo, empty := os.LookupEnv("HOMEBREW_REPOSITORY")
	if empty || brewRepo == "" {
		brewRepo = fmt.Sprintf("%s/Homebrew", brewPrefix)
	}
	brewCellar, empty := os.LookupEnv("HOMEBREW_CELLAR")
	if empty || brewCellar == "" {
		brewCellar = fmt.Sprintf("%s/Cellar", brewPrefix)
	}
	brewPath, empty := os.LookupEnv("HOMEBREW_PATH")
	if empty || brewPath == "" {
		brewPath = fmt.Sprintf("%s/bin/brew", brewPrefix)
	}

	up.Environment = map[string]string{
		"HOMEBREW_PREFIX":     brewPrefix,
		"HOMEBREW_REPOSITORY": brewRepo,
		"HOMEBREW_CELLAR":     brewCellar,
		"HOMEBREW_PATH":       brewPath,
	}
	up.Config = DriverConfiguration{
		Title:       "Brew",
		Description: "CLI Apps",
		Enabled:     true,
		MultiUser:   false,
		DryRun:      config.DryRun,
	}

	if up.Config.DryRun {
		return up, nil
	}

	uid, err := up.GetBrewUID()
	if err != nil {
		return up, err
	}

	up.BaseUser = uid
	return up, nil
}
