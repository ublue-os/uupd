package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func resetConfig() {
	viper.Reset()
	cfg = Config{}
}

func TestInitConfigDefaults(t *testing.T) {
	resetConfig()

	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("HOME")

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default logging level 'info', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.JSON {
		t.Error("Expected default logging JSON to be false")
	}
	if cfg.Logging.Quiet {
		t.Error("Expected default logging quiet to be false")
	}
	if cfg.Logging.File != "-" {
		t.Errorf("Expected default logging file '-', got '%s'", cfg.Logging.File)
	}

	if cfg.Checks.Hardware.Enable {
		t.Error("Expected hardware checks to be disabled by default")
	}
	if cfg.Checks.Hardware.BatteryMinPercent != 20 {
		t.Errorf("Expected default battery min percent 20, got %d", cfg.Checks.Hardware.BatteryMinPercent)
	}
	if cfg.Checks.Hardware.NetworkMaxBytes != 700000 {
		t.Errorf("Expected default network max bytes 700000, got %d", cfg.Checks.Hardware.NetworkMaxBytes)
	}
	if cfg.Checks.Hardware.MemoryMaxPercent != 90 {
		t.Errorf("Expected default memory max percent 90, got %d", cfg.Checks.Hardware.MemoryMaxPercent)
	}
	if cfg.Checks.Hardware.CPUMaxLoadPercent != 50 {
		t.Errorf("Expected default CPU max load percent 50, got %d", cfg.Checks.Hardware.CPUMaxLoadPercent)
	}

	if cfg.Modules.System.Disable {
		t.Error("Expected system module to be enabled by default")
	}
	if cfg.Modules.System.BootcBinary != "/usr/bin/bootc" {
		t.Errorf("Expected default bootc binary '/usr/bin/bootc', got '%s'", cfg.Modules.System.BootcBinary)
	}
	if cfg.Modules.System.RpmOstreeBinary != "/usr/bin/rpm-ostree" {
		t.Errorf("Expected default rpm-ostree binary '/usr/bin/rpm-ostree', got '%s'", cfg.Modules.System.RpmOstreeBinary)
	}
	if cfg.Modules.System.SkopeoBinary != "/usr/bin/skopeo" {
		t.Errorf("Expected default skopeo binary '/usr/bin/skopeo', got '%s'", cfg.Modules.System.SkopeoBinary)
	}

	if cfg.Modules.Flatpak.Disable {
		t.Error("Expected flatpak module to be enabled by default")
	}
	if cfg.Modules.Flatpak.FlatpakBinary != "/usr/bin/flatpak" {
		t.Errorf("Expected default flatpak binary '/usr/bin/flatpak', got '%s'", cfg.Modules.Flatpak.FlatpakBinary)
	}

	if cfg.Modules.Distrobox.Disable {
		t.Error("Expected distrobox module to be enabled by default")
	}
	if cfg.Modules.Distrobox.DistroboxBinary != "/usr/bin/distrobox" {
		t.Errorf("Expected default distrobox binary '/usr/bin/distrobox', got '%s'", cfg.Modules.Distrobox.DistroboxBinary)
	}

	if cfg.Modules.Brew.Disable {
		t.Error("Expected brew module to be enabled by default")
	}

	if cfg.Update.Force {
		t.Error("Expected update force to be false by default")
	}
	if cfg.Update.Verbose {
		t.Error("Expected update verbose to be false by default")
	}
}

func TestGetModules(t *testing.T) {
	resetConfig()

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	modules := GetModules()
	if modules == nil {
		t.Fatal("GetModules() returned nil")
	}

	cfg := Get()
	if modules.System.BootcBinary != cfg.Modules.System.BootcBinary {
		t.Error("GetModules() returned different data than Get().Modules")
	}
}

