package session

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func runLogWithDeadline(t *testing.T, command *exec.Cmd, deadline time.Duration) ([]byte, error) {
	t.Helper()

	type result struct {
		out []byte
		err error
	}

	finished := make(chan result, 1)
	go func() {
		out, err := RunLog(discardLogger, -4, command)
		finished <- result{out: out, err: err}
	}()

	select {
	case got := <-finished:
		return got.out, got.err
	case <-time.After(deadline):
		_ = command.Process.Kill()
		t.Fatalf("RunLog did not return within %v: the child's pipes are not being drained concurrently", deadline)
		return nil, nil
	}
}

func TestRunLogCapturesStdoutAndStderr(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c", "echo to-stdout; echo to-stderr >&2")

	out, err := runLogWithDeadline(t, command, 30*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v (output: %q)", err, out)
	}

	for _, want := range []string{"to-stdout", "to-stderr"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("Expected %q in captured output, got %q", want, out)
		}
	}
}

func TestRunLogDrainsLargeStderrWithoutDeadlocking(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c",
		`i=0; while [ $i -lt 3000 ]; do echo "flatpak: Info: updating runtime/whatever/$i" >&2; i=$((i+1)); done; echo done-on-stdout`)

	out, err := runLogWithDeadline(t, command, 30*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !strings.Contains(string(out), "done-on-stdout") {
		t.Fatalf("Expected stdout in captured output, got %q", out)
	}
	if !strings.Contains(string(out), "runtime/whatever/2999") {
		t.Fatalf("Expected the tail of stderr in captured output, got %q", out)
	}
	if count := strings.Count(string(out), "flatpak: Info:"); count != 3000 {
		t.Fatalf("Expected all 3000 stderr lines to be captured, got %d", count)
	}
}

func TestRunLogHandlesLinesLongerThanScannerLimit(t *testing.T) {
	t.Parallel()
	const width = 200 * 1024
	command := exec.Command("/bin/sh", "-c", `head -c 204800 /dev/zero | tr '\0' 'x'; echo`)

	out, err := runLogWithDeadline(t, command, 30*time.Second)
	if err != nil {
		t.Fatalf("Expected no error for a successful command, got %v", err)
	}
	if len(out) < width {
		t.Fatalf("Expected at least %d bytes of captured output, got %d", width, len(out))
	}
}

func TestRunLogReportsNonZeroExit(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c", "echo partial-output; exit 3")

	out, err := runLogWithDeadline(t, command, 30*time.Second)
	if err == nil {
		t.Fatal("Expected an error for a command exiting 3")
	}
	if !strings.Contains(string(out), "partial-output") {
		t.Fatalf("Expected output from a failing command to be captured, got %q", out)
	}
}

func TestRunLogCapturesOutputForFailingCommand(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c", "echo failure-detail >&2; exit 1")

	out, err := runLogWithDeadline(t, command, 30*time.Second)
	if err == nil {
		t.Fatal("Expected an error for a failing command")
	}
	if !strings.Contains(string(out), "failure-detail") {
		t.Fatalf("Expected stderr from a failing command to be captured, got %q", out)
	}
}

func TestRunLogHandlesMissingBinary(t *testing.T) {
	t.Parallel()
	command := exec.Command("/definitely/not/a/real/binary")

	if _, err := runLogWithDeadline(t, command, 30*time.Second); err == nil {
		t.Fatal("Expected an error for a missing binary")
	}
}

func TestRunLogEmitsCommandOutputAtInfoLevelForJSONLogging(t *testing.T) {
	t.Parallel()
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	command := exec.Command("/bin/sh", "-c", "echo visible-in-json")

	if _, err := RunLog(logger, slog.LevelDebug, command); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buffer.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("Failed decoding log record %q: %v", line, err)
		}
		records = append(records, record)
	}

	for _, record := range records {
		if record["output"] == "visible-in-json" {
			if record["status"] != "running" {
				t.Fatalf("Expected a running status, got %v", record)
			}
			if record["level"] != "INFO" {
				t.Fatalf("Expected command output at info level, got %v", record)
			}
			return
		}
	}
	t.Fatalf("Expected command output in the JSON logs, got %v", records)
}

func TestRunLogKeepsCommandOutputAtDebugLevelForTextLogging(t *testing.T) {
	t.Parallel()
	buffer := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buffer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	command := exec.Command("/bin/sh", "-c", "echo hidden-without-debug")

	if _, err := RunLog(logger, slog.LevelDebug, command); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if strings.Contains(buffer.String(), "hidden-without-debug") {
		t.Fatalf("Expected command output to stay at debug level for text logs, got %q", buffer.String())
	}
}

func TestPkexecArgsInterposesEnv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		description string
		user        string
		command     []string
		expected    []string
	}{
		{
			"brew upgrade",
			"linuxbrew",
			[]string{"/home/linuxbrew/.linuxbrew/bin/brew", "upgrade", "-y"},
			[]string{"/usr/bin/pkexec", "-u", "linuxbrew", "/usr/bin/env", "/home/linuxbrew/.linuxbrew/bin/brew", "upgrade", "-y"},
		},
		{
			"per-user flatpak update",
			"testuser",
			[]string{"/usr/bin/flatpak", "update", "-y"},
			[]string{"/usr/bin/pkexec", "-u", "testuser", "/usr/bin/env", "/usr/bin/flatpak", "update", "-y"},
		},
		{
			"distrobox upgrade",
			"testuser",
			[]string{"/usr/bin/distrobox", "upgrade", "-a"},
			[]string{"/usr/bin/pkexec", "-u", "testuser", "/usr/bin/env", "/usr/bin/distrobox", "upgrade", "-a"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			got := pkexecArgs(testCase.user, testCase.command)

			if len(got) != len(testCase.expected) {
				t.Fatalf("pkexecArgs(%v, %v) = %v, expected %v", testCase.user, testCase.command, got, testCase.expected)
			}
			for i := range got {
				if got[i] != testCase.expected[i] {
					t.Fatalf("pkexecArgs(%v, %v) = %v, expected %v", testCase.user, testCase.command, got, testCase.expected)
				}
			}
		})
	}
}

func TestPkexecArgsPreservesTheProgramPath(t *testing.T) {
	t.Parallel()
	const program = "/home/linuxbrew/.linuxbrew/bin/brew"

	got := pkexecArgs("linuxbrew", []string{program, "outdated"})
	if got[4] != program {
		t.Fatalf("Expected the program path %q to be passed through untouched, got %q", program, got[4])
	}
}
