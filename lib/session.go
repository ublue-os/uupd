package lib

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"os/exec"
)

type User struct {
	UID  int
	Name string
}

func RunUID(uid int, command []string, env map[string]string) ([]byte, error) {
	// Just fork systemd-run, using the systemd API gave me a massive headache
	// FIXME: use the systemd api instead
	cmdArgs := []string{
		"/usr/bin/systemd-run",
		"--machine",
		fmt.Sprintf("%d@", uid),
		"--pipe",
		"--quiet",
	}
	if uid != 0 {
		cmdArgs = append(cmdArgs, "--user")
	}
	cmdArgs = append(cmdArgs, command...)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	return cmd.CombinedOutput()
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
		if len(data) < 2 {
			return []User{}, fmt.Errorf("Malformed dbus response")
		}
		uidVariant := data[0]
		nameVariant := data[1]

		uid, ok := uidVariant.Value().(uint32)
		if !ok {
			return []User{}, fmt.Errorf("invalid UID type, expected uint32")
		}

		name, ok := nameVariant.Value().(string)
		if !ok {
			return []User{}, fmt.Errorf("invalid Name type, expected string")
		}

		users = append(users, User{
			UID:  int(uid),
			Name: name,
		})
	}
	return users, nil
}

func Notify(summary string, body string) error {
	users, err := ListUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		// we don't care if these exit
		_, _ = RunUID(user.UID, []string{"/usr/bin/notify-send", "--app-name", "uupd", summary, body}, nil)
	}
	return nil
}