func TestConfigFileLoading(t *testing.T) {
	resetConfig()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "uupd")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	configContent := `
logging:
  level: debug
  json: true
  quiet: true
  file: /tmp/test.log

checks:
  hardware:
    enable: true
    battery-min-percent: 30
    network-max-bytes: 500000
    memory-max-percent: 80
    cpu-max-load-percent: 60

modules:
  system:
    disable: true
    bootc-binary: /custom/bin/bootc
    rpm-ostree-binary: /custom/bin/rpm-ostree
    skopeo-binary: /custom/bin/skopeo
  flatpak:
    disable: true
    flatpak-binary: /custom/bin/flatpak
  distrobox:
    disable: false
    distrobox-binary: /custom/bin/distrobox
  brew:
    disable: true

update:
  force: true
  verbose: true
`
	configFile := filepath.Join(configDir, "uupd.yml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", originalXDG) }()

	err = InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	cfg := Get()

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected logging level 'debug', got '%s'", cfg.Logging.Level)
	}
	if !cfg.Logging.JSON {
		t.Error("Expected logging JSON to be true")
	}
	if !cfg.Logging.Quiet {
		t.Error("Expected logging quiet to be true")
	}
	if cfg.Logging.File != "/tmp/test.log" {
		t.Errorf("Expected logging file '/tmp/test.log', got '%s'", cfg.Logging.File)
	}

	if !cfg.Checks.Hardware.Enable {
		t.Error("Expected hardware checks to be enabled")
	}
	if cfg.Checks.Hardware.BatteryMinPercent != 30 {
		t.Errorf("Expected battery min percent 30, got %d", cfg.Checks.Hardware.BatteryMinPercent)
	}
	if cfg.Checks.Hardware.NetworkMaxBytes != 500000 {
		t.Errorf("Expected network max bytes 500000, got %d", cfg.Checks.Hardware.NetworkMaxBytes)
	}
	if cfg.Checks.Hardware.MemoryMaxPercent != 80 {
		t.Errorf("Expected memory max percent 80, got %d", cfg.Checks.Hardware.MemoryMaxPercent)
	}
	if cfg.Checks.Hardware.CPUMaxLoadPercent != 60 {
		t.Errorf("Expected CPU max load percent 60, got %d", cfg.Checks.Hardware.CPUMaxLoadPercent)
	}

	if !cfg.Modules.System.Disable {
		t.Error("Expected system module to be disabled")
	}
	if cfg.Modules.System.BootcBinary != "/custom/bin/bootc" {
		t.Errorf("Expected custom bootc binary, got '%s'", cfg.Modules.System.BootcBinary)
	}
	if cfg.Modules.System.RpmOstreeBinary != "/custom/bin/rpm-ostree" {
		t.Errorf("Expected custom rpm-ostree binary, got '%s'", cfg.Modules.System.RpmOstreeBinary)
	}
	if cfg.Modules.System.SkopeoBinary != "/custom/bin/skopeo" {
		t.Errorf("Expected custom skopeo binary, got '%s'", cfg.Modules.System.SkopeoBinary)
	}

	if !cfg.Modules.Flatpak.Disable {
		t.Error("Expected flatpak module to be disabled")
	}
	if cfg.Modules.Flatpak.FlatpakBinary != "/custom/bin/flatpak" {
		t.Errorf("Expected custom flatpak binary, got '%s'", cfg.Modules.Flatpak.FlatpakBinary)
	}

	if cfg.Modules.Distrobox.Disable {
		t.Error("Expected distrobox module to be enabled")
	}
	if cfg.Modules.Distrobox.DistroboxBinary != "/custom/bin/distrobox" {
		t.Errorf("Expected custom distrobox binary, got '%s'", cfg.Modules.Distrobox.DistroboxBinary)
	}

	if !cfg.Modules.Brew.Disable {
		t.Error("Expected brew module to be disabled")
	}

	if !cfg.Update.Force {
		t.Error("Expected update force to be true")
	}
	if !cfg.Update.Verbose {
		t.Error("Expected update verbose to be true")
	}
}

func TestEnvironmentVariableBinding(t *testing.T) {
	resetConfig()

	testEnvVars := map[string]string{
		"UUPD_LOGGING_LEVEL":                          "warn",
		"UUPD_LOGGING_JSON":                           "true",
		"UUPD_CHECKS_HARDWARE_ENABLE":                "true",
		"UUPD_CHECKS_HARDWARE_BATTERY_MIN_PERCENT":    "35",
		"UUPD_CHECKS_HARDWARE_NETWORK_MAX_BYTES":      "800000",
		"UUPD_MODULES_SYSTEM_DISABLE":                 "true",
		"UUPD_MODULES_FLATPAK_DISABLE":                "true",
		"UUPD_MODULES_BREW_DISABLE":                   "true",
		"UUPD_UPDATE_FORCE":                           "true",
		"UUPD_UPDATE_VERBOSE":                         "true",
	}

	for key, value := range testEnvVars {
		_ = os.Setenv(key, value)
		defer func(k string) { _ = os.Unsetenv(k) }(key)
	}

	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("HOME")

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	cfg := Get()

	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected logging level 'warn' from env, got '%s'", cfg.Logging.Level)
	}
	if !cfg.Logging.JSON {
		t.Error("Expected logging JSON true from env")
	}
	if !cfg.Checks.Hardware.Enable {
		t.Error("Expected hardware checks enabled from env")
	}
	if cfg.Checks.Hardware.BatteryMinPercent != 35 {
		t.Errorf("Expected battery min percent 35 from env, got %d", cfg.Checks.Hardware.BatteryMinPercent)
	}
	if cfg.Checks.Hardware.NetworkMaxBytes != 800000 {
		t.Errorf("Expected network max bytes 800000 from env, got %d", cfg.Checks.Hardware.NetworkMaxBytes)
	}
	if !cfg.Modules.System.Disable {
		t.Error("Expected system module disabled from env")
	}
	if !cfg.Modules.Flatpak.Disable {
		t.Error("Expected flatpak module disabled from env")
	}
	if !cfg.Modules.Brew.Disable {
		t.Error("Expected brew module disabled from env")
	}
	if !cfg.Update.Force {
		t.Error("Expected update force true from env")
	}
	if !cfg.Update.Verbose {
		t.Error("Expected update verbose true from env")
	}
}

