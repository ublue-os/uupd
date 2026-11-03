#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="/var/lib/uupd-e2e"
STAGED_DIGEST_FILE="${STATE_DIR}/staged-digest"
BOOTED_DIGEST_FILE="${STATE_DIR}/booted-digest"
APPLIED_DIGEST_FILE="${STATE_DIR}/applied-digest"
UUPD_LOG="/var/log/uupd-e2e.log"

FAKE_BOOTC="/tmp/e2e-fake-bootc"
FAKE_BOOTC_CALLS="/tmp/e2e-fake-bootc-calls"
FAKE_FLATPAK="/tmp/e2e-fake-flatpak"
FAKE_FLATPAK_CALLS="/tmp/e2e-fake-flatpak-calls"

log() { printf '[uupd-e2e] %s\n' "$*" >&2; }
fail() {
	log "FAIL: $*"
	exit 1
}
pass() { log "PASS: $*"; }

require_root() { [[ $EUID -eq 0 ]] || fail "must run as root"; }

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

assert_no_staged() {
	local staged
	staged="$(image_digest staged)"
	[[ -z "$staged" ]] || fail "unexpected staged deployment: $staged"
}

run_uupd() {
	uupd \
		--hw-check=false \
		--log-level=debug \
		--log-file="$UUPD_LOG" \
		--disable-module-brew \
		--disable-module-flatpak \
		--disable-module-distrobox \
		"$@"
}

dump_log() {
	[[ -f "$UUPD_LOG" ]] || return 0
	log "--- last 200 lines of $UUPD_LOG ---"
	tail -n 200 "$UUPD_LOG" >&2 || true
}

expect_rc() {
	local want="$1" desc="$2"
	shift 2
	local rc=0
	"$@" || rc=$?
	[[ "$rc" -eq "$want" ]] || fail "$desc: expected exit $want, got $rc"
	pass "$desc (exit $want)"
}

expect_fail() {
	local desc="$1"
	shift
	local rc=0
	"$@" || rc=$?
	[[ "$rc" -ne 0 ]] || fail "$desc: expected a non-zero exit, got 0"
	pass "$desc (exit $rc)"
}

json_lines() {
	local file="$1"
	[[ -s "$file" ]] || fail "$file is empty"
	if command -v jq >/dev/null 2>&1; then
		local line
		while IFS= read -r line; do
			[[ -n "$line" ]] || continue
			printf '%s\n' "$line" | jq -e . >/dev/null 2>&1 ||
				fail "invalid JSON line in $file: $line"
		done <"$file"
	elif command -v python3 >/dev/null 2>&1; then
		python3 - "$file" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as handle:
    for line in handle:
        line = line.strip()
        if line:
            json.loads(line)
PY
	else
		fail "no JSON validator (jq or python3) found"
	fi
	pass "$file contains valid JSON lines"
}

write_fake_bootc() {
	cat >"$FAKE_BOOTC" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> /tmp/e2e-fake-bootc-calls
if [[ "${1:-}" == "status" && "${2:-}" == "--format=json" && -n "${FAKE_BOOTC_TIMESTAMP:-}" ]]; then
	printf '{"status":{"booted":{"incompatible":false,"image":{"timestamp":"%s"}},"staged":{"incompatible":false,"image":{"timestamp":""}}}}\n' "$FAKE_BOOTC_TIMESTAMP"
	exit 0
fi
if [[ "${1:-}" == "upgrade" ]]; then
	if [[ "$*" == *--check* ]]; then
		printf 'No changes in: e2e fake\n'
		exit 0
	fi
	exit "${FAKE_BOOTC_UPGRADE_RC:-0}"
fi
exec /usr/bin/bootc "$@"
EOF
	chmod 0755 "$FAKE_BOOTC"
}

write_fake_flatpak() {
	cat >"$FAKE_FLATPAK" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> /tmp/e2e-fake-flatpak-calls
exit "${FAKE_FLATPAK_RC:-0}"
EOF
	chmod 0755 "$FAKE_FLATPAK"
}

