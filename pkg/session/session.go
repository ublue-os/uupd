package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

type User struct {
	UID  int
	Name string
}

// Runs any specified Command while logging it to the logger
// Made to work just like (Command).CombinedOutput()
func RunLog(logger *slog.Logger, level slog.Level, command *exec.Cmd) ([]byte, error) {
	if logger == nil {
		return command.CombinedOutput()
	}

	stdout, _ := command.StdoutPipe()
	stderr, _ := command.StderrPipe()
	multiReader := io.MultiReader(stdout, stderr)
	actuallogger := slog.Default()

	if err := command.Start(); err != nil {
		actuallogger.Warn("Error occoured starting external command", slog.Any("error", err))
	}
	scanner := bufio.NewScanner(multiReader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		actuallogger.Log(context.TODO(), level, scanner.Text())
	}
	if err := command.Wait(); err != nil {
		actuallogger.Warn("Error occoured while waiting for external command", slog.Any("error", err))
	}

	return scanner.Bytes(), scanner.Err()
}

func RunUID(logger *slog.Logger, level slog.Level, uid int, command []string, env map[string]string) ([]byte, error) {
	// make a file to store the exit code (machinectl shell doesn't pass through the exit code)
	exitCodeFile, err := os.CreateTemp("/tmp", "exitcode_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary exit code file: %v", err)
	}
	// err = os.Chmod(exitCodeFile.Name(), 0666) // Allow anyone to read and write
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to set permissions on exit code file: %v", err)
	// }
	defer os.Remove(exitCodeFile.Name())

	cmdArgs := []string{
		"/usr/bin/machinectl",
		"shell",
		"--quiet",
		fmt.Sprintf("%d@", uid),
		"/bin/bash",
		"-c",
	}

	commandWithExitCodeCapture := fmt.Sprintf(
		"%s; echo $? > %s",
		strings.Join(command, " "),
		exitCodeFile.Name(),
	)

	cmdArgs = append(cmdArgs, commandWithExitCodeCapture)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	output, err := cmd.CombinedOutput()

	exitCodeData, _ := os.ReadFile(exitCodeFile.Name())
	exitCode := string(exitCodeData)

	if exitCode != "0" {
		return output, fmt.Errorf("command failed with exit code %s", exitCode)
	}

	return output, nil
}

func ParseUserFromVariant(uidVariant dbus.Variant, nameVariant dbus.Variant) (User, error) {
	uid, ok := uidVariant.Value().(uint32)
	if !ok {
		return User{}, fmt.Errorf("invalid UID type, expected uint32")
	}

	name, ok := nameVariant.Value().(string)
	if !ok {
		return User{}, fmt.Errorf("invalid Name type, expected string")
	}

	return User{
		UID:  int(uid),
		Name: name,
	}, nil
}

func ListUsers() ([]User, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return []User{}, fmt.Errorf("failed to connect to system bus: %v", err)
	}
	defer conn.Close()

	var resp [][]dbus.Variant
	object := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
	err = object.Call("org.freedesktop.login1.Manager.ListUsers", 0).Store(&resp)
	if err != nil {
		return []User{}, err
	}

	var users []User
	for _, data := range resp {
		parsed, err := ParseUserFromVariant(data[0], data[1])
		if err != nil {
			return nil, err
		}

		users = append(users, parsed)
	}
	return users, nil
}

func Notify(users []User, summary string, body string) error {
	for _, user := range users {
		// we don't care if these exit
		_, _ = RunUID(nil, slog.LevelDebug, user.UID, []string{"/usr/bin/notify-send", "--app-name", "uupd", summary, body}, nil)
	}
	return nil
}
