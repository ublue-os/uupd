package config_test

import (
	"testing"

	"github.com/ublue-os/uupd/pkg/config"
)

func TestEnvVars(t *testing.T) {

	t.Setenv("UUPD_BATTERY_MIN_PERCENT", "100")
	t.Setenv("UUPD_CPU_MAX_LOAD_PERCENT", "100")
	t.Setenv("UUPD_NETWORK_MAX_BYTES", "100")
	t.Setenv("UUPD_MEMORY_MAX_PERCENT", "100")

	err := config.InitConfig("")
	if err != nil {
		t.Fatal("unable to init config")
	}

	if config.Conf.Checks.Hardware.BatteryMinPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s", "UUPD_BATTERY_MIN_PERCENT")
	}
	if config.Conf.Checks.Hardware.CpuMaxPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s", "UUPD_CPU_MAX_LOAD_PERCENT")
	}
	if config.Conf.Checks.Hardware.NetMaxBytes != 100 {
		t.Fatalf("environment variable precedence failed: %s", "UUPD_NETWORK_MAX_BYTES")
	}
	if config.Conf.Checks.Hardware.MemMaxPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s", "UUPD_MEMORY_MAX_PERCENT")
	}
}
