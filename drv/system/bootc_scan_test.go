package system

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	appLogging "github.com/ublue-os/uupd/pkg/logging"
	"github.com/ublue-os/uupd/pkg/percent"
)

func TestBootcScanKeepsDiagnosticsOutOfMessage(t *testing.T) {
	originalLogger := slog.Default()
	slog.SetDefault(appLogging.NewMuteLogger())
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	input := `{"type":"ProgressBytes","task":"pulling","bytes":333,"bytesTotal":1000}`
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tracker := percent.NewIncrementer(false, 1)

	bootcScan(bufio.NewScanner(strings.NewReader(input)), &tracker, logger, slog.LevelDebug)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("failed to decode bootc diagnostic log: %v", err)
	}
	if message, ok := record[slog.MessageKey].(string); !ok || message != "" {
		t.Fatalf("expected an empty message, got %#v", record[slog.MessageKey])
	}
	if rawProgress, ok := record["raw_progress"].(string); !ok || rawProgress != input {
		t.Fatalf("expected raw progress in a structured attribute, got %#v", record["raw_progress"])
	}
	if _, ok := record["progress_struct"].(map[string]any); !ok {
		t.Fatalf("expected parsed progress in a structured attribute, got %#v", record["progress_struct"])
	}
}
