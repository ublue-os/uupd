package session

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

// InhibitSleep takes a logind "sleep" block lock, so the system does not
// suspend or hibernate while it is held. The lock lasts until the returned
// release function is called or the process exits.
//
// logind's default LidSwitchIgnoreInhibited=yes means a lid close handled by
// logind itself still suspends; idle suspend and a requested suspend respect
// the lock.
func InhibitSleep(who, why string) (release func() error, err error) {
	// A private connection: ListUsers and the hardware checks close the
	// shared system bus connection when they finish.
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}
	// The lock belongs to the returned file descriptor, not the connection.
	defer conn.Close() //nolint:errcheck

	var fd dbus.UnixFD
	object := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
	err = object.Call("org.freedesktop.login1.Manager.Inhibit", 0, "sleep", who, why, "block").Store(&fd)
	if err != nil {
		return nil, fmt.Errorf("failed to take sleep inhibitor: %w", err)
	}

	lock := os.NewFile(uintptr(fd), "logind-sleep-inhibitor")
	return lock.Close, nil
}
