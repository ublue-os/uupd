package brew

import (
	"slices"
	"testing"

	"github.com/ublue-os/uupd/drv/generic"
)

func TestSudoSkippedCasks(t *testing.T) {
	t.Parallel()

	install := "Error: asusctl-linux: The postflight_steps artifact requires sudo, but HOMEBREW_NO_SUDO is set.\n"
	uninstall := "Error: 1password-cli-linux: Failure while executing; `/usr/bin/sudo -E -- /bin/sh -eu -c ...` exited with 1. Here's the output:\n" +
		"sudo is disabled by HOMEBREW_NO_SUDO.\n"

	cases := []struct {
		description string
		output      string
		casks       []string
		onlySudo    bool
	}{
		{"install skip", "==> Upgrading uv\n" + install, []string{"asusctl-linux"}, true},
		{"uninstall skip", "==> Upgrading uv\n" + uninstall, []string{"1password-cli-linux"}, true},
		{"both kinds, deduped", install + uninstall + install, []string{"asusctl-linux", "1password-cli-linux"}, true},
		{"sudo and other error", install + "Error: node: Failure while executing download\n", []string{"asusctl-linux"}, false},
		{"other error only", "Error: node: Failure while executing download\n", nil, false},
		{"no errors", "==> Upgrading uv\n", nil, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			casks, onlySudo := sudoSkippedCasks(testCase.output)
			if !slices.Equal(casks, testCase.casks) || onlySudo != testCase.onlySudo {
				t.Fatalf("sudoSkippedCasks() = %v, %v, expected %v, %v", casks, onlySudo, testCase.casks, testCase.onlySudo)
			}
		})
	}
}

func TestMarkSudoSkips(t *testing.T) {
	t.Parallel()

	const skipped = "Error: asusctl-linux: The postflight_steps artifact requires sudo, but HOMEBREW_NO_SUDO is set.\n"
	cases := []struct {
		description string
		output      generic.CommandOutput
		marked      bool
		casks       []string
	}{
		{"sudo skip, brew exit 1", generic.CommandOutput{Stdout: skipped, Failure: true, ExitCode: 1}, true, []string{"asusctl-linux"}},
		{"sudo skip and other error", generic.CommandOutput{Stdout: skipped + "Error: node: Failure while executing download\n", Failure: true, ExitCode: 1}, false, []string{"asusctl-linux"}},
		{"sudo skip, killed by a signal", generic.CommandOutput{Stdout: skipped, Failure: true, ExitCode: -1}, false, nil},
		{"other error, brew exit 1", generic.CommandOutput{Stdout: "Error: node: Failure while executing download\n", Failure: true, ExitCode: 1}, false, nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			output := testCase.output
			marked := markSudoSkips(&output)
			if marked != testCase.marked {
				t.Fatalf("markSudoSkips() = %v, expected %v", marked, testCase.marked)
			}
			if !slices.Equal(output.NeedsManualUpdate, testCase.casks) {
				t.Fatalf("NeedsManualUpdate = %v, expected %v", output.NeedsManualUpdate, testCase.casks)
			}
			if marked && (output.Failure || output.ExitCode != 0) {
				t.Fatalf("Expected a successful output, got %+v", output)
			}
			if !marked && !output.Failure {
				t.Fatalf("Expected the output to stay a failure, got %+v", output)
			}
		})
	}
}
