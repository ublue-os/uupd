package drv

import (
	"fmt"
	"github.com/ublue-os/uupd/lib"
	"os"
	"syscall"
)

var (
	brewPrefix = "/home/linuxbrew/.linuxbrew"
	brewCellar = fmt.Sprintf("%s/Cellar", brewPrefix)
	brewRepo   = fmt.Sprintf("%s/Homebrew", brewPrefix)
	brewPath   = fmt.Sprintf("%s/bin/brew", brewPrefix)
)

func GetBrewUID() (int, error) {
	inf, err := os.Stat(brewPrefix)
	if err != nil {
		return -1, err
	}

	if !inf.IsDir() {
		return -1, fmt.Errorf("Brew prefix: %v, is not a dir.", brewPrefix)
	}
	stat, ok := inf.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, fmt.Errorf("Unable to retriev UID info for %v", brewPrefix)
	}
	return int(stat.Uid), nil
}

func BrewUpdate(uid int) ([]byte, error) {
	env := map[string]string{
		"HOMEBREW_PREFIX":     brewPrefix,
		"HOMEBREW_REPOSITORY": brewRepo,
		"HOMEBREW_CELLAR":     brewCellar,
	}
	out, err := lib.RunUID(uid, []string{brewPath, "update"}, env)
	if err != nil {
		return out, err
	}

	out, err = lib.RunUID(uid, []string{brewPath, "upgrade"}, env)
	if err != nil {
		return out, err
	}

	return out, nil
}
