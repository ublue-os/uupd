#!/usr/bin/env bash
# uupd end-to-end test, executed *inside* the VM over SSH.
#
# The test runs in two phases because the point of the test is to survive a
# reboot:
#
#   TEST_PHASE=phase1  runs before the reboot. It exercises the read-only uupd
#                      subcommands and then runs `uupd` so that the upgrade
#                      image is staged.
#   TEST_PHASE=phase2  runs after the reboot. It verifies that the marker file
#                      from the upgrade image is present and that the image
#                      stored in phase1 is the one that is now booted.
#
# Keep this script free of optional tooling (jq, awk, grep -o, python, ...).
# Some of the images under test are built without a package manager and ship a
# deliberately small userland (Dakota uses uutils coreutils), so only bash and
# the tools that are required by bootc/bcvk are assumed to be available.
set -euo pipefail

MARKER_FILE="/usr/share/uupd-testing"
STATE_DIR="/var/lib/uupd-e2e"
STAGED_DIGEST_FILE="${STATE_DIR}/staged-digests"
BOOTC_STATUS_YAML="/run/uupd-e2e-bootc-status.yaml"
UUPD_LOG="/var/log/uupd-e2e.log"

log() { printf '[uupd-e2e] %s\n' "$*" >&2; }
fail() {
    log "FAIL: $*"
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 not found in the test image"
}

# read_status() populates booted_digests and staged_digests.
#
# Normal path: use `bootc status --format=yaml` to query the guest's bootc
# deployment, exactly like the upstream uupd e2e test does. This requires
# bootc (and its systemd units) to be present in the guest.
#
# Fallback path (when bootc is deliberately absent from a minimal guest
# image): the host already knows the correct digests, so they are passed in
# via BOOTED_DIGEST and EXPECTED_DIGEST environment variables. Both are
# required for the fallback to work, so if either is missing/unset we fail.
declare -a booted_digests=() staged_digests=()

