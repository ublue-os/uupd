package rpmostree_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ublue-os/uupd/drv/generic"
	"github.com/ublue-os/uupd/drv/rpmostree"
	appLogging "github.com/ublue-os/uupd/pkg/logging"
)

func InitBaseConfig() rpmostree.RpmOstreeUpdater {
	var initConfiguration = generic.UpdaterInitConfiguration{
		DryRun:      true,
		Ci:          false,
		Verbose:     false,
		Environment: nil,
		Logger:      appLogging.NewMuteLogger(),
	}
	driv, _ := rpmostree.RpmOstreeUpdater{}.New(initConfiguration)
	return driv
}

func TestProperSteps(t *testing.T) {
	rpmostreeUpdater := InitBaseConfig()
	rpmostreeUpdater.Config.Enabled = false

	if rpmostreeUpdater.Steps() != 0 {
		t.Fatalf("Expected no steps when module is disabled")
	}

	rpmostreeUpdater.Config.Enabled = true
	if rpmostreeUpdater.Steps() == 0 {
		t.Fatalf("Expected steps to be added")
	}
}

func writeFakeScript(t *testing.T, script string) string {
	t.Helper()
	f, err := os.CreateTemp("", "fake-binary-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func writeFakeBinary(t *testing.T, output string, code int) string {
	script := "#!/bin/sh\n"
	if output != "" {
		script += fmt.Sprintf("cat <<'EOF'\n%s\nEOF\n", output)
	}
	script += fmt.Sprintf("exit %d\n", code)
	return writeFakeScript(t, script)
}

func writeFakeRpmOstreeForSkopeo(t *testing.T, checkExitCode int, statusJSON, inspectOutput, rawOutput string, skopeoExitCode int) (string, string) {
	rpmOstreeScript := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "upgrade" ]; then
    exit %d
fi
if [ "$1" = "status" ]; then
    cat <<'OSTREE_EOF'
%s
OSTREE_EOF
    exit 0
fi
exit 1
`, checkExitCode, statusJSON)

	skopeoScript := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "inspect" ] && [ "$2" = "--raw" ]; then
    cat <<'RAW_EOF'
%s
RAW_EOF
    exit %d
fi
if [ "$1" = "inspect" ]; then
    cat <<'INSPECT_EOF'
%s
INSPECT_EOF
    exit %d
fi
exit 1
`, rawOutput, skopeoExitCode, inspectOutput, skopeoExitCode)

	return writeFakeScript(t, rpmOstreeScript), writeFakeScript(t, skopeoScript)
}

func TestCheckDryRun(t *testing.T) {
	updater := InitBaseConfig()
	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error in dry-run, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected update to be available in dry-run mode")
	}
}

func TestCheckUpdatesAvailable(t *testing.T) {
	binary := writeFakeBinary(t, "", 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = binary
	updater.SkopeoPath = "/bin/true"

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected updates to be available")
	}
}

func TestCheckNoUpdatesFallbackToSkopeo(t *testing.T) {
	statusJSON := `{
    "deployments": [
        {
            "timestamp": 1700000000,
            "base-commit-meta": {"ostree.manifest-digest": "sha256:same"},
            "container-image-reference": "ostree-image-signed:docker://ghcr.io/ublue-os/test:stable"
        }
    ]
}`
	skopeoOutput := `{"Digest": "sha256:same"}`
	singleManifest := `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json"}`
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, skopeoOutput, singleManifest, 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = rpmOstreeBin
	updater.SkopeoPath = skopeoBin

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if updateAvailable {
		t.Fatalf("Expected no updates when both --check (exit77) and skopeo agree")
	}
}

func TestCheckExit77SkopeoDetectsBaseImageChange(t *testing.T) {
	statusJSON := `{
    "deployments": [
        {
            "timestamp": 1700000000,
            "base-commit-meta": {"ostree.manifest-digest": "sha256:old"},
            "container-image-reference": "ostree-image-signed:docker://ghcr.io/ublue-os/test:stable"
        }
    ]
}`
	skopeoOutput := `{"Digest": "sha256:new"}`
	singleManifest := `{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json"}`
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, skopeoOutput, singleManifest, 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = rpmOstreeBin
	updater.SkopeoPath = skopeoBin

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected updates: --check exited 77 but skopeo detected base image change")
	}
}

