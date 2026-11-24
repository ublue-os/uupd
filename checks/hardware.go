package checks

import (
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/ublue-os/uupd/pkg/config"
)

type Info struct {
	Name string
	Err  error
}

var ErrMeteredNetwork = errors.New("network is metered")

func Hardware(conn *dbus.Conn) []Info {
	var checks []Info
	conf := config.Get()
	checks = append(checks, battery(busProperty(conn), conf.Checks.Hardware.BatteryMinPercent))
	checks = append(checks, network(conn, conf.Checks.Hardware.NetMaxBytes))
	checks = append(checks, cpu(conf.Checks.Hardware.CpuMaxPercent))
	checks = append(checks, memory(conf.Checks.Hardware.MemMaxPercent))

	return checks
}

type propertyGetter func(dest string, path dbus.ObjectPath, property string) (dbus.Variant, error)

func busProperty(conn *dbus.Conn) propertyGetter {
	return func(dest string, path dbus.ObjectPath, property string) (dbus.Variant, error) {
		return conn.Object(dest, path).GetProperty(property)
	}
}

func serviceMissing(err error) bool {
	var dbusErr dbus.Error
	if !errors.As(err, &dbusErr) {
		return false
	}
	switch dbusErr.Name {
	case "org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.systemd1.UnitMasked":
		return true
	}
	return false
}

func battery(get propertyGetter, min int) Info {
	const name string = "Battery"
	// first, check if the device is running on battery
	variant, err := get("org.freedesktop.UPower", "/org/freedesktop/UPower", "org.freedesktop.UPower.OnBattery")
	if err != nil {
		return Info{
			name,
			err,
		}
	}

	onBattery, ok := variant.Value().(bool)
	if !ok {
		return Info{
			name,
			fmt.Errorf("unable to determine if this computer is running on battery with: %v", variant),
		}
	}
	// Not running on battery, skip this test
	if !onBattery {
		return Info{
			name,
			nil,
		}
	}

	variant, err = get("org.freedesktop.UPower", "/org/freedesktop/UPower/devices/DisplayDevice", "org.freedesktop.UPower.Device.Percentage")
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	batteryPercent, ok := variant.Value().(float64)
	if !ok {
		return Info{
			name,
			fmt.Errorf("unable to get battery percent from: %v", variant),
		}
	}
	if batteryPercent < float64(min) {
		return Info{
			name,
			fmt.Errorf("battery percent below %d, detected battery percent: %v", min, batteryPercent),
		}
	}

	return Info{
		name,
		powerSaver(get),
	}
}

func powerSaver(get propertyGetter) error {
	// check if user is running on low power mode
	variant, err := get("org.freedesktop.UPower.PowerProfiles", "/org/freedesktop/UPower/PowerProfiles", "org.freedesktop.UPower.PowerProfiles.ActiveProfile")
	if serviceMissing(err) {
		return nil
	}
	if err != nil {
		return err
	}
	profile, ok := variant.Value().(string)
	if !ok {
		return fmt.Errorf("unable to get power profile from: %v", variant)
	}
	if profile == "power-saver" {
		return fmt.Errorf("current power profile is set to 'power-saver'")
	}

	return nil
}

func network_once(conn *dbus.Conn, max uint64) error {
	nm := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")

	variant, err := nm.GetProperty("org.freedesktop.NetworkManager.Metered")
	if err != nil {
		return err
	}
	metered, ok := variant.Value().(uint32)
	if !ok {
		return fmt.Errorf("Unable to determine if network connection is metered from: %v", variant)
	}
	// The possible values of "Metered" are documented here:
	// https://networkmanager.dev/docs/api/latest/nm-dbus-types.html//NMMetered
	//
	//     NM_METERED_UNKNOWN   = 0 // The metered status is unknown
	//     NM_METERED_YES       = 1 // Metered, the value was explicitly configured
	//     NM_METERED_NO        = 2 // Not metered, the value was explicitly configured
	//     NM_METERED_GUESS_YES = 3 // Metered, the value was guessed
	//     NM_METERED_GUESS_NO  = 4 // Not metered, the value was guessed
	//
	if metered == 1 || metered == 3 {
		return ErrMeteredNetwork
	}

	// check if user is connected to network
	connectivity, err := nmConnectivity(conn)()
	if err != nil {
		return err
	}

	// 4 means fully connected: https://networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMConnectivityState
	if connectivity != nmConnectivityFull {
		return fmt.Errorf("network not online")
	}

	// sample the network for 5 seconds
	s, err := net.IOCounters(false)
	if err != nil {
		return err
	}
	current := s[0].BytesRecv
	var total uint64 = 0
	for range 5 {
		time.Sleep(time.Second)
		s, err := net.IOCounters(false)
		if err != nil {
			return err
		}
		new := s[0].BytesRecv
		total += new - current
		current = new
	}
	netAvg := total / 5

	if netAvg > max {
		return fmt.Errorf("network is busy, with above %d bytes received (%v)", max, netAvg)
	}
	return nil
}

func network(conn *dbus.Conn, max uint64) Info {
	const name string = "Network"

	// retry if failed (6 retries max)
	max_retry := 6

	var err error
	for range max_retry {
		err = network_once(conn, max)
		if err == nil {
			break
		}
		// sleep for 3 seconds
		time.Sleep(time.Second * 3)
	}

	return Info{
		name,
		err,
	}
}

func memory(max int) Info {
	const name string = "Memory"
	v, err := mem.VirtualMemory()
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	if v.UsedPercent > float64(max) {
		return Info{
			name,
			fmt.Errorf("current memory usage above %d percent: %v", max, v.UsedPercent),
		}
	}
	return Info{
		name,
		nil,
	}
}

func cpu(max int) Info {
	const name string = "CPU"
	avg, err := load.Avg()
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	// Check if the CPU load in the 5 minutes was greater than 50%
	if avg.Load5 > float64(max) {
		return Info{
			name,
			fmt.Errorf("CPU load above %d percent: %v", max, avg.Load5),
		}
	}

	return Info{
		name,
		nil,
	}
}

func RunHwChecks() error {
	// (some hardware checks require dbus access)
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck
	checkInfo := Hardware(conn)
	for _, info := range checkInfo {
		if info.Err != nil {
			if errors.Is(info.Err, ErrMeteredNetwork) {
				return ErrMeteredNetwork
			}
			return fmt.Errorf("%s, returned error: %v", info.Name, info.Err)
		}
	}
	return nil
}
