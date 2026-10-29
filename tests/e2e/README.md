# uupd end-to-end tests

These tests install uupd into a real image-based system, boot it in a VM, and
verify that `uupd` can stage an update, reboot into it, and end up on the new
image.

They replaced the earlier `tmt` based setup. The new flow has two properties
that matter for the images we test:

- **Distroless installation.** uupd is a static Go binary, so the test image is
  built by copying `output/uupd` and the systemd/polkit files into the base
  image. No `dnf`, `rpm`, or Go toolchain is needed inside the image. This is
  what makes it possible to test images like Dakota, which is built from source
  with BuildStream and has no package manager.
- **bcvk instead of tmt.** [`bcvk`](https://github.com/bootc-dev/bcvk) installs
  the image to a persistent VM and gives the guest read-only access to the
  host's container storage (`--update-from-host`). The upgrade image is built
  on the host, so nothing has to be built inside the guest, and the guest needs
  neither podman nor a registry.

## How a run works

1. `podman build` the base test image:
   `tests/e2e/Containerfile.e2e` with the image under test as `BASE_IMAGE`.
2. `bcvk libvirt run` installs it to a disk-backed VM and boots it. Host
   container storage is shared read-only with the guest.
3. The same Containerfile is built again with `MARKER=1` and retagged. This
   image contains `/usr/share/uupd-testing`, which is the proof that an upgrade
   happened.
4. `test-uupd-upgrade.sh phase1` runs in the guest: it checks the read-only
   subcommands (`--is-img-outdated`, `--update-check`, `--dry-run`) and runs
   `uupd`, then asserts that the new image was staged.
5. The guest reboots.
6. `test-uupd-upgrade.sh phase2` verifies that the marker file exists and that
   the booted image digest matches the image that uupd staged.

The image list lives in `run-bcvk-e2e.sh` (`--list`), together with the base
image and root filesystem used for each one.

## Running locally

Requirements: root, KVM, podman, libvirt, `virtiofsd`, OVMF (UEFI firmware),
and `bcvk` 0.22 or newer.

```bash
make build                                   # build output/uupd
sudo ./tests/e2e/run-bcvk-e2e.sh --list
sudo ./tests/e2e/run-bcvk-e2e.sh bluefin
sudo ./tests/e2e/run-bcvk-e2e.sh dakota
```

Useful environment variables:

- `FILESYSTEM` – root filesystem for the installed VM (default is `btrfs`, or
  `xfs` for Dakota).
- `DISK_SIZE` – VM disk size (default `20G`).
- `MEMORY` – VM memory (default `4G`).
- `KEEP_VM=1` – keep the VM after the run for debugging.

Or through `just`:

```bash
just e2e-list
just e2e-image name=aurora
```

## CI

`.github/workflows/e2e.yml` runs the same scripted flow for every image in the
matrix on `ubuntu-26.04`, installing bcvk and the virtualization stack first.
