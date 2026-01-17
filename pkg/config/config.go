package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Modules struct {
		Flatpak struct {
			Disable    bool   `mapstructure:"disable"`
			BinaryPath string `mapstructure:"binary-path"`
		} `mapstructure:"flatpak"`

		Brew struct {
			Disable    bool   `mapstructure:"disable"`
			Prefix     string `mapstructure:"prefix"`
			Repository string `mapstructure:"repository"`
			Cellar     string `mapstructure:"cellar"`
			Path       string `mapstructure:"path"`
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
			Enable            bool   `mapstructure:"enable"`
			BatteryMinPercent int    `mapstructure:"bat-min-percent"`
			NetMaxBytes       uint64 `mapstructure:"net-max-bytes"`
			MemMaxPercent     int    `mapstructure:"mem-max-percent"`
			CpuMaxPercent     int    `mapstructure:"cpu-max-percent"`
		} `mapstructure:"hardware"`
	} `mapstructure:"checks"`
}

const DEFAULT_PATH string = "/etc/uupd/config.json"

var conf Config

func defaults() {
	d := viper.SetDefault
	e := viper.BindEnv

	d("logging.level", "debug")
	d("logging.file", "-")
	d("logging.json", false)
	d("logging.quiet", false)

	// modules
	d("modules.flatpak.disable", false)
	d("modules.flatpak.binary-path", "/usr/bin/flatpak")

	d("modules.brew.disable", false)

	d("modules.system.disable", false)
	d("modules.system.rpm-ostree-binary", "/usr/bin/rpm-ostree")
	d("modules.system.bootc-binary", "/usr/bin/bootc")
	d("modules.system.skopeo-binary", "/usr/bin/skopeo")

	d("modules.distrobox.disable", false)
	d("modules.distrobox.binary-path", "/usr/bin/distrobox")

	// checks
	d("checks.hardware.enable", true)
	d("checks.hardware.bat-min-percent", 20)
	d("checks.hardware.net-max-bytes", 700000)
	d("checks.hardware.mem-max-percent", 90)
	d("checks.hardware.cpu-max-percent", 50)

	_ = e("checks.hardware.bat-min-percent", "UUPD_BATTERY_MIN_PERCENT")
	_ = e("checks.hardware.net-max-bytes", "UUPD_NETWORK_MAX_BYTES")
	_ = e("checks.hardware.mem-max-percent", "UUPD_MEMORY_MAX_PERCENT")
	_ = e("checks.hardware.cpu-max-percent", "UUPD_CPU_MAX_LOAD_PERCENT")

	_ = e("modules.system.bootc-binary", "UUPD_BOOTC_BINARY")
	_ = e("modules.system.rpm-ostree-binary", "UUPD_RPMOSTREE_BINARY")
	_ = e("modules.system.skopeo-binary", "UUPD_SKOPEO_BINARY")
	_ = e("modules.flatpak.binary-path", "UUPD_FLATPAK_BINARY")
	_ = e("modules.distrobox.binary-path", "UUPD_DISTROBOX_BINARY")

	var (
		HomebrewDefaultPrefix string = "/home/linuxbrew/.linuxbrew"

		HomebrewDefaultRepository string = fmt.Sprintf("%s/Homebrew", HomebrewDefaultPrefix)
		HomebrewDefaultCellar     string = fmt.Sprintf("%s/Cellar", HomebrewDefaultPrefix)
		HomebrewDefaultPath       string = fmt.Sprintf("%s/bin/brew", HomebrewDefaultPrefix)
	)

	d("modules.brew.prefix", HomebrewDefaultPrefix)
	d("modules.brew.repository", HomebrewDefaultRepository)
	d("modules.brew.cellar", HomebrewDefaultCellar)
	d("modules.brew.path", HomebrewDefaultPath)

	_ = e("modules.brew.prefix", "HOMEBREW_PREFIX")
	_ = e("modules.brew.repository", "HOMEBREW_REPOSITORY")
	_ = e("modules.brew.cellar", "HOMEBREW_CELLAR")
	_ = e("modules.brew.path", "HOMEBREW_PATH")
}

func InitConfig(p string) error {

	viper.SetConfigFile(p)
	defaults()

	if _, err := os.Stat(p); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}
	} else if p != DEFAULT_PATH {
		return fmt.Errorf("Bad config file path: %s", p)
	}

	if err := viper.UnmarshalExact(&conf); err != nil {
		return fmt.Errorf("Failed to unmarshal config: %v", err)
	}

	return nil
}

func Get() *Config {
	return &conf
}
