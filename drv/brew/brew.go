package brew

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	. "github.com/ublue-os/uupd/drv/generic"
	appConfig "github.com/ublue-os/uupd/pkg/config"
	"github.com/ublue-os/uupd/pkg/percent"
	"github.com/ublue-os/uupd/pkg/session"
)

var sudoSkipPattern = regexp.MustCompile(`^Error: (\S+): The \S+ artifact requires sudo, but HOMEBREW_NO_SUDO is set\.$`)

const sudoDisabledMessage = "sudo is disabled by HOMEBREW_NO_SUDO"

// sudoSkippedCasks finds casks refused because sudo is disabled: install
// artifacts are skipped, uninstall artifacts report sudoDisabledMessage.
func sudoSkippedCasks(output string) (casks []string, onlySudo bool) {
	type errorBlock struct {
		cask string
		sudo bool
	}

	blocks := []errorBlock{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Error:") {
			block := errorBlock{}
			body := strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
			if match := sudoSkipPattern.FindStringSubmatch(line); match != nil {
				block.cask, block.sudo = match[1], true
			} else if name, _, found := strings.Cut(body, ":"); found {
				block.cask = strings.TrimSpace(name)
			}
			if strings.Contains(line, sudoDisabledMessage) {
				block.sudo = true
			}
			blocks = append(blocks, block)
			continue
		}
		if len(blocks) > 0 && strings.Contains(line, sudoDisabledMessage) {
			blocks[len(blocks)-1].sudo = true
		}
	}

	onlySudo = len(blocks) > 0
	seen := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		if block.cask == "" || !block.sudo {
			onlySudo = false
			continue
		}
		if !seen[block.cask] {
			seen[block.cask] = true
			casks = append(casks, block.cask)
		}
	}
	return casks, onlySudo && len(casks) > 0
}

// markSudoSkips records sudo-skipped casks and reports if they are the only errors.
func markSudoSkips(output *CommandOutput) bool {
	if output.ExitCode != 1 {
		return false
	}
	casks, onlySudo := sudoSkippedCasks(output.Stdout)
	if len(casks) == 0 {
		return false
	}
	output.NeedsManualUpdate = casks
	if !onlySudo {
		return false
	}
	output.Failure = false
	output.ExitCode = 0
	return true
}

func (up BrewUpdater) GetBrewUID() (int, error) {
	inf, err := os.Stat(up.BrewPrefix)
	if err != nil {
		return -1, err
	}

	if !inf.IsDir() {
		return -1, fmt.Errorf("brew prefix: %v, is not a dir", up.BrewPrefix)
	}
	stat, ok := inf.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, fmt.Errorf("unable to retriev UID info for %v", up.BrewPrefix)
	}
	return int(stat.Uid), nil
}

func (up BrewUpdater) Steps() int {
	if up.Config.Enabled {
		return 1
	}
	return 0
}

func (up BrewUpdater) Check() (bool, error) {
	return true, nil
}

func (up BrewUpdater) Update(_tracker *percent.Incrementer) (*[]CommandOutput, error) {
	var final_output = []CommandOutput{}

	if up.Config.DryRun {
		return &final_output, nil
	}

	cli := []string{up.BrewPath, "update"}
	out, err := session.RunAsUser(up.Config.Logger, slog.LevelDebug, up.BaseUserName, cli)
	tmpout := CommandOutput{}.New(out, err)
	tmpout.Context = "Brew Update"
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	if failure := err != nil; failure {
		tmpout.Failure = failure
		tmpout.Context = "Brew Update"
		final_output = append(final_output, *tmpout)
		return &final_output, err
	}

	// No TTY for sudo prompts, so make brew skip sudo casks early.
	cli = []string{up.BrewPath, "upgrade", "-y"}
	out, err = session.RunAsUserWithEnv(up.Config.Logger, slog.LevelDebug, up.BaseUserName, []string{"HOMEBREW_NO_SUDO=1"}, cli)
	tmpout = CommandOutput{}.New(out, err)
	tmpout.Context = "Brew Upgrade"
	tmpout.Cli = cli
	tmpout.Failure = err != nil
	if err != nil {
		onlySudo := markSudoSkips(tmpout)
		if len(tmpout.NeedsManualUpdate) > 0 {
			// Bazzite Updater shows only the message text, not attributes.
			up.Config.Logger.Warn(
				fmt.Sprintf("Some casks need sudo and were skipped; upgrade them in a terminal: brew upgrade --cask %s", strings.Join(tmpout.NeedsManualUpdate, " ")),
				slog.Any("casks", tmpout.NeedsManualUpdate),
			)
		}
		if onlySudo {
			err = nil
		}
	}
	final_output = append(final_output, *tmpout)
	return &final_output, err
}

type BrewUpdater struct {
	Config       DriverConfiguration
	BaseUser     int
	BaseUserName string
	BrewRepo     string
	BrewPrefix   string
	BrewCellar   string
	BrewPath     string
}

func (up BrewUpdater) New(config UpdaterInitConfiguration) (BrewUpdater, error) {
	conf := appConfig.Get().Modules.Brew
	up.Config = DriverConfiguration{
		Title:       "Brew",
		Description: "CLI Apps",
		Enabled:     true,
		MultiUser:   false,
		DryRun:      config.DryRun,
		Environment: config.Environment,
	}
	up.Config.Logger = config.Logger.With(slog.String("module", strings.ToLower(up.Config.Title)))

	up.BrewPrefix = conf.Prefix
	up.BrewRepo = conf.Repository
	up.BrewCellar = conf.Cellar
	up.BrewPath = conf.Path

	if up.Config.DryRun {
		return up, nil
	}

	uid, err := up.GetBrewUID()
	if err != nil {
		return up, err
	}
	up.BaseUser = uid

	name, err := session.LookupUserName(uid)
	if err != nil {
		slog.Warn("Could not resolve brew owner name, falling back to uid", slog.Int("uid", uid), slog.Any("error", err))
		name = strconv.Itoa(uid)
	}
	up.BaseUserName = name

	return up, nil
}
