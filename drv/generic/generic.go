package generic

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ublue-os/uupd/pkg/session"
)

type EnvironmentMap map[string]string

type UpdaterInitConfiguration struct {
	DryRun      bool
	Ci          bool
	Verbose     bool
	Environment EnvironmentMap
	Logger      *slog.Logger
}

func GetEnvironment(data []string) map[string]string {
	items := make(map[string]string)
	for _, item := range data {
		splits := strings.Split(item, "=")
		items[splits[0]] = splits[1]
	}
	return items
}

func (up UpdaterInitConfiguration) New() *UpdaterInitConfiguration {
	up.DryRun = false
	up.Ci = false
	up.Environment = GetEnvironment(os.Environ())
	up.Logger = slog.Default()

	return &up
}

type CommandOutput struct {
	Stdout   string   `json:"stdout"`
	Failure  bool     `json:"failure"`
	Stderr   error    `json:"-"`
	Context  string   `json:"context"`
	Cli      []string `json:"cli"`
	Module   string   `json:"module"`
	ExitCode int      `json:"exit_code"`
	// Skipped packages that need sudo; not counted as failures.
	NeedsManualUpdate []string `json:"needs_manual_update,omitempty"`
}

func (output CommandOutput) New(out []byte, err error) *CommandOutput {
	return &CommandOutput{
		Failure:  err != nil,
		ExitCode: exitCode(err),
		Stdout:   string(out),
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

type DriverConfiguration struct {
	Title                 string
	Description           string
	Enabled               bool
	MultiUser             bool
	DryRun                bool
	OutdatedThresholdDays int
	Environment           EnvironmentMap `json:"-"`
	Logger                *slog.Logger   `json:"-"`
	UserDescription       *string
}

func OutdatedCutoff(days int) time.Time {
	if days <= 0 {
		return time.Now().AddDate(0, -1, 0).UTC()
	}
	return time.Now().AddDate(0, 0, -days).UTC()
}

type UpdateDriver interface {
	Steps() int
	Check() (bool, error)
	Update() (*[]CommandOutput, error)
	Config() *DriverConfiguration
	SetEnabled(value bool)
	Logger() *slog.Logger
	SetLogger(value *slog.Logger)
}

type MultiUserUpdateDriver interface {
	*UpdateDriver
	SetUsers(users []session.User)
}
