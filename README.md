# u(niversal )upd(ate) 

Small update program written in golang intended for use in Universal Blue, updates flatpak apps, distrobox, brew, bootc and rpm-ostree (as a fallback)

Includes systemd timers and services for auto update

# Installation

There will eventually be a COPR with the RPM
In the meantime it's possible to build the binary statically and copy the files (binary, polkit rules, and systemd units) refer to [How do I build this](#How-do-I-build-this?) for more info

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

```

```

# Troubleshooting

You can check the ublue-update logs by running this command:
```
$ journalctl -exu 'uupd.service'
```

# How do I build this?

1. `just build` will build this project and place the binary in `output/ublue-upd`
1. `sudo ./output/ublue-upd` will run an update

# FAQ

Q: How do I add my own custom update script?

A: This is meant purely for updating the 'system' components of a Universal Blue image (Distrobox, Flatpak, Bootc, and Brew), anything outside of updating these core components is out of scope
