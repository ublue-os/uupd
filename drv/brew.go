package drv

import (
	"fmt"
	"github.com/ublue-os/uupd/lib"
	"os"
	"syscall"
)

func (up BrewUpdater) GetBrewUID() (int, error) {
	inf, err := os.Stat(up.BrewPrefix)
	if err != nil {
		return -1, err
	}

	if !inf.IsDir() {
		return -1, fmt.Errorf("Brew prefix: %v, is not a dir.", up.BrewPrefix)
	}
	stat, ok := inf.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, fmt.Errorf("Unable to retriev UID info for %v", up.BrewPrefix)
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

	cli := []string{up.BrewPath, "update"}
	out, err := lib.RunUID(up.BaseUser, cli, up.Environment)
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Context = "Brew Update"
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	if err != nil {
		tmpout.SetFailureContext("Brew update")
		final_output = append(final_output, *tmpout)
		return &final_output, err
	}

	cli = []string{up.BrewPath, "upgrade"}
	out, err = lib.RunUID(up.BaseUser, cli, up.Environment)
	tmpout = CommandOutput{}.New(out, err)
	tmpout.Context = "Brew Upgrade"
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	final_output = append(final_output, *tmpout)
	return &final_output, err
}

type BrewUpdater struct {
	Config      DriverConfiguration
	BaseUser    int
	Environment EnvironmentMap
	BrewRepo    string
	BrewPrefix  string
	BrewCellar  string
	BrewPath    string
}

func (up BrewUpdater) New(config UpdaterInitConfiguration) (BrewUpdater, error) {
	up.Environment = config.Environment

	brewPrefix, exists := up.Environment["HOMEBREW_PREFIX"]
	if !exists || brewPrefix == "" {
		up.BrewPrefix = "/home/linuxbrew/.linuxbrew"
	} else {
		up.BrewPrefix = brewPrefix
	}
	brewRepo, exists := up.Environment["HOMEBREW_REPOSITORY"]
	if !exists || brewRepo == "" {
		up.BrewRepo = fmt.Sprintf("%s/Homebrew", up.BrewPrefix)
	} else {
		up.BrewRepo = brewRepo
	}
	brewCellar, exists := up.Environment["HOMEBREW_CELLAR"]
	if !exists || brewCellar == "" {
		up.BrewCellar = fmt.Sprintf("%s/Cellar", up.BrewPrefix)
	} else {
		up.BrewCellar = brewCellar
	}
	brewPath, exists := up.Environment["HOMEBREW_PATH"]
	if !exists || brewPath == "" {
		up.BrewPath = fmt.Sprintf("%s/bin/brew", up.BrewPrefix)
	} else {
		up.BrewPath = brewPath
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
