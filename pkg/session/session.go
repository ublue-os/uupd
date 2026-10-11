package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const notifyTimeout = 15 * time.Second

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
	var output strings.Builder
	multiReader := io.TeeReader(io.MultiReader(stdout, stderr), &output)
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
		return []byte(output.String()), err
	}

	return []byte(output.String()), scanner.Err()
}

func RunAsUser(logger *slog.Logger, level slog.Level, name string, command []string) ([]byte, error) {
	cmdArgs := append([]string{
		"/usr/bin/pkexec",
		"-u",
		name,
	}, command...)

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
	defer conn.Close() //nolint:errcheck

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

	return filterUsers(users, slog.Default()), nil
}

func filterUsers(users []User, logger *slog.Logger) []User {
	updateTargets := make([]User, 0, len(users))

	for _, user := range users {
		if user.UID == 0 {
			continue
		}

		record, err := LookupPasswd(user.UID)
		if err != nil {
			logger.Warn("Skipping user that cannot be resolved, this is not a failed update",
				slog.Int("uid", user.UID),
				slog.String("name", user.Name),
				slog.Any("error", err),
			)
			continue
		}

		if !IsUpdateTarget(record) {
			logger.Debug("Skipping non-interactive user",
				slog.Int("uid", user.UID),
				slog.String("name", user.Name),
				slog.String("home", record.Home),
			)
			continue
		}

		updateTargets = append(updateTargets, user)
	}

	return updateTargets
}

func Notify(users []User, summary string, body string, urgency string) error {
	for _, user := range users {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		cmd := exec.CommandContext(ctx, "/usr/bin/machinectl", "shell", fmt.Sprintf("%d@", user.UID), "/usr/bin/notify-send", "--urgency", urgency, "--app-name", "uupd", summary, body)
		// we don't care if these exit
		if err := cmd.Run(); err != nil {
			slog.Debug("Failed sending notification to user", slog.Int("uid", user.UID), slog.Any("error", err))
		}
		cancel()
	}
	return nil
}
