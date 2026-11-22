#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

TEST_IMAGE="localhost/uupd-e2e:latest"
CONTAINERFILE="tests/e2e/Containerfile.e2e"
GUEST_SCRIPT="tests/e2e/guest-verify.sh"

IMAGES=(bluefin bazzite aurora quay.io/fedora/fedora-bootc:rawhide)
SUITES=(core cli extended)

SUMMARY=()

log() { printf '[e2e] %s\n' "$*" >&2; }
die() {
	log "ERROR: $*"
	exit 1
}

group() {
	if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
		printf '::group::%s\n' "$*" >&2
	else
		log "=== $* ==="
	fi
}
endgroup() {
	if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
		printf '::endgroup::\n' >&2
	fi
}
record() {
	SUMMARY+=("$1")
	if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
		printf '%s\n' "$1" >>"$GITHUB_STEP_SUMMARY"
	fi
	log "$1"
}

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
	echo "Usage: $0 [options] <image|--all>" >&2
	echo "       --suite <core|cli|extended|all>   suite to run (default: core)" >&2
	echo "       --list / -l                       list images" >&2
	echo "       --list-suites                     list suites" >&2
	printf 'Images: %s\n' "${IMAGES[*]}" >&2
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
	bcvk libvirt ssh "$VM_NAME" \
		"TEST_PHASE=$phase $extra_env bash -c $(printf '%q' "$script") 2>&1"
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
	group "extended: --apply"
	local old_boot_id
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "building upgraded2 and retagging $TEST_IMAGE"
	build_image upgraded2
	run_phase phase_apply || log "SSH disconnected during uupd --apply (expected)"
	wait_for_reboot "$old_boot_id" 60 5 "uupd --apply" ||
		die "uupd --apply did not reboot the VM"
	run_phase phase_apply_verify
	endgroup

	group "extended: rollback"
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	run_phase phase_rollback
	in_vm 'systemctl reboot' || true
	wait_for_reboot "$old_boot_id" 60 10 "the rollback reboot" ||
		die "the VM did not reboot after bootc rollback"
	log "building upgraded3 and retagging $TEST_IMAGE"
	build_image upgraded3
	run_phase phase_rollback_verify
	endgroup

	group "extended: systemd units"
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	run_phase phase_systemd
	in_vm 'systemctl reboot' || true
	wait_for_reboot "$old_boot_id" 60 10 "the service reboot" ||
		die "the VM did not reboot after the uupd.service update"
	run_phase phase_systemd_verify
	endgroup
}

run_upgrade() {
	group "core: stage and reboot"
	log "building upgraded and retagging $TEST_IMAGE"
	build_image upgraded

	local old_boot_id
	old_boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
	log "boot id before the upgrade: $old_boot_id"

	run_phase phase1

	log "rebooting the VM"
	in_vm 'systemctl reboot' || true
	wait_for_reboot "$old_boot_id" 60 10 "the VM" ||
		die "the VM did not reboot after the upgrade"
	endgroup

	group "core: verify booted image"
	run_phase phase2
	endgroup
}

run_one() {
	local image_name="$1" suite="$2"
	BASE_IMAGE="${BASE_IMAGE:-${3:-}}"
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
			record "FAIL: $suite on $image_name"
		fi
		if [[ "${KEEP_VM:-0}" != "1" ]]; then
			bcvk libvirt rm -f "$VM_NAME" >/dev/null 2>&1 || true
		else
			log "KEEP_VM=1, leaving $VM_NAME running"
		fi
		exit "$rc"
	}
	trap cleanup EXIT

	group "provision $image_name"
	log "testing $image_name ($BASE_IMAGE), root filesystem $filesystem, suite $suite"
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
	endgroup

	case "$suite" in
		cli)
			group "cli suite"
			run_phase cli
			endgroup
			;;
		core | extended)
			run_upgrade
			if [[ "$suite" == "extended" ]]; then
				run_extended
			fi
			;;
	esac
	record "PASS: $suite on $image_name"
}

SUITE="${E2E_SUITE:-${E2E_SCENARIO:-core}}"
ALL_IMAGES=0

while [[ $# -gt 0 ]]; do
	case "$1" in
		--suite)
			SUITE="${2:-}"
			shift 2
			;;
		--list | -l)
			printf '%s\n' "${IMAGES[@]}"
			exit 0
			;;
		--list-suites)
			printf '%s\n' "${SUITES[@]}"
			exit 0
			;;
		--extended)
			SUITE=extended
			shift
			;;
		--all)
			ALL_IMAGES=1
			shift
			;;
		-*)
			die "unknown option '$1'"
			;;
		*)
			break
			;;
	esac
done

case "$SUITE" in
	core | cli | extended | all) ;;
	*) die "unknown suite '$SUITE' (expected core, cli, extended or all)" ;;
esac

[[ $EUID -ne 0 ]] || die "run this test as an unprivileged user (rootless podman and libvirt session), not as root"

: "${LIBVIRT_DEFAULT_URI:=qemu:///session}"
export LIBVIRT_DEFAULT_URI

command -v podman >/dev/null || die "podman is required"
command -v bcvk >/dev/null || die "bcvk is required (see https://github.com/bootc-dev/bcvk)"
find_virtiofsd || log "WARNING: virtiofsd not found, bcvk may fail to share host storage"

if [[ -n "${1:-}" ]]; then
	[[ "$ALL_IMAGES" -eq 0 ]] || die "--all takes no image argument"
	suites_to_run=("$SUITE")
	[[ "$SUITE" == "all" ]] && suites_to_run=(core cli extended)
	for suite in "${suites_to_run[@]}"; do
		run_one "$1" "$suite"
	done
	exit 0
fi

if [[ "$ALL_IMAGES" -eq 1 ]]; then
	for image in "${IMAGES[@]}"; do
		suites_to_run=("$SUITE")
		[[ "$SUITE" == "all" ]] && suites_to_run=(core cli extended)
		for suite in "${suites_to_run[@]}"; do
			env -u BASE_IMAGE -u FILESYSTEM -u DISK_SIZE -u MEMORY \
				E2E_SUITE="$suite" "$0" "$image" || exit 1
		done
	done
	exit 0
fi

usage
exit 1
