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

func recordOutputs(records []map[string]any) map[string]bool {
	outputs := map[string]bool{}
	for _, record := range records {
		if output, ok := record["output"].(string); ok {
			outputs[output] = true
		}
	}
	return outputs
}

func bootcEventStream() string {
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
	return input.String()
}

// A high-frequency bootc stream used to produce one status and one raw record
// per event; only transitions and occasional updates should be logged now.
func TestBootcScanRateLimitsProgressRecords(t *testing.T) {
	var statusBuf, outputBuf bytes.Buffer
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&statusBuf, nil)))
	logger := slog.New(slog.NewJSONHandler(&outputBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tracker := percent.NewIncrementer(false, 3)
	bootcScan(bufio.NewScanner(strings.NewReader(bootcEventStream())), &tracker, logger, slog.LevelDebug)

	statusRecords := decodeScanRecords(t, statusBuf.Bytes())
	outputRecords := decodeScanRecords(t, outputBuf.Bytes())

	updating := countMsg(statusRecords, "Updating")
	if updating < 3 || updating > 10 {
		t.Fatalf("Expected a handful of Updating records for 500 events, got %d: %v", updating, statusRecords)
	}
	if got := countMsg(outputRecords, "Command output"); got != updating+1 {
		t.Fatalf("Expected %d Command output records, got %d", updating+1, got)
	}

	statuses := recordOutputs(statusRecords)
	for _, stage := range []string{"Updating System (Downloading)", "Updating System (Importing)", "Updating System (Deploying)"} {
		if !statuses[stage] {
			t.Fatalf("Expected a record for stage %q, got %v", stage, statuses)
		}
	}
	if !recordOutputs(outputRecords)["bootc: non-progress diagnostic line"] {
		t.Fatalf("Non-progress bootc output should still be logged, got %v", outputRecords)
	}
}