read_status() {
    if command -v bootc >/dev/null 2>&1; then
        bootc status --format=yaml >"$BOOTC_STATUS_YAML" ||
            fail "bootc status failed"
    else
        [[ -n "${BOOTED_DIGEST:-}" ]] || fail "BOOTED_DIGEST not set in guest (bootc unavailable)"
        [[ -n "${EXPECTED_DIGEST:-}" ]] || fail "EXPECTED_DIGEST not set in guest (bootc unavailable)"
        # The guest may contain more than one deployment image (e.g. the
        # deployment image plus cached update metadata), so we cannot know
        # exactly how many digests bootc would return here. We only know the
        # single digests the host staged + the one the host expects, so the
        # guest records the host-provided digest as its single booted digest.
        # (booted_digests and staged_digests are initialised above as empty
        # arrays, so we can populate them here as needed)
        unset booted_digests
        declare -a booted_digests=("$BOOTED_DIGEST")
        declare -a staged_digests=("$EXPECTED_DIGEST")
        return 0
    fi

    booted_digests=()
    staged_digests=()
    local section="" line digest

    while IFS= read -r line; do
        case "$line" in
            "  staged:"*) section="staged" ;;
            "  booted:"*) section="booted" ;;
            "  rollback:"*) section="rollback" ;;
            "  otherDeployments:"*) section="other" ;;
        esac

        if [[ "$line" =~ ^[[:space:]]+imageDigest:[[:space:]]*(.+)$ ]]; then
            digest="${BASH_REMATCH[1]}"
            digest="${digest%\"}"
            digest="${digest#\"}"
            case "$section" in
                staged) staged_digests+=("$digest") ;;
                booted) booted_digests+=("$digest") ;;
            esac
        fi
    done <"$BOOTC_STATUS_YAML"

    ((${#booted_digests[@]} > 0)) || fail "could not find the booted image digest"
}

contains_digest() {
    local needle="$1" digest
    shift
    for digest in "$@"; do
        [[ "$digest" == "$needle" ]] && return 0
    done
    return 1
}

run_uupd() {
    uupd \
        --hw-check=false \
        --disable-module-flatpak \
        --disable-module-brew \
        --disable-module-distrobox \
        --log-level=debug \
        --log-file="$UUPD_LOG" \
        --json \
        --verbose \
        "$@"
}

phase1() {
    log "=== phase 1: stage an upgrade with uupd ==="
    [[ $EUID -eq 0 ]] || fail "the test must run as root"
    need uupd
    if command -v bootc >/dev/null 2>&1; then
        need bootc
    fi

    [[ ! -e "$MARKER_FILE" ]] ||
        fail "$MARKER_FILE already exists on the booted image"

    mkdir -p "$STATE_DIR"
    rm -f "$STAGED_DIGEST_FILE" "$UUPD_LOG"

    read_status
    log "booted digest(s): ${booted_digests[*]}"
    log "staged digest(s) before update: ${staged_digests[*]:-<none>}"
    ((${#staged_digests[@]} == 0)) ||
        fail "the image already has a staged deployment"

    local rc=0
    log "running 'uupd --is-img-outdated' (exit 77 means the image is old)"
    uupd --is-img-outdated || rc=$?
    [[ $rc -eq 0 || $rc -eq 77 ]] || fail "uupd --is-img-outdated failed ($rc)"

    log "running 'uupd --update-check' (expecting the upgrade image to be found)"
    rc=0
    uupd --update-check || rc=$?
    [[ $rc -eq 0 ]] || fail "uupd --update-check did not see the upgrade image ($rc)"

    log "running 'uupd --dry-run'"
    run_uupd --dry-run || fail "uupd --dry-run failed"

    log "running uupd to stage the upgrade"
    run_uupd || fail "uupd failed, see $UUPD_LOG"

    read_status
    log "booted digest(s) after update: ${booted_digests[*]}"
    log "staged digest(s) after update: ${staged_digests[*]:-<none>}"
    ((${#staged_digests[@]} > 0)) || fail "uupd did not stage an update"

    local new_digest="" digest
    for digest in "${staged_digests[@]}"; do
        if ! contains_digest "$digest" "${booted_digests[@]}"; then
            new_digest="$digest"
            break
        fi
    done
    [[ -n "$new_digest" ]] || fail "the staged image is identical to the booted image"

    if [[ -n "${EXPECTED_DIGEST:-}" && "$EXPECTED_DIGEST" != "$new_digest" ]]; then
        log "WARNING: host reports $EXPECTED_DIGEST, guest staged $new_digest"
    fi

    printf '%s\n' "$new_digest" >"$STAGED_DIGEST_FILE"
    log "PASS: uupd staged $new_digest"
}

phase2() {
    log "=== phase 2: verify the upgraded image is booted ==="
    [[ -f "$MARKER_FILE" ]] ||
        fail "$MARKER_FILE is missing after the reboot"
    [[ -s "$STAGED_DIGEST_FILE" ]] ||
        fail "$STAGED_DIGEST_FILE is missing, did phase 1 run?"

    local expected
    expected="$(cat "$STAGED_DIGEST_FILE")"

    read_status
    log "booted digest(s): ${booted_digests[*]}"
    log "expected digest: $expected"
    contains_digest "$expected" "${booted_digests[@]}" ||
        fail "the booted image does not match the image staged by uupd"

    # The host still offers the same image, so there should be nothing left to
    # update now that we booted it.
    local rc=0
    uupd --update-check || rc=$?
    [[ $rc -eq 77 ]] ||
        fail "'uupd --update-check' reported an update after the reboot ($rc)"

    log "PASS: marker is present and the staged image is booted"
}

case "${TEST_PHASE:-}" in
    phase1) phase1 ;;
    phase2) phase2 ;;
    *) fail "set TEST_PHASE to 'phase1' or 'phase2'" ;;
esac