has_regular_users() {
	getent passwd | awk -F: '
		$3 >= 1000 && $3 < 65534 && $6 != "/" && $7 !~ /(nologin|false)$/ { found = 1 }
		END { exit(found ? 0 : 1) }
	'
}

non_root_guard() {
	log "checking that privileged commands refuse to run as non-root"
	command -v runuser >/dev/null 2>&1 || fail "runuser is required for the non-root tests"

	local sub rc
	for sub in "" update-check is-img-outdated hw-check wait; do
		local -a args=()
		[[ -n "$sub" ]] && args=("$sub")
		rc=0
		runuser -u nobody -- uupd "${args[@]}" >/tmp/e2e-nonroot.log 2>&1 || rc=$?
		[[ "$rc" -ne 0 ]] || fail "uupd ${sub:-<update>} as non-root unexpectedly succeeded"
		grep -q "needs to be invoked as root" /tmp/e2e-nonroot.log ||
			fail "uupd ${sub:-<update>} did not report the root requirement"
	done
	pass "privileged commands reject non-root users"

	runuser -u nobody -- uupd config-dump >/tmp/e2e-nonroot-dump.json ||
		fail "config-dump should work as a non-root user"
	jq -e . /tmp/e2e-nonroot-dump.json >/dev/null || fail "config-dump as non-root did not emit JSON"
	pass "config-dump works as a non-root user"
}

config_checks() {
	log "checking config parsing and precedence"
	cat >/tmp/e2e-config-a.json <<'EOF'
{
    "checks": {"hardware": {"enable": false}},
    "modules": {"system": {"disable": true}}
}
EOF
	uupd --config /tmp/e2e-config-a.json config-dump >/tmp/e2e-dump-a.json ||
		fail "config-dump failed"
	jq -e '.modules.system.disable == true' /tmp/e2e-dump-a.json >/dev/null ||
		fail "config file did not disable the system module"
	jq -e '.checks.hardware.enable == false' /tmp/e2e-dump-a.json >/dev/null ||
		fail "config file did not disable hardware checks"
	jq -e '.modules.system["bootc-binary"] == "/usr/bin/bootc"' /tmp/e2e-dump-a.json >/dev/null ||
		fail "config-dump did not include the default bootc binary"
	pass "config file values and defaults are reflected in config-dump"

	UUPD_BOOTC_BINARY="$FAKE_BOOTC" uupd --config /tmp/e2e-config-a.json config-dump |
		jq -e --arg path "$FAKE_BOOTC" '.modules.system["bootc-binary"] == $path' >/dev/null ||
		fail "UUPD_BOOTC_BINARY did not override the config value"
	pass "environment variables override config values"

	expect_fail "config-dump rejects a missing config file" \
		uupd --config /tmp/e2e-does-not-exist.json config-dump
	printf '{"modules":' >/tmp/e2e-config-malformed.json
	expect_fail "config-dump rejects malformed JSON" \
		uupd --config /tmp/e2e-config-malformed.json config-dump
	printf '{"bogus": true}\n' >/tmp/e2e-config-unknown.json
	expect_fail "config-dump rejects unknown keys" \
		uupd --config /tmp/e2e-config-unknown.json config-dump
}

