package main

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"testing"
)

func TestDockerRestartBaselineAndDelta(t *testing.T) {
	a := &agent{}
	sample := func(count float64) []model.Result {
		return []model.Result{{Name: "container:0:api", State: model.OK, Values: map[string]float64{"restart_count": count}}}
	}
	first := a.withDockerRestarts(sample(4))
	if first[1].State != model.Unknown {
		t.Fatalf("first sample must be unknown: %+v", first[1])
	}
	second := a.withDockerRestarts(sample(6))
	if second[1].Values["count"] != 2 {
		t.Fatalf("restart delta: %+v", second[1])
	}
	third := a.withDockerRestarts(sample(6))
	if third[1].Values["count"] != 2 {
		t.Fatalf("24-hour count lost: %+v", third[1])
	}
}
