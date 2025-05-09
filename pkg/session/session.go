package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	osUser "os/user"

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
	actualLogger := slog.Default()
	err := command.Start()
	if err != nil {
		actualLogger.Warn("Error occurred starting external command", slog.Any("error", err))
		return []byte{}, err
	}
	scanner := bufio.NewScanner(multiReader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		actualLogger.Log(context.TODO(), level, scanner.Text())
	}
	err = command.Wait()
	if err != nil {
		actualLogger.Warn("Error occurred while waiting for external command", slog.Any("error", err))
		return []byte{}, err
	}

	return scanner.Bytes(), scanner.Err()
}

func RunUID(logger *slog.Logger, level slog.Level, uid int, command []string, env map[string]string) ([]byte, error) {
	user, err := osUser.LookupId(fmt.Sprintf("%d", uid))

	if err != nil {
		return []byte{}, fmt.Errorf("Failed to lookup UID: %d, returned error: %v", uid, err)
	}
	cmdArgs := []string{
		"/usr/bin/pkexec",
		"-u",
		user.Username,
	}
	cmdArgs = append(cmdArgs, command...)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	return RunLog(logger, level, cmd)
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
		if parsed.UID == 0 {
			continue
		}

		users = append(users, parsed)
	}
	return users, nil
}

func Notify(users []User, summary string, body string, urgency string) error {
	for _, user := range users {
		// we don't care if these exit
		cmd := exec.Command("/usr/bin/machinectl", "shell", fmt.Sprintf("%d@", user.UID), "/usr/bin/notify-send", "--urgency", urgency, "--app-name", "uupd", summary, body)
		_ = cmd.Run()
	}
	return nil
}
