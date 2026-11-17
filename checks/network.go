package checks

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	// nmConnectivityFull is NMConnectivityStateFull: NetworkManager has
	// verified that the machine has full internet connectivity.
	// https://networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMConnectivityState
	nmConnectivityFull uint32 = 4

	// NetworkWaitTimeout is the maximum time to wait for the network to become
	// ready before running updates. Waking from suspend (or reconnecting a
	// cable) can leave the network stack and DNS unavailable for a while.
	// Without waiting, an update starts too early, fails, and notifies the
	// user even though the systemd retry a minute later would succeed.
	NetworkWaitTimeout = 60 * time.Second

	// NetworkWaitInterval is how often connectivity is polled while waiting.
	NetworkWaitInterval = 2 * time.Second
)

// connectivityCheck returns NetworkManager's current connectivity state.
type connectivityCheck func() (uint32, error)

// nmConnectivity builds a connectivityCheck backed by the system bus.
func nmConnectivity(conn *dbus.Conn) connectivityCheck {
	return func() (uint32, error) {
		nm := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
		var connectivity uint32
		if err := nm.Call("org.freedesktop.NetworkManager.CheckConnectivity", 0).Store(&connectivity); err != nil {
			return 0, err
		}
		return connectivity, nil
	}
}

// waitForNetwork polls check until it reports full connectivity or until
// timeout elapses. It is a no-op when the network is already online.
func waitForNetwork(timeout, interval time.Duration, check connectivityCheck) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := check()
		if err == nil && state == nmConnectivityFull {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("network not online: %w", err)
			}
			return fmt.Errorf("network not online (connectivity state %d)", state)
		}
		time.Sleep(interval)
	}
}

// WaitForNetwork waits until NetworkManager reports full internet
// connectivity or until NetworkWaitTimeout elapses. It returns an error if the
// bus is unavailable or the network didn't come up in time.
func WaitForNetwork() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck
	return waitForNetwork(NetworkWaitTimeout, NetworkWaitInterval, nmConnectivity(conn))
}
