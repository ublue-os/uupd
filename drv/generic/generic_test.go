package generic_test

import (
	"log"
	"testing"

	"github.com/ublue-os/uupd/drv/generic"
)

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
