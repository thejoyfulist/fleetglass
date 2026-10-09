package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestTrafficTotalsCountCounterChangesOnce(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "traffic.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC().Truncate(time.Second)
	for i, rx := range []float64{1000, 1500, 2000} {
		at := now.Add(time.Duration(i) * time.Second)
		b := model.Batch{HostID: "fixture", At: at, Results: []model.Result{{Name: "network:eth0", State: model.OK, Values: map[string]float64{"rx_bytes": rx, "tx_bytes": rx * 2}}}}
		if err := s.Ingest(b); err != nil {
			t.Fatal(err)
		}
		if err := s.Ingest(b); err != nil {
			t.Fatal(err)
		}
	}
	totals, err := s.Traffic("fixture", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].TodayRX != 1000 || totals[0].TodayTX != 2000 {
		t.Fatalf("traffic: %+v", totals)
	}
}