fake_bootc_checks() {
	log "checking system-module behavior with a fake bootc"
	write_fake_bootc
	cat >/tmp/e2e-config-fake.json <<EOF
{
    "checks": {"hardware": {"enable": false}},
    "modules": {"system": {"bootc-binary": "$FAKE_BOOTC"}}
}
EOF

	: >"$FAKE_BOOTC_CALLS"
	expect_rc 0 "uupd runs the system module without --force" \
		uupd --config /tmp/e2e-config-fake.json --hw-check=false --log-level=debug \
		--log-file=/tmp/e2e-fake-run.log \
		--disable-module-brew --disable-module-flatpak --disable-module-distrobox
	grep -qx 'upgrade --check' "$FAKE_BOOTC_CALLS" ||
		fail "uupd did not run bootc upgrade --check without --force"
	grep -qx 'upgrade --quiet --progress-fd 3' "$FAKE_BOOTC_CALLS" ||
		fail "uupd did not run bootc upgrade without --force"
	pass "the system module checks and updates"

	: >"$FAKE_BOOTC_CALLS"
	expect_rc 0 "uupd --force skips the update check" \
		uupd --config /tmp/e2e-config-fake.json --hw-check=false --log-level=debug \
		--log-file=/tmp/e2e-fake-force.log --force \
		--disable-module-brew --disable-module-flatpak --disable-module-distrobox
	if grep -qx 'upgrade --check' "$FAKE_BOOTC_CALLS"; then
		fail "--force still ran bootc upgrade --check"
	fi
	grep -qx 'upgrade --quiet --progress-fd 3' "$FAKE_BOOTC_CALLS" ||
		fail "--force did not run bootc upgrade"
	pass "--force runs the update without checking first"

	: >"$FAKE_BOOTC_CALLS"
	expect_rc 0 "uupd --ci disables the system module" \
		uupd --config /tmp/e2e-config-fake.json --hw-check=false --log-level=debug \
		--log-file=/tmp/e2e-fake-ci.log --ci \
		--disable-module-brew --disable-module-flatpak --disable-module-distrobox
	if grep -q '^upgrade' "$FAKE_BOOTC_CALLS"; then
		fail "--ci still ran the system module"
	fi
	pass "--ci leaves the system image alone"

	: >"$FAKE_BOOTC_CALLS"
	expect_rc 0 "uupd --disable-module-system skips the system module" \
		uupd --config /tmp/e2e-config-fake.json --hw-check=false --log-level=debug \
		--log-file=/tmp/e2e-fake-disabled.log --disable-module-system \
		--disable-module-brew --disable-module-flatpak --disable-module-distrobox
	if grep -q '^upgrade' "$FAKE_BOOTC_CALLS"; then
		fail "--disable-module-system still ran the system module"
	fi
	pass "--disable-module-system overrides the config and skips the update"

	: >"$FAKE_BOOTC_CALLS"
	expect_fail "a failed system update fails the run" \
		env FAKE_BOOTC_UPGRADE_RC=1 uupd --config /tmp/e2e-config-fake.json \
		--hw-check=false --log-level=debug --log-file=/tmp/e2e-fake-fail.log --force \
		--disable-module-brew --disable-module-flatpak --disable-module-distrobox
	grep -q "Updates finished with errors" /tmp/e2e-fake-fail.log ||
		fail "a failed system update was not reported"
	assert_no_staged
	pass "a failed system update does not stage anything"

	expect_rc 77 "update-check reports 77 when bootc sees no changes" \
		uupd --config /tmp/e2e-config-fake.json --quiet update-check

	local old_timestamp new_timestamp
	old_timestamp="$(date -u -d '-2 months' '+%Y-%m-%dT%H:%M:%SZ')"
	new_timestamp="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
	expect_rc 77 "is-img-outdated reports an old image" \
		env FAKE_BOOTC_TIMESTAMP="$old_timestamp" \
		uupd --config /tmp/e2e-config-fake.json --quiet is-img-outdated
	expect_rc 0 "is-img-outdated accepts a fresh image" \
		env FAKE_BOOTC_TIMESTAMP="$new_timestamp" \
		uupd --config /tmp/e2e-config-fake.json --quiet is-img-outdated
}

