package session

import (
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestInhibitTypesCoverSleepAndLidSwitch(t *testing.T) {
	for _, want := range []string{"sleep", "handle-lid-switch"} {
		found := false
		for _, got := range inhibitTypes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("inhibitTypes is missing %q: %v", want, inhibitTypes)
		}
	}
}

func TestReleaseInhibitorsClosesDescriptors(t *testing.T) {
	var first [2]int
	var second [2]int
	if err := syscall.Pipe(first[:]); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Pipe(second[:]); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(first[0]) }()
	defer func() { _ = syscall.Close(second[0]) }()

	fds := []dbus.UnixFD{dbus.UnixFD(first[1]), dbus.UnixFD(second[1])}
	releaseInhibitors(fds)()

	if _, err := syscall.Write(first[1], []byte("x")); err == nil {
		t.Fatal("first inhibitor descriptor was not closed")
	}
	if _, err := syscall.Write(second[1], []byte("x")); err == nil {
		t.Fatal("second inhibitor descriptor was not closed")
	}
}
