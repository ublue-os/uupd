package generic_test

import (
	"log"
	"testing"
	"time"

	"github.com/ublue-os/uupd/drv/generic"
)

func TestOutdatedCutoff(t *testing.T) {
	month := time.Now().AddDate(0, -1, 0)
	for _, days := range []int{0, -5} {
		cutoff := generic.OutdatedCutoff(days)
		if diff := cutoff.Sub(month); diff > time.Minute || diff < -time.Minute {
			t.Fatalf("days %d should fall back to a calendar month, got %v", days, cutoff)
		}
	}

	days := generic.OutdatedCutoff(10)
	want := time.Now().AddDate(0, 0, -10)
	if diff := days.Sub(want); diff > time.Minute || diff < -time.Minute {
		t.Fatalf("10 days gave unexpected cutoff %v", days)
	}
}

func TestProperEnvironment(t *testing.T) {
	valuesExpected := map[string]string{
		"SIGMA":   "true",
		"CHUD":    "false",
		"AMOGUS":  "sus",
		"NOTHING": "",
	}

	for key, value := range valuesExpected {
		testmap := generic.GetEnvironment([]string{key + "=" + value})
		valuegot, exists := testmap[key]
		if !exists {
			log.Fatalf("Could not get environment variable at all: %s", key)
		}
		if valuegot != value {
			log.Fatalf("Value gotten from variable was not expected: Got %s, Expected %s", valuegot, value)
		}
	}
}
