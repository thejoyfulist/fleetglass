package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestSeriesNamedDoesNotMixProbes(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "named.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.Ingest(model.Batch{HostID: "h", At: time.Now().UTC(), Results: []model.Result{
		{Name: "memory", State: model.OK, Values: map[string]float64{"used_bytes": 10}},
		{Name: "disk:/", State: model.OK, Values: map[string]float64{"used_bytes": 100}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	points, err := s.SeriesNamed("h", "used_bytes", "24h", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Value != 10 {
		t.Fatalf("mixed series: %+v", points)
	}
}
