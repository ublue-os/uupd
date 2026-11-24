package checks

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func onBattery(profile any, profileErr error) propertyGetter {
	return func(dest string, path dbus.ObjectPath, property string) (dbus.Variant, error) {
		switch property {
		case "org.freedesktop.UPower.OnBattery":
			return dbus.MakeVariant(true), nil
		case "org.freedesktop.UPower.Device.Percentage":
			return dbus.MakeVariant(float64(80)), nil
		}
		if profileErr != nil {
			return dbus.Variant{}, profileErr
		}
		return dbus.MakeVariant(profile), nil
	}
}

func TestBatteryPassesWithBalancedProfile(t *testing.T) {
	info := battery(onBattery("balanced", nil), 20)
	if info.Err != nil {
		t.Fatalf("expected no error with the balanced profile, got %v", info.Err)
	}
}

func TestBatteryFailsWithPowerSaverProfile(t *testing.T) {
	info := battery(onBattery("power-saver", nil), 20)
	if info.Err == nil {
		t.Fatal("expected an error with the power-saver profile")
	}
}

func TestBatteryPassesWhenPowerProfilesIsMissing(t *testing.T) {
	for _, name := range []string{
		"org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.systemd1.UnitMasked",
	} {
		info := battery(onBattery(nil, dbus.Error{Name: name}), 20)
		if info.Err != nil {
			t.Fatalf("expected no error for %s, got %v", name, info.Err)
		}
	}
}

func TestBatteryFailsOnOtherPowerProfilesErrors(t *testing.T) {
	for _, name := range []string{
		"org.freedesktop.DBus.Error.AccessDenied",
		"org.freedesktop.DBus.Error.TimedOut",
	} {
		info := battery(onBattery(nil, dbus.Error{Name: name}), 20)
		if info.Err == nil {
			t.Fatalf("expected an error for %s", name)
		}
	}
}
