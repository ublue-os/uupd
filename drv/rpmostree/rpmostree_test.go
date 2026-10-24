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

func writeFakeBinary(t *testing.T, output string, code int) string {
	t.Helper()
	f, err := os.CreateTemp("", "fake-binary-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	script := "#!/bin/sh\n"
	if output != "" {
		script += fmt.Sprintf("cat <<'EOF'\n%s\nEOF\n", output)
	}
	script += fmt.Sprintf("exit %d\n", code)

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

func writeFakeRpmOstreeForSkopeo(t *testing.T, checkExitCode int, statusJSON, skopeoDigest string, skopeoExitCode int) (string, string) {
	t.Helper()
	rpmOstreeBin, err := os.CreateTemp("", "fake-rpm-ostree-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(rpmOstreeBin.Name()) })

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

	if _, err := rpmOstreeBin.WriteString(rpmOstreeScript); err != nil {
		t.Fatal(err)
	}
	if err := rpmOstreeBin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rpmOstreeBin.Name(), 0755); err != nil {
		t.Fatal(err)
	}

	skopeoBin, err := os.CreateTemp("", "fake-skopeo-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(skopeoBin.Name()) })

	skopeoScript := "#!/bin/sh\n"
	if skopeoDigest != "" {
		skopeoScript += fmt.Sprintf("cat <<'SKOPEO_EOF'\n%s\nSKOPEO_EOF\n", skopeoDigest)
	}
	skopeoScript += fmt.Sprintf("exit %d\n", skopeoExitCode)

	if _, err := skopeoBin.WriteString(skopeoScript); err != nil {
		t.Fatal(err)
	}
	if err := skopeoBin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(skopeoBin.Name(), 0755); err != nil {
		t.Fatal(err)
	}

	return rpmOstreeBin.Name(), skopeoBin.Name()
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
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, skopeoOutput, 0)
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
	rpmOstreeBin, skopeoBin := writeFakeRpmOstreeForSkopeo(t, 77, statusJSON, skopeoOutput, 0)
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