func TestCheckIndexDigestMatchesPlatformManifest(t *testing.T) {
	statusJSON := `{
    "deployments": [
        {
            "timestamp": 1700000000,
            "base-commit-meta": {"ostree.manifest-digest": "sha256:cf32381a509dcd446fadcbe5c0558c3021b8b90879ae0d02b22b5d0e5c2c587e"},
            "container-image-reference": "ostree-image-signed:docker://ghcr.io/ublue-os/test:stable"
        }
    ]
}`
	inspectOutput := `{"Digest": "sha256:71df44e56aaea670bd3c0e2820190d6b2b49578d6b18e11be5049ca751686997"}`
	rawIndex := `{
    "mediaType": "application/vnd.oci.image.index.v1+json",
    "manifests": [
        {"digest": "sha256:cf32381a509dcd446fadcbe5c0558c3021b8b90879ae0d02b22b5d0e5c2c587e", "platform": {"os": "linux", "architecture": "amd64"}},
        {"digest": "sha256:arm64digest", "platform": {"os": "linux", "architecture": "arm64"}},
        {"digest": "sha256:attestation", "platform": {"os": "unknown", "architecture": "unknown"}}
    ]
}`
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, inspectOutput, rawIndex, 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = rpmOstreeBin
	updater.SkopeoPath = skopeoBin

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if updateAvailable {
		t.Fatalf("Expected no updates when the platform manifest digest matches the installed digest")
	}
}

func TestCheckIndexDigestPlatformManifestChanged(t *testing.T) {
	statusJSON := `{
    "deployments": [
        {
            "timestamp": 1700000000,
            "base-commit-meta": {"ostree.manifest-digest": "sha256:old"},
            "container-image-reference": "ostree-image-signed:docker://ghcr.io/ublue-os/test:stable"
        }
    ]
}`
	inspectOutput := `{"Digest": "sha256:index"}`
	rawIndex := `{
    "mediaType": "application/vnd.oci.image.index.v1+json",
    "manifests": [
        {"digest": "sha256:new", "platform": {"os": "linux", "architecture": "amd64"}},
        {"digest": "sha256:arm64digest", "platform": {"os": "linux", "architecture": "arm64"}}
    ]
}`
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, inspectOutput, rawIndex, 0)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = rpmOstreeBin
	updater.SkopeoPath = skopeoBin

	updateAvailable, err := updater.Check()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !updateAvailable {
		t.Fatalf("Expected updates when the platform manifest digest changed")
	}
}

func TestCheckErrorReturnsTrue(t *testing.T) {
	binary := writeFakeBinary(t, "", 1)
	updater := InitBaseConfig()
	updater.Config.DryRun = false
	updater.BinaryPath = binary
	updater.SkopeoPath = "/bin/true"

	updateAvailable, err := updater.Check()
	if err == nil {
		t.Fatalf("Expected error when rpm-ostree --check fails with non-77 exit code")
	}
	if !updateAvailable {
		t.Fatalf("Expected updateAvailable=true on check error (safe default)")
	}
}

func TestUpdateErrorHintSpace(t *testing.T) {
	externalOutput := `ERROR: no space left on device while writing to /var/cache/dnf
    Error summary: Command completed with status 1
    Status: error: rpm-ostree failed: unknown
    Writing /etc/yum.repos.d/50-ci-repositories.repo
    Cleaning up /var/cache/dnf
`
	hint := rpmostree.ClassifyUpdateError(externalOutput)
	if hint != "Ran out of disk space during the update. Free up storage and try again." {
		t.Fatalf("Expected storage hint, got: %s", hint)
	}
}

func TestUpdateErrorHintNetwork(t *testing.T) {
	externalOutput := `ERROR: failed to fetch https://example.com/repo/data.tar.xz: Connection timed out
    Error summary: Command completed with status 1
    Status: error: rpm-ostree failed: unknown
`
	hint := rpmostree.ClassifyUpdateError(externalOutput)
	if hint != "Failed to fetch update data. Check your network connection and try again." {
		t.Fatalf("Expected network hint, got: %s", hint)
	}
}

func TestUpdateErrorHintChecksumPriority(t *testing.T) {
	externalOutput := `ERROR: failed to fetch: invalid checksum
    Error summary: Command completed with status 1
    Status: error: rpm-ostree failed: unknown
`
	hint := rpmostree.ClassifyUpdateError(externalOutput)
	if hint != "Invalid checksum while downloading. The update data may be corrupted; try again." {
		t.Fatalf("Expected checksum hint, got: %s", hint)
	}
}

func TestUpdateErrorHintNotFound(t *testing.T) {
	externalOutput := `ERROR: something else happened
    Error summary: Command completed with status 1
    Status: error: rpm-ostree failed: unknown
`
	hint := rpmostree.ClassifyUpdateError(externalOutput)
	if hint != "" {
		t.Fatalf("Expected no hint for unknown error, got: %s", hint)
	}
}
