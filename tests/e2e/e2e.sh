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

IMAGES=(bluefin bazzite aurora quay.io/fedora/fedora-bootc:rawhide)

SCENARIO="${E2E_SCENARIO:-core}"
case "$SCENARIO" in
	core | extended) ;;
	*) die "unknown E2E_SCENARIO '$SCENARIO' (expected core or extended)" ;;
esac

retry() {
	if [[ "${#}" -lt 3 ]]; then
		echo "retry usage: <number of tries> <time between retries> <command> ..."
		return 1
	fi
	local tries="${1}" delay="${2}" r=0 i
	shift 2
	for ((i = 1; i <= tries; i++)); do
		if [[ ${i} -gt 1 ]]; then
			sleep "${delay}"
		fi
		if "${@}"; then
			r=0
			break
		else
			r=$?
		fi
	done
	return "$r"
}

image_ref() {
	local stream
	case "$1" in
		bluefin) stream=stable ;;
		bluefin:*) stream="${1#*:}" ;;
		bazzite) stream=stable ;;
		bazzite:*) stream="${1#*:}" ;;
		aurora) stream=stable ;;
		aurora:*) stream="${1#*:}" ;;
		fedora-bootc-rawhide) stream=rawhide ;;
		*/*:*) printf '%s\n' "$1" && return 0 ;;
		*) return 1 ;;
	esac
	case "$1" in
		fedora-bootc-rawhide) printf '%s\n' "quay.io/fedora/fedora-bootc:${stream}" ;;
		*) printf '%s\n' "ghcr.io/ublue-os/${1%%:*}:${stream}" ;;
	esac
}

image_filesystem() {
	case "$1" in
		bluefin | bluefin:* | bazzite | bazzite:* | aurora | aurora:*) printf '%s\n' btrfs ;;
		fedora-bootc-rawhide | quay.io/fedora/fedora-bootc:rawhide) printf '%s\n' ext4 ;;
		*) return 1 ;;
	esac
}

image_disk_size() {
	case "$1" in
		bazzite | bazzite:*) printf '%s\n' 40G ;;
		*) printf '%s\n' 30G ;;
	esac
}

image_memory() {
	case "$1" in
		bazzite | bazzite:*) printf '%s\n' 6G ;;
		*) printf '%s\n' 4G ;;
	esac
}

usage() {
	echo "Usage: $0 <image|--all|--list> [base-image-ref]" >&2
	echo "       E2E_SCENARIO=extended $0 <image>  # also exercise --apply, rollback and the systemd units" >&2
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
	local phase="$1" extra_env="${2:-}" script
	script="$(cat "$GUEST_SCRIPT")"
	log "running $phase in the VM"
	bcvk libvirt ssh "$VM_NAME" \
		"TEST_PHASE=$phase $extra_env bash -c $(printf '%q' "$script")"
}

dump_guest_logs() {
	log "collecting logs from $VM_NAME"
	in_vm 'echo "--- bootc status ---"; bootc status --format=yaml || true;
	       echo "--- uupd log ---"; cat /var/log/uupd-e2e.log 2>/dev/null || true;
	       echo "--- uupd.service journal ---"; journalctl -b --no-pager -u uupd.service 2>/dev/null || true;
	       echo "--- uupd journal matches ---"; journalctl -b --no-pager 2>/dev/null | grep -i uupd || true' ||
		log "could not reach the VM to collect logs"
}

wait_for_reboot() {
	local old_boot_id="$1" attempts="${2:-60}" interval="${3:-10}" label="${4:-the VM}" boot_id
	for _ in $(seq 1 "$attempts"); do
		sleep "$interval"
		boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id' 2>/dev/null || true)"
		if [[ -n "$boot_id" && "$boot_id" != "$old_boot_id" ]]; then
			log "$label rebooted (boot id $boot_id)"
			return 0
		fi
	done
	return 1
}

run_extended() {
	log "=== running the extended scenario ==="

	local old_boot_id
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"

	log "building the second upgrade image and retagging $TEST_IMAGE"
	build_image upgraded2

	log "running uupd --apply; uupd should reboot the VM itself"
	run_phase phase_apply || log "SSH disconnected during uupd --apply (expected)"
	wait_for_reboot "$old_boot_id" 30 5 "uupd --apply" ||
		die "uupd --apply did not reboot the VM"
	run_phase phase_apply_verify

	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "queueing a rollback"
	run_phase phase_rollback
	in_vm 'systemctl reboot' || true
	wait_for_reboot "$old_boot_id" 60 10 "the rollback reboot" ||
		die "the VM did not reboot after bootc rollback"

	# Build the next update before verifying the rollback. bootc does not
	# re-offer an image that is already imported as the rollback deployment,
	# so the update check after a rollback needs a genuinely new image to see
	# an update.
	log "building the third upgrade image and retagging $TEST_IMAGE"
	build_image upgraded3

	run_phase phase_rollback_verify

	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "staging an update through uupd.service"
	run_phase phase_systemd
	in_vm 'systemctl reboot' || true
	wait_for_reboot "$old_boot_id" 60 10 "the service reboot" ||
		die "the VM did not reboot after the uupd.service update"
	run_phase phase_systemd_verify
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

	log "testing $image_name ($BASE_IMAGE), root filesystem $filesystem, scenario $SCENARIO"

	log "pulling $BASE_IMAGE"
	retry 5 60 podman pull --retry 5 "$BASE_IMAGE"

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

	local old_boot_id
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "boot id before the upgrade: $old_boot_id"

	if [[ "$SCENARIO" == "extended" ]]; then
		run_phase phase1 "E2E_EXTENDED=1"
	else
		run_phase phase1
	fi

	log "rebooting the VM"
	in_vm 'systemctl reboot' || true

	log "waiting for the VM to come back"
	wait_for_reboot "$old_boot_id" 60 10 "the VM" ||
		die "the VM did not reboot after the upgrade"

	run_phase phase2

	if [[ "$SCENARIO" == "extended" ]]; then
		run_extended
	fi

	log "PASS: $image_name upgraded with uupd ($SCENARIO)"
}

if [[ "${1:-}" == "--extended" ]]; then
	SCENARIO=extended
	shift
fi

if [[ "${1:-}" == "--list" || "${1:-}" == "-l" ]]; then
	printf '%s\n' "${IMAGES[@]}"
	exit 0
fi

if [[ -z "${1:-}" ]]; then
	usage
	exit 1
fi

[[ $EUID -ne 0 ]] || die "run this test as an unprivileged user (rootless podman and libvirt session), not as root"

# bcvk expects rootless podman with the matching per-user libvirt session.
# Honour an explicit override, otherwise default to the session daemon.
: "${LIBVIRT_DEFAULT_URI:=qemu:///session}"
export LIBVIRT_DEFAULT_URI

command -v podman >/dev/null || die "podman is required"
command -v bcvk >/dev/null || die "bcvk is required (see https://github.com/bootc-dev/bcvk)"
find_virtiofsd || log "WARNING: virtiofsd not found, bcvk may fail to share host storage"

if [[ "$1" == "--all" ]]; then
	overall=0
	for image in "${IMAGES[@]}"; do
		env -u BASE_IMAGE -u FILESYSTEM -u DISK_SIZE -u MEMORY \
			E2E_SCENARIO="$SCENARIO" "$0" "$image" || overall=1
	done
	exit "$overall"
fi

run_one "$@"
