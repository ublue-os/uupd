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

> **Note**
> With hardware checks enabled, uupd waits up to 60s for NetworkManager to report
> full connectivity before updating, so a wake-from-suspend update doesn't fail
> before the network is ready. The systemd units order after `network-online.target`
> at boot.


## Run updates from command line (not recommended)

```
sudo uupd
```

# CLI Options

```
uupd --help
```

# Configuration

The default configuration is installed at `/usr/lib/uupd/config.json`. To override it, create `/etc/uupd/config.json`; when present, that file takes precedence over the one in `/usr` and is preserved across upgrades.
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
        "fwupd": {
            "disable": true,
            "binary-path": "/usr/bin/fwupdmgr"
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
- `fwupd.disable`: disable the fwupd firmware notification module (`true` by default)
- `fwupd.binary-path`: path to the `fwupdmgr` binary (default `/usr/bin/fwupdmgr`)
- `system.disable`: disable system update (bootc/rpm-ostree) module

> **Note**
> The `fwupd` module only checks for available firmware updates and notifies you
> about them. uupd will **never** install or upgrade firmware under any
> circumstances — enabling it is harmless and only provides notifications.

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
- `make e2e` / `make e2e-image IMAGE=<name>` run the bootc VM end-to-end suite (see below)
- `make e2e-cli` / `make e2e-extended` run the other suites (see below)
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
| `config.json` | `/usr/lib/uupd/config.json` |

An existing `/usr/lib/uupd/config.json` is never overwritten. Put local overrides in `/etc/uupd/config.json`, which takes precedence over the default config. Use `make install-bin` to install only the binary.

`PREFIX` and `DESTDIR` are supported for staged or custom installs:

```
make install DESTDIR="$PWD/pkgroot"
make install PREFIX=/usr/local
```

> **Note**
> The supplied systemd units call `/usr/bin/uupd`, so a non-default `PREFIX` also requires adjusting the units.

`sudo make uninstall` removes the installed files but preserves the user override in `/etc/uupd`.

## End-to-end tests

The e2e suite builds `uupd` in a builder stage inside the container image,
installs it into a bootc base image, boots that image in a VM with
[bcvk](https://github.com/bootc-dev/bcvk), rebuilds the same tag as an upgraded
image, and checks that `uupd` stages the upgrade and that the VM reboots into
it.

Suites (`make e2e-list-suites`):

- `core` — `update-check` / `is-img-outdated`, `--dry-run` safety, staging
  without `--apply`, the reboot, digest and payload verification, idempotent
  re-runs.
- `cli` — non-root guards, config file/env/flag precedence, `--force`, `--ci`
  and `CI=true`, lockfile contention, JSON and log-file output, module command
  construction and graceful degradation. Needs no image upgrade.
- `extended` — `core` plus `uupd --apply`, `bootc rollback`, and the shipped
  `uupd.service` / `uupd.timer` units.

Supported images (`make e2e-list`):

- `bluefin`, `bazzite`, `aurora` (`ghcr.io/ublue-os/...:stable`)
- `fedora-bootc-rawhide` (`quay.io/fedora/fedora-bootc:rawhide`)

Requirements: Linux with KVM, an unprivileged user with rootless `podman`
and libvirt session access (`LIBVIRT_DEFAULT_URI=qemu:///session` is the
default), `virtiofsd`, OVMF/edk2 and `bcvk` (≥ 0.22 for `--update-from-host`).

```
make e2e-image IMAGE=bluefin                    # core on one image
make e2e                                        # core on every image
make e2e-cli                                    # cli suite on rawhide
make e2e-extended                               # extended suite on rawhide
make e2e-image IMAGE=bluefin SUITE=cli          # any suite on any image
E2E_SUITE=extended ./tests/e2e/e2e.sh bluefin
KEEP_VM=1 ./tests/e2e/e2e.sh bluefin            # leave the VM running
```

CI runs `core` on every PR for bluefin, aurora, bazzite and rawhide, plus
`cli` and `extended` on rawhide. The matrix lives in
`.github/workflows/e2e-ci.yml` and calls the reusable
`.github/workflows/e2e.yml`, which accepts `image`, `suite` and `test_ref`
inputs so other workflows can run the same suite.

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
