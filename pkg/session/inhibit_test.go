package session

import (
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestReleaseInhibitorClosesDescriptor(t *testing.T) {
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(fds[0]) }()

	releaseInhibitor(dbus.UnixFD(fds[1]), nil)()

	if _, err := syscall.Write(fds[1], []byte("x")); err == nil {
		t.Fatal("inhibitor descriptor was not closed")
	}
}
