#!/usr/bin/env bash
set -euo pipefail

MARKER_FILE="/usr/share/uupd-testing"
STATE_DIR="/var/lib/uupd-e2e"
STAGED_DIGEST_FILE="${STATE_DIR}/staged-digest"
UUPD_LOG="/var/log/uupd-e2e.log"

log() { printf '[uupd-e2e] %s\n' "$*" >&2; }
fail() {
	log "FAIL: $*"
	exit 1
}

image_digest() {
	local section="$1"
	bootc status --format=yaml | awk -v want="$section" '
		/^  (staged|booted|rollback|otherDeployments):/ {
			section = $1
			sub(/:$/, "", section)
		}
		section == want && /imageDigest:/ {
			gsub(/[" ]/, "", $2)
			print $2
			exit
		}
	'
}

run_uupd() {
	uupd \
		--hw-check=false \
		--disable-module-flatpak \
		--disable-module-brew \
		--disable-module-distrobox \
		--log-level=debug \
		--log-file="$UUPD_LOG" \
		"$@"
}

dump_log() {
	[[ -f "$UUPD_LOG" ]] || return 0
	log "--- last 200 lines of $UUPD_LOG ---"
	tail -n 200 "$UUPD_LOG" >&2 || true
}

phase1() {
	log "=== phase 1: stage the rebuilt image with uupd ==="
	[[ $EUID -eq 0 ]] || fail "must run as root"
	command -v uupd >/dev/null || fail "uupd is not installed"
	command -v bootc >/dev/null || fail "bootc is not installed"
	[[ ! -e "$MARKER_FILE" ]] || fail "$MARKER_FILE already exists in the booted image"

	log "uupd version: $(uupd --version 2>/dev/null || echo unknown)"

	mkdir -p "$STATE_DIR"
	rm -f "$STAGED_DIGEST_FILE" "$UUPD_LOG"

	local booted staged_before
	booted="$(image_digest booted)"
	staged_before="$(image_digest staged)"
	log "booted digest: ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ -z "$staged_before" ]] || fail "a deployment is already staged: $staged_before"

	local rc=0
	uupd is-img-outdated || rc=$?
	[[ $rc -eq 0 || $rc -eq 77 ]] || fail "uupd is-img-outdated exited $rc"

	log "checking that uupd sees the rebuilt image as an update"
	rc=0
	uupd update-check || rc=$?
	[[ $rc -eq 0 ]] || fail "uupd update-check did not find an update (exit $rc)"

	log "running uupd --dry-run"
	run_uupd --dry-run || {
		dump_log
		fail "uupd --dry-run failed"
	}

	log "running uupd to stage the update"
	run_uupd || {
		dump_log
		fail "uupd failed to stage the update"
	}

	local staged
	staged="$(image_digest staged)"
	[[ -n "$staged" ]] || {
		dump_log
		fail "uupd did not stage a deployment"
	}
	[[ "$staged" != "$booted" ]] || fail "staged deployment is identical to the booted image"

	printf '%s\n' "$staged" >"$STAGED_DIGEST_FILE"
	log "PASS: uupd staged $staged"
}

phase2() {
	log "=== phase 2: verify the staged image booted ==="
	[[ -f "$MARKER_FILE" ]] || fail "$MARKER_FILE is missing after the reboot"
	[[ -s "$STAGED_DIGEST_FILE" ]] || fail "$STAGED_DIGEST_FILE is missing; phase 1 did not complete"

	local expected booted
	expected="$(cat "$STAGED_DIGEST_FILE")"
	booted="$(image_digest booted)"
	log "expected digest: $expected"
	log "booted digest:   ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ "$booted" == "$expected" ]] || fail "booted image does not match what uupd staged"

	local rc=0
	uupd update-check || rc=$?
	[[ $rc -eq 77 ]] || fail "uupd update-check still reports an update after the reboot (exit $rc)"

	log "PASS: marker is present and the booted image matches what uupd staged"
}

case "${TEST_PHASE:-}" in
	phase1) phase1 ;;
	phase2) phase2 ;;
	*) fail "set TEST_PHASE to 'phase1' or 'phase2'" ;;
esac
