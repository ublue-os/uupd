# u(niversal )upd(ate) 

Small update program written in golang intended for use in Universal Blue, updates flatpak apps, distrobox, brew, bootc and rpm-ostree (as a fallback)

Includes systemd timers and services for auto update

# Installation

This program is now in the [ublue-os/packages COPR](https://copr.fedorainfracloud.org/coprs/ublue-os/packages/)

You can install it on Fedora by running:

> **Note**
> `dnf` can be substituted for `rpm-ostree` or `dnf5`. The [dnf COPR plugin](https://dnf-plugins-core.readthedocs.io/en/latest/copr.html) must also be installed for the `dnf copr` command.

```
sudo dnf copr enable ublue-os/packages
sudo dnf install uupd
```

> **Note**
> If you are on an image derived from uBlue main, you will need to remove or disable automatic updates with rpm-ostreed, to do this, you need to remove or change this line in the config file: `AutomaticUpdatePolicy=stage` (set to `none` if you don't want to remove the line)


# Command Line

To run a complete system update you can use the command line

```
sudo uupd
```

This allows for passwordless system updates (user must be in `wheel` group)


## Run updates from command line (not recommended)

```
sudo uupd
```

# CLI Options

```
uupd --help
```

# Configuration

Automatic updates are ran from the systemd service to edit basic options, you can edit `/etc/uupd/config.json`
```json
{
    "checks": {
        "hardware": {
            "enable": true,
            "bat-min-percent": 20,
            "cpu-max-percent": 50,
            "mem-max-percent": 90,
            "net-max-bytes": 700000
        }
    },
    "modules": {
        "brew": {
            "disable": false
        },
        "distrobox": {
            "disable": false
        },
        "flatpak": {
            "disable": false
        },
        "system": {
            "disable": false
        }
    }
}
```

## Options

### `modules`  

- `brew.disable`: disable brew update module
- `distrobox.disable`: disable distrobox update module
- `flatpak.disable`: disable flatpak update module
- `system.disable`: disable system update (bootc/rpm-ostree) module

### `checks.hardware`

- `enable`: enable hardware checks when running automatic updates (making sure wifi, etc is runnable)
- `bat-min-percent`: minimum battery percentage for checks to pass
- `cpu-min-percent`: maxmium cpu load percentage before checks fail
- `mem-max-percent`: maximum memory usage percentage before checks fail
- `net-max-bytes`: maximum amount of bytes transferred over the network before checks fail


# Troubleshooting

You can check the uupd logs by running this command:
```
journalctl -exu 'uupd.service'
```

# How do I build this?

The `Makefile` is the primary interface for building, testing and installing. Run `make` or `make help` to list all targets.

## Build and test

- `make build` builds the binary into `output/uupd`
- `make test` runs the unit tests (`go test -v -cover ./...`)
- `make test-cover` writes a coverage profile to `output/coverage.out` and prints the coverage summary
- `make vet` runs `go vet ./...`
- `make check` runs both `vet` and `test`
- `make fmt` formats the Go sources, and `make fmt-check` verifies formatting
- `make lint` runs `golangci-lint`
- `make tidy` updates `go.mod`/`go.sum`, and `make vendor` refreshes the vendor directory
- `make clean` removes build artifacts

To build and run an update:

```
make build
sudo ./output/uupd
```

## Installing the tool

`make install` builds and installs the binary, systemd units, polkit rule and default config:

```
sudo make install
sudo systemctl daemon-reload
sudo systemctl enable --now uupd.timer
```

Files are installed under `/usr` by default, matching the paths used by the shipped units:

| File | Destination |
| --- | --- |
| `uupd` | `/usr/bin/uupd` |
| `uupd.service`, `uupd-manual.service`, `uupd.timer` | `/usr/lib/systemd/system/` |
| `uupd.rules` | `/usr/share/polkit-1/rules.d/` |
| `config.json` | `/etc/uupd/config.json` |

An existing `/etc/uupd/config.json` is never overwritten. Use `make install-bin` to install only the binary.

`PREFIX` and `DESTDIR` are supported for staged or custom installs:

```
make install DESTDIR="$PWD/pkgroot"
make install PREFIX=/usr/local
```

> **Note**
> The supplied systemd units call `/usr/bin/uupd`, so a non-default `PREFIX` also requires adjusting the units.

`sudo make uninstall` removes the installed files but preserves the config in `/etc/uupd`.

## Development System Installation

1. `just build-package` will build `uupd` as an rpm and place it into `output/rpms/uupd.rpm`
1. Install the rpm with a package manager of your choice or into a VM for testing

## Devcontainer Usage

1. When prompted, reopen the repository in Container
2. Follow above building instructions
3. Download `uupd` from container to host and run on your host

# FAQ

Q: How do I add my own custom update script?

A: This is meant purely for updating the 'system' components of a Universal Blue image (Distrobox, Flatpak, Bootc, and Brew), anything outside of updating these core components is out of scope
