package session

import (
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	login1Dest    = "org.freedesktop.login1"
	login1Path    = "/org/freedesktop/login1"
	login1Manager = "org.freedesktop.login1.Manager"
)

// InhibitSleep holds a logind sleep lock until the returned function is called.
func InhibitSleep() (func(), error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	var fd dbus.UnixFD
	err = conn.Object(login1Dest, login1Path).
		Call(login1Manager+".Inhibit", 0, "sleep", "uupd", "System update in progress", "block").
		Store(&fd)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return releaseInhibitor(fd, conn), nil
}

func releaseInhibitor(fd dbus.UnixFD, conn *dbus.Conn) func() {
	return func() {
		_ = syscall.Close(int(fd))
		if conn != nil {
			_ = conn.Close()
		}
	}
}
