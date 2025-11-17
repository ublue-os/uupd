# u(niversal )upd(ate) 

Small update program written in golang intended for use in Universal Blue, updates flatpak apps, distrobox, brew, bootc and rpm-ostree (as a fallback)

Includes systemd timers and services for auto update

# Installation

This program is now in the [ublue-os/packages COPR](https://copr.fedorainfracloud.org/coprs/ublue-os/packages/)

You can install it on Fedora by running:

> **Note**
> `dnf` can be substituted for `rpm-ostree` or `dnf5`. The [dnf COPR plugin](https://dnf-plugins-core.readthedocs.io/en/latest/copr.html) must also be installed for the `dnf copr` command.

```
$ sudo dnf copr enable ublue-os/packages
$ sudo dnf install uupd
```

> **Note**
> If you are on an image derived from uBlue main, you will need to remove or disable automatic updates with rpm-ostreed, to do this, you need to remove or change this line in the config file: `AutomaticUpdatePolicy=stage` (set to `none` if you don't want to remove the line)


# Command Line

To run a complete system update, it's recommended to use systemd:

```
$ systemctl start uupd.service
```

This allows for passwordless system updates (user must be in `wheel` group)


## Run updates from command line (not recommended)

```
$ sudo uupd
```

# CLI Options

```
$ uupd --help
```

# Configuration

uupd can be configured using configuration files or environment variables.

## Configuration Files

Configuration files are loaded in the following order (highest precedence last):

1. `/etc/uupd/uupd.yml` - System-wide configuration
2. `~/.config/uupd/uupd.yml` - User-specific configuration

See `etc/uupd/uupd.yml` in the repository for a complete configuration example.

## Environment Variables

Environment variables can be used with the `UUPD_` prefix. Nested configuration keys use underscores as separators:

```bash
UUPD_LOGGING_LEVEL=debug
UUPD_CHECKS_HARDWARE_ENABLE=true
UUPD_CHECKS_HARDWARE_BATTERY_MIN_PERCENT=30
UUPD_MODULES_SYSTEM_DISABLE=true
UUPD_UPDATE_FORCE=true
```

## Configuration Precedence

Configuration is loaded with the following precedence (highest to lowest):

1. CLI flags
2. Environment variables (UUPD_*)
3. User configuration file (~/.config/uupd/uupd.yml)
4. System configuration file (/etc/uupd/uupd.yml)
5. Default values

# Troubleshooting

You can check the uupd logs by running this command:
```
$ journalctl -exu 'uupd.service'
```

# How do I build this?

1. `just build` will build this project and place the binary in `output/uupd`
1. `sudo ./output/uupd` will run an update
1. You can install this to the system by copying the rules

##  Devcontainer Usage
  1. When prompted, reopen the repository in Container
  2. Follow above building instructions
  3. Download `uupd` from container to host and run on your host

# FAQ

Q: How do I add my own custom update script?

A: This is meant purely for updating the 'system' components of a Universal Blue image (Distrobox, Flatpak, Bootc, and Brew), anything outside of updating these core components is out of scope
