package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestDriveWrittenTodaySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drives.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	appendSample := func(write float64, at time.Time) {
		t.Helper()
		b := model.Batch{HostID: "fixture", At: at, Results: []model.Result{{Name: "drive:nvme0n1", State: model.OK, Values: map[string]float64{"read_bytes": write, "write_bytes": write}}}}
		if err := s.Ingest(b); err != nil {
			t.Fatal(err)
		}
	}
	appendSample(1000, now)
	appendSample(1500, now.Add(time.Second))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	appendSample(1700, now.Add(2*time.Second))
	o, err := s.Overview("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Results) != 1 || o.Results[0].Values["written_today_bytes"] != 700 {
		t.Fatalf("drive today: %+v", o.Results)
	}
}
