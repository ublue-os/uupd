package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Modules struct {
		Flatpak struct {
			Disable    bool   `mapstructure:"disable"`
			BinaryPath string `mapstructure:"binary-path"`
		} `mapstructure:"flatpak"`

		Brew struct {
			Disable bool `mapstructure:"disable"`
		} `mapstructure:"brew"`

		System struct {
			Disable         bool   `mapstructure:"disable"`
			RpmOstreeBinary string `mapstructure:"rpm-ostree-binary"`
			BootcBinary     string `mapstructure:"bootc-binary"`
			SkopeoBinary    string `mapstructure:"skopeo-binary"`
		} `mapstructure:"system"`

		Distrobox struct {
			Disable    bool   `mapstructure:"disable"`
			BinaryPath string `mapstructure:"binary-path"`
		} `mapstructure:"distrobox"`
	} `mapstructure:"modules"`

	Logging struct {
		Level string `mapstructure:"level"`
		File  string `mapstructure:"file"`
		Quiet bool   `mapstructure:"quiet"`
		JSON  bool   `mapstructure:"json"`
	} `mapstructure:"logging"`

	Checks struct {
		Hardware struct {
			Enable            bool `mapstructure:"enable"`
			BatteryMinPercent int  `mapstructure:"bat-min-percent"`
			NetMaxBytes       int  `mapstructure:"net-max-bytes"`
			MemMaxPercent     int  `mapstructure:"mem-max-percent"`
			CpuMaxPercent     int  `mapstructure:"cpu-max-percent"`
		} `mapstructure:"hardware"`
	} `mapstructure:"checks"`
}

const DEFAULT_PATH string = "/etc/uupd/config.json"
const AUTOUPDATE_DEFAULT_PATH string = "/etc/uupd/auto-config.json"

var Conf Config

func defaults() {
	d := viper.SetDefault
	e := viper.BindEnv

	d("log.level", "info")
	d("log.file", "-")
	d("log.json", false)
	d("log.quiet", false)

	// modules
	d("modules.flatpak.disable", true)
	d("modules.flatpak.binary-path", true)

	d("modules.brew.disable", true)

	d("modules.system.disable", true)
	d("modules.system.rpm-ostree-binary", "/usr/bin/rpm-ostree")
	d("modules.system.bootc-binary", "/usr/bin/bootc")
	d("modules.system.skopeo-binary", "/usr/bin/skopeo")

	d("modules.distrobox.disable", true)
	d("modules.distrobox.binary-path", true)

	// checks
	d("checks.hardware.enable", true)
	d("checks.hardware.bat-min-percent", 20)
	d("checks.hardware.net-max-bytes", 700000)
	d("checks.hardware.mem-max-percent", 90)
	d("checks.hardware.cpu-max-percent", 50)

	_ = e("checks.hardware.bat-min-percent", "UUPD_BATTERY_MIN_PERCENT")
	_ = e("checks.hardware.net-max-bytes", "UUPD_NETWORK_MAX_BYTES")
	_ = e("checks.hardware.mem-max-percent", "UUPD_MEMORY_MAX_PERCENT")
	_ = e("checks.hardware.cpu-max-load-percent", "UUPD_CPU_MAX_LOAD_PERCENT")
	_ = e("modules.system.bootc-binary", "UUPD_BOOTC_BINARY")
	_ = e("modules.system.rpm-ostree-binary", "UUPD_RPMOSTREE_BINARY")
	_ = e("modules.system.skopeo-binary", "UUPD_SKOPEO_BINARY")
	_ = e("modules.flatpak.binary-path", "UUPD_FLATPAK_BINARY")
	_ = e("modules.distrobox.binary-path", "UUPD_DISTROBOX_BINARY")
}

func InitConfig(p string) error {

	viper.SetConfigFile(p)
	viper.SetConfigType("json")
	defaults()

	if err := viper.Unmarshal(&Conf); err != nil {
		return fmt.Errorf("Failed to unmarshal config: %v", err)
	}

	_ = viper.ReadInConfig()

	return nil
}
