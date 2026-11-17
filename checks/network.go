package checks

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	// NMConnectivityStateFull: https://networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMConnectivityState
	nmConnectivityFull uint32 = 4

	// Max time to wait for the network (DNS may lag on wake from suspend).
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

// waitForNetwork polls until the network is online or timeout elapses.
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

// WaitForNetwork waits for full connectivity or until NetworkWaitTimeout elapses.
func WaitForNetwork() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck
	return waitForNetwork(NetworkWaitTimeout, NetworkWaitInterval, nmConnectivity(conn))
}
