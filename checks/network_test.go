package checks

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitForNetworkReturnsImmediatelyWhenOnline(t *testing.T) {
	calls := 0
	err := waitForNetwork(time.Second, time.Millisecond, func() (uint32, error) {
		calls++
		return nmConnectivityFull, nil
	})
	if err != nil {
		t.Fatalf("expected no error when the network is online, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected a single connectivity check, got %d", calls)
	}
}

func TestWaitForNetworkRetriesUntilOnline(t *testing.T) {
	calls := 0
	err := waitForNetwork(time.Second, time.Millisecond, func() (uint32, error) {
		calls++
		if calls < 3 {
			// NMConnectivityStateLimited - link is up, no internet yet.
			return 1, nil
		}
		return nmConnectivityFull, nil
	})
	if err != nil {
		t.Fatalf("expected no error once the network comes online, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 connectivity checks, got %d", calls)
	}
}

func TestWaitForNetworkTimesOut(t *testing.T) {
	err := waitForNetwork(10*time.Millisecond, time.Millisecond, func() (uint32, error) {
		return 1, nil
	})
	if err == nil {
		t.Fatal("expected an error when the network never comes online")
	}
	if !strings.Contains(err.Error(), "network not online") {
		t.Fatalf("expected a 'network not online' error, got %v", err)
	}
}

func TestWaitForNetworkSurfacesCheckErrorOnTimeout(t *testing.T) {
	checkErr := errors.New("dbus unavailable")
	err := waitForNetwork(10*time.Millisecond, time.Millisecond, func() (uint32, error) {
		return 0, checkErr
	})
	if !errors.Is(err, checkErr) {
		t.Fatalf("expected the underlying connectivity error to be wrapped, got %v", err)
	}
}
