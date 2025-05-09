package checks

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type Info struct {
	Name string
	Err  error
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
			fmt.Errorf("Unable to determine if this computer is running on battery with: %v", variant),
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
			fmt.Errorf("Unable to get battery percent from: %v", variant),
		}
	}
	if batteryPercent < 20 {
		return Info{
			name,
			fmt.Errorf("Battery percent below 20, detected battery percent: %v", batteryPercent),
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
			fmt.Errorf("Unable to get power profile from: %v", variant),
		}
	}
	if profile == "power-saver" {
		return Info{
			name,
			fmt.Errorf("Current power profile is set to 'power-saver'"),
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
			fmt.Errorf("Network is metered"),
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
			fmt.Errorf("Network not online"),
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
		fmt.Println(new - current)
		current = new
	}
	netAvg := total / 5

	if netAvg > 500000 {
		return Info{
			name,
			fmt.Errorf("Network is busy, with %v bytes recieved", netAvg),
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
	if v.UsedPercent > 90.0 {
		return Info{
			name,
			fmt.Errorf("Current memory usage above 90 percent: %v", v.UsedPercent),
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
	if avg.Load5 > 50.0 {
		return Info{
			name,
			fmt.Errorf("CPU load above 50 percent: %v", avg.Load5),
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
	defer conn.Close()
	checkInfo := Hardware(conn)
	for _, info := range checkInfo {
		if info.Err != nil {
			return fmt.Errorf("%s, returned error: %v", info.Name, info.Err)
		}
	}
	return nil
}