module_checks() {
	log "checking module command construction and degradation"
	write_fake_bootc
	write_fake_flatpak

	cat >/tmp/e2e-config-flatpak.json <<EOF
{
    "checks": {"hardware": {"enable": false}},
    "modules": {
        "system": {"bootc-binary": "$FAKE_BOOTC"},
        "flatpak": {"binary-path": "$FAKE_FLATPAK"},
        "brew": {"disable": true},
        "distrobox": {"disable": true}
    }
}
EOF

	if has_regular_users; then
		log "WARN: regular users present, skipping the flatpak module run test"
	else
		: >"$FAKE_FLATPAK_CALLS"
		expect_rc 0 "the flatpak module runs its update command" \
			uupd --config /tmp/e2e-config-flatpak.json --hw-check=false \
			--log-level=debug --log-file=/tmp/e2e-flatpak.log
		grep -qx 'update -y --noninteractive' "$FAKE_FLATPAK_CALLS" ||
			fail "the flatpak module did not run flatpak update"
		pass "the flatpak module invokes flatpak update"

		: >"$FAKE_FLATPAK_CALLS"
		: >"$FAKE_BOOTC_CALLS"
		expect_fail "a failed flatpak update fails the run" \
			env FAKE_FLATPAK_RC=1 uupd --config /tmp/e2e-config-flatpak.json \
			--hw-check=false --log-level=debug --log-file=/tmp/e2e-flatpak-fail.log
		grep -q "Exited with failed updates" /tmp/e2e-flatpak-fail.log ||
			fail "a failed flatpak update was not reported"
		grep -qx 'upgrade --quiet --progress-fd 3' "$FAKE_BOOTC_CALLS" ||
			fail "the system module did not keep running after a flatpak failure"
		pass "module failures are aggregated without skipping the system update"
	fi

	cat >/tmp/e2e-config-missing.json <<EOF
{
    "checks": {"hardware": {"enable": false}},
    "modules": {
        "system": {"bootc-binary": "$FAKE_BOOTC"},
        "flatpak": {"binary-path": "/nonexistent/flatpak"},
        "brew": {"prefix": "/nonexistent/linuxbrew"},
        "distrobox": {"binary-path": "/nonexistent/distrobox"}
    }
}
EOF
	: >"$FAKE_BOOTC_CALLS"
	expect_rc 0 "modules with missing binaries are disabled" \
		uupd --config /tmp/e2e-config-missing.json --log-level=debug \
		--log-file=/tmp/e2e-degraded.log
	grep -q "Flatpak driver failed to initialize" /tmp/e2e-degraded.log ||
		fail "the flatpak driver was not disabled gracefully"
	grep -q "Brew driver failed to initialize" /tmp/e2e-degraded.log ||
		fail "the brew driver was not disabled gracefully"
	grep -q "Distrobox driver failed to initialize" /tmp/e2e-degraded.log ||
		fail "the distrobox driver was not disabled gracefully"
	grep -qx 'upgrade --quiet --progress-fd 3' "$FAKE_BOOTC_CALLS" ||
		fail "the system module did not run when other modules were missing"
	pass "missing module binaries are skipped instead of failing the update"
}

lock_check() {
	log "checking lockfile contention"
	flock -x /run/uupd.lock sleep 20 &
	local holder=$!
	sleep 1
	local rc=0
	uupd --config /tmp/e2e-config-fake.json --hw-check=false --log-level=debug \
		--log-file=/tmp/e2e-lock.log --dry-run >/dev/null 2>&1 || rc=$?
	kill "$holder" 2>/dev/null || true
	wait "$holder" 2>/dev/null || true
	[[ "$rc" -ne 0 ]] || fail "uupd did not fail while the lock was held"
	grep -q "Could not acquire lock" /tmp/e2e-lock.log ||
		fail "uupd did not report the lock contention"
	pass "a second uupd fails cleanly when the lock is held"
}

hardware_check() {
	log "checking the hardware check gate"
	cat >/tmp/e2e-config-hw-bad.json <<'EOF'
{
    "checks": {"hardware": {"enable": true, "mem-max-percent": 0}},
    "modules": {
        "flatpak": {"disable": true},
        "brew": {"disable": true},
        "distrobox": {"disable": true}
    }
}
EOF
	expect_fail "hardware checks stop an update" \
		uupd --config /tmp/e2e-config-hw-bad.json --hw-check=true --dry-run --log-file=/tmp/e2e-hw.log
	grep -q "Hardware checks failed" /tmp/e2e-hw.log ||
		fail "the hardware failure was not reported"
	pass "failing hardware checks abort the update"

	expect_rc 0 "hardware checks can be bypassed" \
		uupd --config /tmp/e2e-config-hw-bad.json --hw-check=false --dry-run \
		--log-level=debug --log-file=/tmp/e2e-hw-bypass.log
	pass "--hw-check=false bypasses the hardware gate"

	expect_fail "the hw-check subcommand reports the failure" \
		uupd --config /tmp/e2e-config-hw-bad.json hw-check --log-file=/tmp/e2e-hw-subcommand.log
}

