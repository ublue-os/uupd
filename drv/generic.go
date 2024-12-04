package drv

import (
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/ublue-os/uupd/lib"
)

type CommandOutput struct {
	Stdout  string
	Failure bool
	Stderr  error
	Context string
}

func (output CommandOutput) New(out []byte, err error) *CommandOutput {
	return &CommandOutput{
		Context: "",
		Failure: err != nil,
		Stderr:  nil,
		Stdout:  string(out),
	}
}

func (out *CommandOutput) SetFailureContext(context string) {
	out.Failure = true
	out.Context = context
}

type DriverConfiguration struct {
	Title           string
	Description     string
	Enabled         bool
	MultiUser       bool
	DryRun          bool
	UserDescription *string
}

type TrackerConfiguration struct {
	Tracker  *lib.IncrementTracker
	Writer   *progress.Writer
	Progress bool
}

type UpdateDriver interface {
	Steps() int
	Check() (*[]CommandOutput, error)
	Update() (*[]CommandOutput, error)
	New(dryrun bool) (*UpdateDriver, error)
}

type MultiUserUpdateDriver interface {
	*UpdateDriver
	SetUsers(users []lib.User)
}
