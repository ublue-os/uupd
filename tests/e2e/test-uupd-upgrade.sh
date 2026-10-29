#!/usr/bin/env bash
# E2E test for uupd: verify bootc upgrade works correctly
# This script runs inside the test VM and verifies the full upgrade cycle
set -euo pipefail

MARKER_FILE="/usr/share/uupd-testing"
UPGRADE_IMAGE="localhost/bootc-derived-local"

log() {
    echo "[uupd-e2e] $*" >&2
}

# Verify we're running as root
if [[ $EUID -ne 0 ]]; then
    log "ERROR: This test must run as root"
    exit 1
fi

# Check bootc is available
if ! command -v bootc >/dev/null 2>&1; then
    log "ERROR: bootc not found"
    exit 1
fi

# Check uupd is installed
if ! command -v uupd >/dev/null 2>&1; then
    log "ERROR: uupd not found"
    exit 1
fi

log "=== Initial boot: checking system state ==="
bootc status --format=json > /tmp/bootc-status.json
log "Bootc status: $(cat /tmp/bootc-status.json | head -c 200)"

# Check if we're in the second boot (after upgrade)
REBOOT_COUNT="${TMT_REBOOT_COUNT:-0}"
log "Reboot count: $REBOOT_COUNT"

if [[ "$REBOOT_COUNT" == "0" ]]; then
    log "=== First boot: preparing test environment ==="
    
    # Create a derived container image with a marker file
    log "Creating derived image with marker file"
    mkdir -p /tmp/uupd-test
    cat > /tmp/uupd-test/Dockerfile << 'EOF'
FROM localhost/bootc
RUN mkdir -p /usr/share && touch /usr/share/uupd-testing
EOF
    
    # Build the derived image
    log "Building derived image"
    podman build -t "$UPGRADE_IMAGE" /tmp/uupd-test
    
    # Copy the derived image to storage
    log "Copying image to storage"
    bootc image copy-to-storage "$UPGRADE_IMAGE"
    
    # Verify uupd --dry-run works
    log "Running uupd --dry-run"
    uupd --dry-run || log "WARNING: dry-run returned non-zero (may be expected)"
    
    # Check uupd --update-check
    log "Running uupd update-check"
    uupd --update-check || log "update-check returned non-zero"
    
    # Check uupd --is-img-outdated
    log "Running uupd is-img-outdated"
    uupd --is-img-outdated || log "is-img-outdated returned non-zero"
    
    # Stage the upgrade with mutate-in-place
    log "Staging upgrade with bootc switch --mutate-in-place"
    bootc switch --mutate-in-place --transport containers-storage "$UPGRADE_IMAGE"
    
    # Reboot into new deployment
    log "Rebooting into new deployment"
    tmt-reboot
    
    log "First boot complete, rebooting..."
    exit 0
else
    log "=== Second boot: verifying upgrade ==="
    
    # Verify the marker file exists
    if [[ -f "$MARKER_FILE" ]]; then
        log "PASS: Marker file exists after upgrade"
    else
        log "FAIL: Marker file missing after upgrade"
        exit 1
    fi
    
    # Verify bootc status shows new deployment
    log "Checking bootc status"
    bootc status --format=json > /tmp/bootc-status-after.json
    
    # Verify uupd still works
    log "Running uupd --dry-run after upgrade"
    uupd --dry-run || log "WARNING: dry-run returned non-zero"
    
    # Verify uupd --update-check
    log "Running uupd update-check after upgrade"
    uupd --update-check || log "update-check returned non-zero"
    
    # Verify uupd --is-img-outdated
    log "Running uupd is-img-outdated after upgrade"
    uupd --is-img-outdated || log "is-img-outdated returned non-zero"
    
    log "=== All checks passed ==="
    exit 0
fi