json_logging_checks() {
	log "checking JSON logging and log files"
	uupd --config /tmp/e2e-config-a.json --hw-check=false --dry-run --json \
		>/tmp/e2e-json.log 2>&1 || fail "uupd --json failed"
	json_lines /tmp/e2e-json.log

	: >/tmp/e2e-logfile.log
	uupd --config /tmp/e2e-config-a.json --hw-check=false --dry-run \
		--log-file=/tmp/e2e-logfile.log || fail "uupd --log-file failed"
	json_lines /tmp/e2e-logfile.log
	local before after
	before="$(wc -l </tmp/e2e-logfile.log)"
	uupd --config /tmp/e2e-config-a.json --hw-check=false --dry-run \
		--log-file=/tmp/e2e-logfile.log || fail "uupd --log-file failed on the second run"
	after="$(wc -l </tmp/e2e-logfile.log)"
	[[ "$after" -gt "$before" ]] || fail "the log file was not appended to"
	pass "log files are valid JSON and are appended to"

	expect_fail "an invalid log level is rejected" uupd --log-level=bogus --dry-run
}

extended_prechecks() {
	log "=== extended prechecks ==="
	command -v jq >/dev/null 2>&1 || fail "jq is required for the extended scenario"
	command -v flock >/dev/null 2>&1 || fail "flock is required for the extended scenario"
	write_fake_bootc

	non_root_guard
	config_checks
	fake_bootc_checks
	module_checks
	lock_check
	hardware_check
	json_logging_checks
	assert_no_staged
	pass "extended prechecks left no staged deployment behind"
}

phase1() {
	log "=== phase 1: stage the rebuilt image with uupd ==="
	require_root
	command -v uupd >/dev/null || fail "uupd is not installed"
	command -v bootc >/dev/null || fail "bootc is not installed"

	log "uupd version: $(uupd --version 2>/dev/null || echo unknown)"

	mkdir -p "$STATE_DIR"
	rm -f "$STAGED_DIGEST_FILE" "$BOOTED_DIGEST_FILE" "$APPLIED_DIGEST_FILE" "$UUPD_LOG"
	rm -f "$FAKE_BOOTC_CALLS" "$FAKE_FLATPAK_CALLS"

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
	expect_rc 0 "update-check finds the rebuilt image" uupd update-check

	if [[ "${E2E_EXTENDED:-0}" == "1" ]]; then
		extended_prechecks
	fi

	log "running uupd --dry-run"
	run_uupd --dry-run || {
		dump_log
		fail "uupd --dry-run failed"
	}
	assert_no_staged
	pass "uupd --dry-run did not stage a deployment"

	local boot_id_before boot_id_after
	boot_id_before="$(cat /proc/sys/kernel/random/boot_id)"

	log "running uupd to stage the update"
	run_uupd || {
		dump_log
		fail "uupd failed to stage the update"
	}

	boot_id_after="$(cat /proc/sys/kernel/random/boot_id)"
	[[ "$boot_id_before" == "$boot_id_after" ]] ||
		fail "uupd rebooted even though --apply was not passed"
	pass "uupd did not reboot without --apply"

	local staged
	staged="$(image_digest staged)"
	[[ -n "$staged" ]] || {
		dump_log
		fail "uupd did not stage a deployment"
	}
	[[ "$staged" != "$booted" ]] || fail "staged deployment is identical to the booted image"
	grep -q "Updates Completed Successfully" "$UUPD_LOG" ||
		fail "uupd did not report a successful update"

	printf '%s\n' "$staged" >"$STAGED_DIGEST_FILE"
	pass "uupd staged $staged"
}

