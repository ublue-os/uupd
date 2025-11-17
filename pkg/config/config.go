package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Logging LoggingConfig `mapstructure:"logging"`
	Checks  ChecksConfig  `mapstructure:"checks"`
	Modules ModulesConfig `mapstructure:"modules"`
	Update  UpdateConfig  `mapstructure:"update"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	JSON  bool   `mapstructure:"json"`
	Quiet bool   `mapstructure:"quiet"`
	File  string `mapstructure:"file"`
}

type ChecksConfig struct {
	Hardware HardwareCheckConfig `mapstructure:"hardware"`
}

type HardwareCheckConfig struct {
	Enable            bool `mapstructure:"enable"`
	BatteryMinPercent int  `mapstructure:"battery-min-percent"`
	NetworkMaxBytes   int  `mapstructure:"network-max-bytes"`
	MemoryMaxPercent  int  `mapstructure:"memory-max-percent"`
	CPUMaxLoadPercent int  `mapstructure:"cpu-max-load-percent"`
}

type ModulesConfig struct {
	System    SystemModuleConfig    `mapstructure:"system"`
	Flatpak   FlatpakModuleConfig   `mapstructure:"flatpak"`
	Distrobox DistroboxModuleConfig `mapstructure:"distrobox"`
	Brew      BrewModuleConfig      `mapstructure:"brew"`
}

type SystemModuleConfig struct {
	Disable         bool   `mapstructure:"disable"`
	BootcBinary     string `mapstructure:"bootc-binary"`
	RpmOstreeBinary string `mapstructure:"rpm-ostree-binary"`
	SkopeoBinary    string `mapstructure:"skopeo-binary"`
}

type FlatpakModuleConfig struct {
	Disable       bool   `mapstructure:"disable"`
	FlatpakBinary string `mapstructure:"flatpak-binary"`
}

type DistroboxModuleConfig struct {
	Disable         bool   `mapstructure:"disable"`
	DistroboxBinary string `mapstructure:"distrobox-binary"`
}

type BrewModuleConfig struct {
	Disable bool `mapstructure:"disable"`
}

type UpdateConfig struct {
	Force   bool `mapstructure:"force"`
	Verbose bool `mapstructure:"verbose"`
}

var cfg Config

// InitConfig initializes Viper configuration
func InitConfig() error {
	viper.SetConfigName("uupd")
	viper.SetConfigType("yml")

	viper.AddConfigPath("/etc/uupd")

	if sudoHome := os.Getenv("SUDO_HOME"); sudoHome != "" {
		viper.AddConfigPath(filepath.Join(sudoHome, ".config", "uupd"))
	} else if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		viper.AddConfigPath(filepath.Join(xdgConfig, "uupd"))
	} else if home := os.Getenv("HOME"); home != "" {
		viper.AddConfigPath(filepath.Join(home, ".config", "uupd"))
	}

	viper.SetEnvPrefix("uupd")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	// Bind legacy environment variable names for backward compatibility
	_ = viper.BindEnv("checks.hardware.battery-min-percent", "UUPD_BATTERY_MIN_PERCENT")
	_ = viper.BindEnv("checks.hardware.network-max-bytes", "UUPD_NETWORK_MAX_BYTES")
	_ = viper.BindEnv("checks.hardware.memory-max-percent", "UUPD_MEMORY_MAX_PERCENT")
	_ = viper.BindEnv("checks.hardware.cpu-max-load-percent", "UUPD_CPU_MAX_LOAD_PERCENT")
	_ = viper.BindEnv("modules.system.bootc-binary", "UUPD_BOOTC_BINARY")
	_ = viper.BindEnv("modules.system.rpm-ostree-binary", "UUPD_RPMOSTREE_BINARY")
	_ = viper.BindEnv("modules.system.skopeo-binary", "UUPD_SKOPEO_BINARY")
	_ = viper.BindEnv("modules.flatpak.flatpak-binary", "UUPD_FLATPAK_BINARY")
	_ = viper.BindEnv("modules.distrobox.distrobox-binary", "UUPD_DISTROBOX_BINARY")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		} else {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	return &cfg
}

// GetModules returns the modules configuration
func GetModules() *ModulesConfig {
	return &cfg.Modules
}

// setDefaults sets default values for all configuration options
func setDefaults() {
	// Logging configuration
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.json", false)
	viper.SetDefault("logging.quiet", false)
	viper.SetDefault("logging.file", "-")

	// Hardware checks
	viper.SetDefault("checks.hardware.enable", false)
	viper.SetDefault("checks.hardware.battery-min-percent", 20)
	viper.SetDefault("checks.hardware.network-max-bytes", 700000)
	viper.SetDefault("checks.hardware.memory-max-percent", 90)
	viper.SetDefault("checks.hardware.cpu-max-load-percent", 50)

	// Update modules configuration
	viper.SetDefault("modules.system.disable", false)
	viper.SetDefault("modules.system.bootc-binary", "/usr/bin/bootc")
	viper.SetDefault("modules.system.rpm-ostree-binary", "/usr/bin/rpm-ostree")
	viper.SetDefault("modules.system.skopeo-binary", "/usr/bin/skopeo")
	viper.SetDefault("modules.flatpak.disable", false)
	viper.SetDefault("modules.flatpak.flatpak-binary", "/usr/bin/flatpak")
	viper.SetDefault("modules.distrobox.disable", false)
	viper.SetDefault("modules.distrobox.distrobox-binary", "/usr/bin/distrobox")
	viper.SetDefault("modules.brew.disable", false)

	// Other options
	viper.SetDefault("update.force", false)
	viper.SetDefault("update.verbose", false)
}
