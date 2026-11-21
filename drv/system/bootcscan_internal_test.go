package system

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/ublue-os/uupd/pkg/percent"
)

func decodeScanRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatalf("Failed decoding log record: %v", err)
		}
		records = append(records, record)
	}
	return records
}

func countMsg(records []map[string]any, msg string) int {
	count := 0
	for _, record := range records {
		if record["msg"] == msg {
			count++
		}
	}
	return count
}

// bootc emits a progress event per downloaded chunk. In the non-TUI path each
// event used to produce both an INFO "Updating" record and a raw payload
// record. This test feeds a few hundred events through bootcScan and checks
// that only stage transitions and occasional updates are logged.
func TestBootcScanRateLimitsProgressRecords(t *testing.T) {
	var statusBuf, outputBuf bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&statusBuf, nil)))
	logger := slog.New(slog.NewJSONHandler(&outputBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var input strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&input, `{"type":"ProgressBytes","task":"pulling","bytes":%d,"bytesTotal":300}`+"\n", i)
	}
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&input, `{"type":"ProgressSteps","task":"importing","steps":%d,"stepsTotal":100}`+"\n", i)
	}
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&input, `{"type":"ProgressSteps","task":"staging","steps":%d,"stepsTotal":100}`+"\n", i)
	}
	input.WriteString("bootc: non-progress diagnostic line\n")

	scanner := bufio.NewScanner(strings.NewReader(input.String()))
	tracker := percent.NewIncrementer(false, 3)
	bootcScan(scanner, &tracker, logger, slog.LevelDebug)

	statusRecords := decodeScanRecords(t, statusBuf.Bytes())
	outputRecords := decodeScanRecords(t, outputBuf.Bytes())

	updating := countMsg(statusRecords, "Updating")
	commandOutput := countMsg(outputRecords, "Command output")
	if updating < 3 || updating > 10 {
		t.Fatalf("Expected a handful of Updating records for 500 events, got %d: %v", updating, statusRecords)
	}
	// One raw record per logged status, plus the non-progress diagnostic line.
	if commandOutput != updating+1 {
		t.Fatalf("Expected %d Command output records, got %d", updating+1, commandOutput)
	}

	stages := map[string]bool{}
	for _, record := range statusRecords {
		if output, ok := record["output"].(string); ok {
			stages[output] = true
		}
	}
	for _, stage := range []string{"Updating System (Downloading)", "Updating System (Importing)", "Updating System (Deploying)"} {
		if !stages[stage] {
			t.Fatalf("Expected a record for stage %q, got %v", stage, stages)
		}
	}

	foundDiagnostic := false
	for _, record := range outputRecords {
		if record["output"] == "bootc: non-progress diagnostic line" {
			foundDiagnostic = true
		}
	}
	if !foundDiagnostic {
		t.Fatalf("Non-progress bootc output should still be logged, got %v", outputRecords)
	}
}
