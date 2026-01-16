package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ublue-os/uupd/pkg/config"
)

func TestEnvVars(t *testing.T) {

	t.Setenv("UUPD_BATTERY_MIN_PERCENT", "100")
	t.Setenv("UUPD_CPU_MAX_LOAD_PERCENT", "100")
	t.Setenv("UUPD_NETWORK_MAX_BYTES", "100")
	t.Setenv("UUPD_MEMORY_MAX_PERCENT", "100")

	err := config.InitConfig(config.DEFAULT_PATH)
	if err != nil {
		t.Fatalf("unable to init config: %v", err)
	}

	if config.Conf.Checks.Hardware.BatteryMinPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s, current: %d", "UUPD_BATTERY_MIN_PERCENT", config.Conf.Checks.Hardware.BatteryMinPercent)
	}
	if config.Conf.Checks.Hardware.CpuMaxPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s, current: %d", "UUPD_CPU_MAX_LOAD_PERCENT", config.Conf.Checks.Hardware.CpuMaxPercent)
	}
	if config.Conf.Checks.Hardware.NetMaxBytes != 100 {
		t.Fatalf("environment variable precedence failed: %s, current: %d", "UUPD_NETWORK_MAX_BYTES", config.Conf.Checks.Hardware.NetMaxBytes)
	}
	if config.Conf.Checks.Hardware.MemMaxPercent != 100 {
		t.Fatalf("environment variable precedence failed: %s, current: %d", "UUPD_MEMORY_MAX_PERCENT", config.Conf.Checks.Hardware.MemMaxPercent)
	}
}

func TestConfigLocation(t *testing.T) {
	newConfig := `{
			"modules": {
				"flatpak": {
					"disable": true
				}
			}
		}
	`

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	os.WriteFile(path, []byte(newConfig), 0644)

	if err := config.InitConfig(path); err != nil {
		t.Fatalf("unable to init config: %v", err)
	}

	if config.Conf.Modules.Flatpak.Disable != true {
		t.Fatal("Unable to override config path!")
	}

	// defaults?
	if config.Conf.Checks.Hardware.BatteryMinPercent != 20 {
		t.Fatalf("BatteryMinPercent is not 20: %d", config.Conf.Checks.Hardware.BatteryMinPercent)
	}
}

func TestConfigInvalidConfig(t *testing.T) {
	newConfig := `{
			"modules" {
				"flatpak": {
					disable: true
				}
			}
			asdkfadshf
		}
	`

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	os.WriteFile(path, []byte(newConfig), 0644)

	if err := config.InitConfig(path); err == nil {
		t.Fatalf("bad config went through")
	}

}
func TestConfigInvalidConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "bogus.json")

	if err := config.InitConfig(path); err == nil {
		t.Fatalf("bad config file path went through")
	}
}