phase2() {
	log "=== phase 2: verify the staged image booted ==="
	require_root
	[[ -s "$STAGED_DIGEST_FILE" ]] || fail "$STAGED_DIGEST_FILE is missing; phase 1 did not complete"

	local expected booted
	expected="$(cat "$STAGED_DIGEST_FILE")"
	booted="$(image_digest booted)"
	log "expected digest: $expected"
	log "booted digest:   ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ "$booted" == "$expected" ]] || fail "booted image does not match what uupd staged"
	[[ -f /usr/share/uupd-e2e/upgraded ]] || fail "the upgraded payload is missing"
	pass "the booted image matches what uupd staged"

	expect_rc 77 "update-check reports no update after the reboot" uupd update-check
	local outdated_rc=0
	uupd is-img-outdated || outdated_rc=$?
	[[ "$outdated_rc" -eq 0 || "$outdated_rc" -eq 77 ]] ||
		fail "uupd is-img-outdated exited $outdated_rc"
	pass "is-img-outdated returned $outdated_rc for the booted image"

	log "running uupd again on the up-to-date image"
	local boot_id_before boot_id_after
	boot_id_before="$(cat /proc/sys/kernel/random/boot_id)"
	run_uupd || {
		dump_log
		fail "uupd failed on an up-to-date image"
	}
	boot_id_after="$(cat /proc/sys/kernel/random/boot_id)"
	[[ "$boot_id_before" == "$boot_id_after" ]] || fail "uupd rebooted on an up-to-date image"
	assert_no_staged
	expect_rc 77 "update-check still reports no update after a second run" uupd update-check
	pass "a second run is a no-op"

	local rollback
	rollback="$(image_digest rollback)"
	[[ -n "$rollback" ]] || fail "no rollback deployment is available"
	[[ "$rollback" != "$booted" ]] || fail "the rollback deployment is the booted image"

	printf '%s\n' "$booted" >"$BOOTED_DIGEST_FILE"
	pass "the booted image is recorded for the extended scenario"
}

phase_apply() {
	log "=== extended: stage a second update with --apply ==="
	require_root
	run_uupd --apply || true
	log "uupd --apply returned; the host will verify that it rebooted the VM"
}

phase_apply_verify() {
	log "=== extended: verify the --apply reboot ==="
	require_root
	[[ -s "$BOOTED_DIGEST_FILE" ]] || fail "$BOOTED_DIGEST_FILE is missing"
	local previous booted
	previous="$(cat "$BOOTED_DIGEST_FILE")"
	booted="$(image_digest booted)"
	log "previous digest: $previous"
	log "booted digest:   ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ "$booted" != "$previous" ]] || fail "uupd --apply did not boot a new image"
	[[ -f /usr/share/uupd-e2e/upgraded2 ]] || fail "the upgraded2 payload is missing"
	[[ ! -f /usr/share/uupd-e2e/upgraded3 ]] || fail "the upgraded3 payload booted too early"
	assert_no_staged
	expect_rc 77 "update-check reports no update after uupd --apply" uupd update-check
	printf '%s\n' "$booted" >"$APPLIED_DIGEST_FILE"
	pass "uupd --apply staged and rebooted into $booted"
}

phase_rollback() {
	log "=== extended: queue a rollback ==="
	require_root
	[[ -s "$BOOTED_DIGEST_FILE" ]] || fail "$BOOTED_DIGEST_FILE is missing"
	log "queueing rollback to $(cat "$BOOTED_DIGEST_FILE")"
	bootc rollback || fail "bootc rollback failed"
	pass "rollback is queued for the next boot"
}