func TestLegacyEnvironmentVariableBinding(t *testing.T) {
	resetConfig()

	testEnvVars := map[string]string{
		"UUPD_BATTERY_MIN_PERCENT":    "50",
		"UUPD_NETWORK_MAX_BYTES":      "999999",
		"UUPD_MEMORY_MAX_PERCENT":     "95",
		"UUPD_CPU_MAX_LOAD_PERCENT":   "75",
		"UUPD_BOOTC_BINARY":           "/env/bin/bootc",
		"UUPD_RPMOSTREE_BINARY":       "/env/bin/rpm-ostree",
		"UUPD_SKOPEO_BINARY":          "/env/bin/skopeo",
		"UUPD_FLATPAK_BINARY":         "/env/bin/flatpak",
		"UUPD_DISTROBOX_BINARY":       "/env/bin/distrobox",
	}

	for key, value := range testEnvVars {
		_ = os.Setenv(key, value)
		defer func(k string) { _ = os.Unsetenv(k) }(key)
	}

	_ = os.Unsetenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("HOME")

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	cfg := Get()

	if cfg.Checks.Hardware.BatteryMinPercent != 50 {
		t.Errorf("Expected battery min percent 50 from env, got %d", cfg.Checks.Hardware.BatteryMinPercent)
	}
	if cfg.Checks.Hardware.NetworkMaxBytes != 999999 {
		t.Errorf("Expected network max bytes 999999 from env, got %d", cfg.Checks.Hardware.NetworkMaxBytes)
	}
	if cfg.Checks.Hardware.MemoryMaxPercent != 95 {
		t.Errorf("Expected memory max percent 95 from env, got %d", cfg.Checks.Hardware.MemoryMaxPercent)
	}
	if cfg.Checks.Hardware.CPUMaxLoadPercent != 75 {
		t.Errorf("Expected CPU max load percent 75 from env, got %d", cfg.Checks.Hardware.CPUMaxLoadPercent)
	}

	if cfg.Modules.System.BootcBinary != "/env/bin/bootc" {
		t.Errorf("Expected bootc binary from env, got '%s'", cfg.Modules.System.BootcBinary)
	}
	if cfg.Modules.System.RpmOstreeBinary != "/env/bin/rpm-ostree" {
		t.Errorf("Expected rpm-ostree binary from env, got '%s'", cfg.Modules.System.RpmOstreeBinary)
	}
	if cfg.Modules.System.SkopeoBinary != "/env/bin/skopeo" {
		t.Errorf("Expected skopeo binary from env, got '%s'", cfg.Modules.System.SkopeoBinary)
	}
	if cfg.Modules.Flatpak.FlatpakBinary != "/env/bin/flatpak" {
		t.Errorf("Expected flatpak binary from env, got '%s'", cfg.Modules.Flatpak.FlatpakBinary)
	}
	if cfg.Modules.Distrobox.DistroboxBinary != "/env/bin/distrobox" {
		t.Errorf("Expected distrobox binary from env, got '%s'", cfg.Modules.Distrobox.DistroboxBinary)
	}
}

func TestConfigPrecedence(t *testing.T) {
	resetConfig()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "uupd")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	configContent := `
checks:
  hardware:
    battery-min-percent: 40
`
	configFile := filepath.Join(configDir, "uupd.yml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", originalXDG) }()

	_ = os.Setenv("UUPD_BATTERY_MIN_PERCENT", "60")
	defer func() { _ = os.Unsetenv("UUPD_BATTERY_MIN_PERCENT") }()

	err = InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	cfg := Get()

	if cfg.Checks.Hardware.BatteryMinPercent != 60 {
		t.Errorf("Expected env var to override config file, got %d instead of 60", cfg.Checks.Hardware.BatteryMinPercent)
	}
}

func TestInvalidConfigFile(t *testing.T) {
	resetConfig()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "uupd")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}

	invalidYAML := `
this is not valid: yaml: content:
  - broken
    - more broken
`
	configFile := filepath.Join(configDir, "uupd.yml")
	err = os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", originalXDG) }()

	err = InitConfig()
	if err == nil {
		t.Error("Expected InitConfig() to fail with invalid YAML, but it succeeded")
	}
}
