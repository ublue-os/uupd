#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

TEST_IMAGE="localhost/uupd-e2e:latest"
CONTAINERFILE="tests/e2e/Containerfile.e2e"
GUEST_SCRIPT="tests/e2e/guest-verify.sh"

log() { printf '[e2e] %s\n' "$*" >&2; }
die() {
	log "ERROR: $*"
	exit 1
}

IMAGES=(bluefin bazzite aurora fedora-bootc-rawhide dakota)

image_ref() {
	case "$1" in
		bluefin) printf '%s\n' ghcr.io/ublue-os/bluefin:stable ;;
		bazzite) printf '%s\n' ghcr.io/ublue-os/bazzite:stable ;;
		aurora) printf '%s\n' ghcr.io/ublue-os/aurora:stable ;;
		fedora-bootc-rawhide) printf '%s\n' quay.io/fedora/fedora-bootc:rawhide ;;
		dakota) printf '%s\n' ghcr.io/projectbluefin/dakota:stable ;;
		*) return 1 ;;
	esac
}

image_filesystem() {
	case "$1" in
		bluefin | bazzite | aurora) printf '%s\n' btrfs ;;
		dakota) printf '%s\n' xfs ;;
		fedora-bootc-rawhide) printf '%s\n' ext4 ;;
		*) return 1 ;;
	esac
}

image_disk_size() {
	case "$1" in
		bazzite) printf '%s\n' 40G ;;
		*) printf '%s\n' 30G ;;
	esac
}

image_memory() {
	case "$1" in
		bazzite) printf '%s\n' 6G ;;
		*) printf '%s\n' 4G ;;
	esac
}

usage() {
	echo "Usage: sudo $0 <image|--all|--list> [base-image-ref]" >&2
	echo "Available images:" >&2
	printf '  %s\n' "${IMAGES[@]}" >&2
}

find_virtiofsd() {
	if [[ -n "${VIRTIOFSD_BIN:-}" ]]; then
		return 0
	fi
	local candidate
	for candidate in "$(command -v virtiofsd || true)" \
		/usr/lib/qemu/virtiofsd /usr/libexec/virtiofsd /usr/lib/virtiofsd; do
		if [[ -n "$candidate" && -x "$candidate" ]]; then
			export VIRTIOFSD_BIN="$candidate"
			return 0
		fi
	done
	return 1
}

build_image() {
	local target="$1"
	podman build \
		--build-arg BASE_IMAGE="$BASE_IMAGE" \
		--target "$target" \
		-t "$TEST_IMAGE" \
		-f "$CONTAINERFILE" \
		.
}

in_vm() {
	bcvk libvirt ssh "$VM_NAME" "$1"
}

run_phase() {
	local phase="$1" script
	script="$(cat "$GUEST_SCRIPT")"
	log "running $phase in the VM"
	bcvk libvirt ssh "$VM_NAME" \
		"TEST_PHASE=$phase bash -c $(printf '%q' "$script")"
}

dump_guest_logs() {
	log "collecting logs from $VM_NAME"
	in_vm 'echo "--- bootc status ---"; bootc status || true;
	       echo "--- uupd log ---"; cat /var/log/uupd-e2e.log 2>/dev/null || true;
	       echo "--- uupd journal ---"; journalctl -b --no-pager -u uupd.service 2>/dev/null || true' ||
		log "could not reach the VM to collect logs"
}

wait_for_reboot() {
	local old_boot_id="$1" boot_id
	for _ in $(seq 1 60); do
		sleep 10
		boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id' 2>/dev/null || true)"
		if [[ -n "$boot_id" && "$boot_id" != "$old_boot_id" ]]; then
			log "VM rebooted (boot id $boot_id)"
			return 0
		fi
	done
	return 1
}

run_one() {
	local image_name="$1"
	BASE_IMAGE="${BASE_IMAGE:-${2:-}}"
	if [[ -z "$BASE_IMAGE" ]]; then
		BASE_IMAGE="$(image_ref "$image_name" || true)"
	fi
	[[ -n "$BASE_IMAGE" ]] || {
		usage
		die "unknown image name '$image_name'"
	}

	local filesystem disk_size memory
	filesystem="${FILESYSTEM:-$(image_filesystem "$image_name" || true)}"
	[[ -n "$filesystem" ]] || die "no filesystem mapping for '$image_name'"
	disk_size="${DISK_SIZE:-$(image_disk_size "$image_name")}"
	memory="${MEMORY:-$(image_memory "$image_name")}"
	VM_NAME="uupd-e2e-${image_name//[^A-Za-z0-9]/-}"

	[[ -f "$CONTAINERFILE" ]] || die "missing $CONTAINERFILE"
	[[ -f "$GUEST_SCRIPT" ]] || die "missing $GUEST_SCRIPT"

	cleanup() {
		local rc=$?
		trap - EXIT
		if ((rc != 0)); then
			dump_guest_logs || true
		fi
		if [[ "${KEEP_VM:-0}" != "1" ]]; then
			bcvk libvirt rm -f "$VM_NAME" >/dev/null 2>&1 || true
		else
			log "KEEP_VM=1, leaving $VM_NAME running"
		fi
		exit "$rc"
	}
	trap cleanup EXIT

	log "testing $image_name ($BASE_IMAGE), root filesystem $filesystem"

	log "pulling $BASE_IMAGE"
	podman pull --retry 5 "$BASE_IMAGE"

	log "building the base test image"
	build_image base

	log "installing $TEST_IMAGE to a VM with bcvk"
	bcvk libvirt run \
		--name "$VM_NAME" \
		--replace \
		--update-from-host \
		--filesystem "$filesystem" \
		--disk-size "$disk_size" \
		--memory "$memory" \
		--firmware uefi-insecure \
		--detach \
		--ssh-wait \
		"$TEST_IMAGE"

	log "building the upgrade image and retagging $TEST_IMAGE"
	build_image upgraded
	local upgrade_digest
	upgrade_digest="$(podman image inspect --format '{{.Digest}}' "$TEST_IMAGE" || true)"
	log "upgrade image digest: ${upgrade_digest:-<unknown>}"

	local old_boot_id
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "boot id before the upgrade: $old_boot_id"

	run_phase phase1

	log "rebooting the VM"
	in_vm 'systemctl reboot' || true

	log "waiting for the VM to come back"
	wait_for_reboot "$old_boot_id" || die "the VM did not reboot after the upgrade"

	run_phase phase2

	log "PASS: $image_name upgraded with uupd"
}

if [[ "${1:-}" == "--list" || "${1:-}" == "-l" ]]; then
	printf '%s\n' "${IMAGES[@]}"
	exit 0
fi

if [[ -z "${1:-}" ]]; then
	usage
	exit 1
fi

[[ $EUID -eq 0 ]] || die "this test needs root (podman, libvirt, KVM); run it with sudo"

command -v podman >/dev/null || die "podman is required"
command -v bcvk >/dev/null || die "bcvk is required (see https://github.com/bootc-dev/bcvk)"
find_virtiofsd || log "WARNING: virtiofsd not found, bcvk may fail to share host storage"

if [[ "$1" == "--all" ]]; then
	overall=0
	for image in "${IMAGES[@]}"; do
		env -u BASE_IMAGE -u FILESYSTEM -u DISK_SIZE -u MEMORY \
			"$0" "$image" || overall=1
	done
	exit "$overall"
fi

run_one "$@"
