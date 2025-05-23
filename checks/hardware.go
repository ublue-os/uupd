package checks

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

const (
	envBatMinPercent     string = "UUPD_BATTERY_MIN_PERCENT"
	envNetMaxBytes       string = "UUPD_NETWORK_MAX_BYTES"
	envMemMaxPercent     string = "UUPD_MEMORY_MAX_PERCENT"
	envCpuMaxLoadPercent string = "UUPD_CPU_MAX_LOAD_PERCENT"

	batMinPercent     int = 20
	netMaxBytes       int = 500000
	memMaxPercent     int = 90
	cpuMaxLoadPercent int = 50
)

type Info struct {
	Name string
	Err  error
}

func envOrFallbackInt(key string, fallback int) int {
	valStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}

	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return fallback
	}

	return valInt
}

func Hardware(conn *dbus.Conn) []Info {
	var checks []Info
	checks = append(checks, battery(conn))
	checks = append(checks, network(conn))
	checks = append(checks, cpu())
	checks = append(checks, memory())

	return checks
}

func battery(conn *dbus.Conn) Info {
	const name string = "Battery"
	upower := conn.Object("org.freedesktop.UPower", "/org/freedesktop/UPower")
	// first, check if the device is running on battery
	variant, err := upower.GetProperty("org.freedesktop.UPower.OnBattery")
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

	dev := conn.Object("org.freedesktop.UPower", "/org/freedesktop/UPower/devices/DisplayDevice")
	variant, err = dev.GetProperty("org.freedesktop.UPower.Device.Percentage")
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
	min := envOrFallbackInt(envBatMinPercent, batMinPercent)
	if batteryPercent < float64(min) {
		return Info{
			name,
			fmt.Errorf("battery percent below %d, detected battery percent: %v", min, batteryPercent),
		}
	}

	// check if user is running on low power mode
	powerProfiles := conn.Object("org.freedesktop.UPower.PowerProfiles", "/org/freedesktop/UPower/PowerProfiles")
	variant, err = powerProfiles.GetProperty("org.freedesktop.UPower.PowerProfiles.ActiveProfile")
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	profile, ok := variant.Value().(string)
	if !ok {
		return Info{
			name,
			fmt.Errorf("unable to get power profile from: %v", variant),
		}
	}
	if profile == "power-saver" {
		return Info{
			name,
			fmt.Errorf("current power profile is set to 'power-saver'"),
		}
	}

	return Info{
		name,
		nil,
	}
}

func network(conn *dbus.Conn) Info {
	const name string = "Network"

	nm := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")

	variant, err := nm.GetProperty("org.freedesktop.NetworkManager.Metered")
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	metered, ok := variant.Value().(uint32)
	if !ok {
		return Info{
			name,
			fmt.Errorf("Unable to determine if network connection is metered from: %v", variant),
		}
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
		return Info{
			name,
			fmt.Errorf("network is metered"),
		}
	}

	// check if user is connected to network
	var connectivity uint32
	err = nm.Call("org.freedesktop.NetworkManager.CheckConnectivity", 0).Store(&connectivity)
	if err != nil {
		return Info{
			name,
			err,
		}
	}

	// 4 means fully connected: https://networkmanager.dev/docs/api/latest/nm-dbus-types.html#NMConnectivityState
	if connectivity != 4 {
		return Info{
			name,
			fmt.Errorf("network not online"),
		}
	}

	// sample the network for 5 seconds
	s, err := net.IOCounters(false)
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	current := s[0].BytesRecv
	var total uint64 = 0
	for range 5 {
		time.Sleep(time.Second)
		s, err := net.IOCounters(false)
		if err != nil {
			return Info{
				name,
				err,
			}
		}
		new := s[0].BytesRecv
		total += new - current
		current = new
	}
	netAvg := total / 5

	max := envOrFallbackInt(envNetMaxBytes, netMaxBytes)
	if netAvg > uint64(max) {
		return Info{
			name,
			fmt.Errorf("network is busy, with above %d bytes recieved (%v)", max, netAvg),
		}
	}

	return Info{
		name,
		nil,
	}

}

func memory() Info {
	const name string = "Memory"
	v, err := mem.VirtualMemory()
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	max := envOrFallbackInt(envMemMaxPercent, memMaxPercent)
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

func cpu() Info {
	const name string = "CPU"
	avg, err := load.Avg()
	if err != nil {
		return Info{
			name,
			err,
		}
	}
	// Check if the CPU load in the 5 minutes was greater than 50%
	max := envOrFallbackInt(envCpuMaxLoadPercent, cpuMaxLoadPercent)
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
			return fmt.Errorf("%s, returned error: %v", info.Name, info.Err)
		}
	}
	return nil
}
