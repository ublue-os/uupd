package generic

import (
	"log/slog"
	"os"
	"strings"

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

func EnvOrFallback(environment EnvironmentMap, key string, fallback string) string {
	validCase, exists := environment[key]
	if exists && validCase != "" {
		return validCase
	}
	return fallback
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
	Stdout  string
	Failure bool
	Stderr  error
	Context string
	Cli     []string
}

func (output CommandOutput) New(out []byte, err error) *CommandOutput {
	return &CommandOutput{
		Context: "",
		Failure: err != nil,
		Stderr:  nil,
		Stdout:  string(out),
	}
}

type DriverConfiguration struct {
	Title           string
	Description     string
	Enabled         bool
	MultiUser       bool
	DryRun          bool
	Environment     EnvironmentMap `json:"-"`
	Logger          *slog.Logger   `json:"-"`
	UserDescription *string
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
