#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

TEST_IMAGE="localhost/uupd-test:latest"
TEST_SCRIPT="tests/e2e/test-uupd-upgrade.sh"
CONTAINERFILE="tests/e2e/Containerfile.e2e"

log() { printf '[e2e] %s\n' "$*" >&2; }
die() {
    log "ERROR: $*"
    exit 1
}

list_images() {
    printf '%s\n' bazzite aurora bluefin fedora-bootc-rawhide dakota
}

base_image_for() {
    case "$1" in
        bazzite) printf '%s\n' ghcr.io/ublue-os/bazzite:stable ;;
        aurora) printf '%s\n' ghcr.io/ublue-os/aurora:stable ;;
        bluefin) printf '%s\n' ghcr.io/ublue-os/bluefin:stable ;;
        fedora-bootc-rawhide) printf '%s\n' ghcr.io/fedora/fedora-bootc:rawhide ;;
        dakota) printf '%s\n' ghcr.io/projectbluefin/dakota:stable ;;
        *) return 1 ;;
    esac
}

# Dakota pulls its filesystem tools from the bootc dependency, which only
# depends on xfsprogs; the other images are btrfs based.
filesystem_for() {
    case "$1" in
        dakota) printf '%s\n' xfs ;;
        *) printf '%s\n' btrfs ;;
    esac
}

usage() {
    echo "Usage: sudo $0 <image-name> [base-image]" >&2
    echo "Available image names:" >&2
    list_images | sed 's/^/  /' >&2
}

if [[ "${1:-}" == "--list" || "${1:-}" == "-l" ]]; then
    list_images
    exit 0
fi

if [[ -z "${1:-}" ]]; then
    usage
    exit 1
fi

[[ $EUID -eq 0 ]] || die "this test needs root (podman, libvirt, KVM); run it with sudo"

IMAGE_NAME="$1"
BASE_IMAGE="${2:-$(base_image_for "$IMAGE_NAME" || true)}"
[[ -n "$BASE_IMAGE" ]] || {
    usage
    die "unknown image name '$IMAGE_NAME'"
}

FILESYSTEM="${FILESYSTEM:-$(filesystem_for "$IMAGE_NAME")}"
DISK_SIZE="${DISK_SIZE:-20G}"
MEMORY="${MEMORY:-4G}"
VM_NAME="uupd-e2e-${IMAGE_NAME//[^A-Za-z0-9]/-}"

command -v podman >/dev/null || die "podman is required"
command -v bcvk >/dev/null || die "bcvk is required (see https://github.com/bootc-dev/bcvk)"

# Ubuntu/Debian install virtiofsd outside the paths bcvk searches.
if [[ -z "${VIRTIOFSD_BIN:-}" ]]; then
    for candidate in "$(command -v virtiofsd || true)" \
        /usr/lib/qemu/virtiofsd /usr/libexec/virtiofsd /usr/lib/virtiofsd; do
        if [[ -n "$candidate" && -x "$candidate" ]]; then
            export VIRTIOFSD_BIN="$candidate"
            break
        fi
    done
fi
[[ -n "${VIRTIOFSD_BIN:-}" ]] || log "WARNING: virtiofsd not found, bcvk may fail to share host storage"

build_image() {
    local marker="$1"
    podman build \
        --build-arg BASE_IMAGE="$BASE_IMAGE" \
        --build-arg MARKER="$marker" \
        -t "$TEST_IMAGE" \
        -f "$CONTAINERFILE" \
        .
}

in_vm() {
    bcvk libvirt ssh "$VM_NAME" "$1"
}

run_phase() {
    local phase="$1" script
    script="$(cat "$TEST_SCRIPT")"
    log "running $phase in the VM"
    bcvk libvirt ssh "$VM_NAME" \
        "TEST_PHASE=$phase BOOTED_DIGEST=${BOOTED_DIGEST:-} EXPECTED_DIGEST=${EXPECTED_DIGEST:-} bash -c $(printf '%q' "$script")"
}

dump_guest_logs() {
    log "collecting logs from $VM_NAME"
    in_vm 'echo "--- bootc status ---"; bootc status || true;
           echo "--- uupd log ---"; cat /var/log/uupd-e2e.log 2>/dev/null || true;
           echo "--- uupd journal ---"; journalctl -b --no-pager -u uupd.service 2>/dev/null || true' ||
        log "could not reach the VM to collect logs"
}

cleanup() {
    local rc=$?
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

wait_for_reboot() {
    local old_boot_id="$1" boot_id
    for _ in $(seq 1 24); do
        sleep 10
        boot_id="$(in_vm 'cat /proc/sys/kernel/random/boot_id' 2>/dev/null || true)"
        if [[ -n "$boot_id" && "$boot_id" != "$old_boot_id" ]]; then
            log "VM rebooted (boot id $boot_id)"
            return 0
        fi
    done
    return 1
}

log "testing $IMAGE_NAME ($BASE_IMAGE), root filesystem $FILESYSTEM"

log "pulling $BASE_IMAGE"
podman pull --retry 5 "$BASE_IMAGE"

log "building the base test image"
build_image 0
BOOTED_DIGEST="$(
    podman image inspect --format '{{.Digest}}' "$TEST_IMAGE" 2>/dev/null || true
)"
[[ -n "$BOOTED_DIGEST" ]] || log "WARNING: could not read the booted image digest from podman"

log "installing $TEST_IMAGE to a VM with bcvk"
bcvk libvirt run \
    --name "$VM_NAME" \
    --replace \
    --update-from-host \
    --pull=always \
    --filesystem "$FILESYSTEM" \
    --disk-size "$DISK_SIZE" \
    --memory "$MEMORY" \
    --firmware uefi-insecure \
    --detach \
    --ssh-wait \
    "$TEST_IMAGE"

log "building the upgrade image (MARKER=1) and retagging $TEST_IMAGE"
build_image 1
EXPECTED_DIGEST="$(podman image inspect --format '{{.Digest}}' "$TEST_IMAGE" || true)"
[[ -n "$EXPECTED_DIGEST" ]] || log "WARNING: could not read the image digest from podman"
log "upgrade image digest: ${EXPECTED_DIGEST:-<unknown>}"

OLD_BOOT_ID="$(in_vm 'cat /proc/sys/kernel/random/boot_id')"
log "boot id before the upgrade: $OLD_BOOT_ID"

run_phase phase1

log "rebooting the VM"
in_vm 'systemctl reboot' || true

log "waiting for the VM to come back"
wait_for_reboot "$OLD_BOOT_ID" || die "the VM did not reboot after the upgrade"

run_phase phase2

log "PASS: $IMAGE_NAME upgraded with uupd"
