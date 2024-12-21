package percent_test

import (
	"math"
	"testing"

	"github.com/ublue-os/uupd/pkg/percent"
)

func InitIncrementer(max int) percent.Incrementer {
	return percent.Incrementer{MaxIncrements: max}
}

func TestOverflow(t *testing.T) {
	max := 3
	tracker := InitIncrementer(max)

	iter := 0
	for iter < max {
		tracker.IncrementSection(nil)
		iter++
	}
	// +1 so that it overflows
	tracker.IncrementSection(nil)

	if tracker.CurrentStep() > max {
		t.Fatalf("Incremented with overflow. Expected: %d, Got: %d", max, tracker.CurrentStep())
	}
}

func TestProperIncrement(t *testing.T) {
	num := math.MaxInt8
	tracker := InitIncrementer(num)

	iter := 0
	for iter < num {
		if tracker.CurrentStep() != iter {
			t.Fatalf("Misstep increment. Expected: %d, Got: %d", iter, tracker.CurrentStep())
		}
		tracker.IncrementSection(nil)
		iter++
	}
}