phase_rollback_verify() {
	log "=== extended: verify the rollback ==="
	require_root
	[[ -s "$BOOTED_DIGEST_FILE" ]] || fail "$BOOTED_DIGEST_FILE is missing"
	local expected booted
	expected="$(cat "$BOOTED_DIGEST_FILE")"
	booted="$(image_digest booted)"
	log "expected digest: $expected"
	log "booted digest:   ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ "$booted" == "$expected" ]] || fail "the rollback did not boot the previous image"
	[[ -f /usr/share/uupd-e2e/upgraded ]] || fail "the upgraded payload is missing after rollback"
	[[ ! -f /usr/share/uupd-e2e/upgraded2 ]] || fail "the upgraded2 payload is still present after rollback"
	assert_no_staged
	expect_rc 0 "update-check finds an update again after the rollback" uupd update-check
	pass "the previous deployment booted cleanly after rollback"
}

phase_systemd() {
	log "=== extended: stage an update through uupd.service ==="
	require_root

	mkdir -p /etc/uupd
	cat >/etc/uupd/config.json <<'EOF'
{
    "checks": {"hardware": {"enable": false}},
    "modules": {
        "flatpak": {"disable": true},
        "brew": {"disable": true},
        "distrobox": {"disable": true}
    }
}
EOF

	systemctl daemon-reload
	systemctl cat uupd-manual.service >/dev/null || fail "uupd-manual.service is not installed"
	systemd-analyze verify /usr/lib/systemd/system/uupd.service \
		/usr/lib/systemd/system/uupd-manual.service \
		/usr/lib/systemd/system/uupd.timer ||
		fail "systemd-analyze rejected the installed units"

	log "checking the timer unit"
	systemctl enable uupd.timer >/dev/null || fail "could not enable uupd.timer"
	[[ "$(systemctl is-enabled uupd.timer)" == "enabled" ]] || fail "uupd.timer is not enabled"
	systemctl disable uupd.timer >/dev/null || fail "could not disable uupd.timer"
	[[ "$(systemctl is-enabled uupd.timer)" == "disabled" ]] || fail "uupd.timer is still enabled"
	pass "the shipped units are valid and uupd.timer can be enabled and disabled"

	local booted
	booted="$(image_digest booted)"
	log "running uupd.service"
	systemctl start uupd.service || {
		journalctl -b --no-pager -u uupd.service >&2 || true
		fail "uupd.service failed"
	}
	journalctl -b --no-pager -u uupd.service | grep -q "Updates Completed Successfully" ||
		fail "uupd.service did not report a successful update"

	local staged
	staged="$(image_digest staged)"
	[[ -n "$staged" ]] || fail "uupd.service did not stage a deployment"
	[[ "$staged" != "$booted" ]] || fail "uupd.service staged the booted image"
	printf '%s\n' "$staged" >"$STAGED_DIGEST_FILE"
	pass "uupd.service staged $staged"
}

phase_systemd_verify() {
	log "=== extended: verify the uupd.service upgrade ==="
	require_root
	[[ -s "$STAGED_DIGEST_FILE" ]] || fail "$STAGED_DIGEST_FILE is missing"
	local expected booted
	expected="$(cat "$STAGED_DIGEST_FILE")"
	booted="$(image_digest booted)"
	log "expected digest: $expected"
	log "booted digest:   ${booted:-<none>}"
	[[ -n "$booted" ]] || fail "could not determine the booted image digest"
	[[ "$booted" == "$expected" ]] || fail "the service upgrade did not boot"
	[[ -f /usr/share/uupd-e2e/upgraded3 ]] || fail "the upgraded3 payload is missing"
	assert_no_staged
	expect_rc 77 "update-check reports no update after the service upgrade" uupd update-check
	pass "the systemd service staged a bootable upgrade"
}

case "${TEST_PHASE:-}" in
	phase1) phase1 ;;
	phase2) phase2 ;;
	phase_apply) phase_apply ;;
	phase_apply_verify) phase_apply_verify ;;
	phase_rollback) phase_rollback ;;
	phase_rollback_verify) phase_rollback_verify ;;
	phase_systemd) phase_systemd ;;
	phase_systemd_verify) phase_systemd_verify ;;
	*) fail "set TEST_PHASE to a known phase, got '${TEST_PHASE:-}'" ;;
esac
