package store

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestAppHistoryAndHourlyLeaders(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC().Truncate(time.Hour).Add(20 * time.Minute)
	for i := 0; i < 2; i++ {
		at := now.Add(time.Duration(i) * time.Minute)
		apps := []model.AppSample{{Name: "build.service", Kind: "unit", CPUPercent: float64(10 + i*10), MemoryBytes: 100, DiskWriteRate: 200, Processes: []model.AppProcess{{Name: "fixture", PID: 123}}}, {Name: "Container api", Kind: "container", CPUPercent: 5, MemoryBytes: 200, DiskWriteRate: 100}}
		if err := s.Ingest(model.Batch{HostID: "fixture", At: at, Apps: apps}); err != nil {
			t.Fatal(err)
		}
	}
	v, err := s.Apps("fixture", "1h", now.Add(time.Minute+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Apps) < 2 || len(v.History) != 4 || len(v.TopHours) != 1 || v.TopHours[0].CPU != "build.service" || v.TopHours[0].Memory != "Container api" {
		t.Fatalf("apps: %+v", v)
	}
	latestProcess := false
	for _, app := range v.Apps {
		if app.Name == "build.service" && len(app.Processes) == 1 && app.Processes[0].PID == 123 {
			latestProcess = true
		}
	}
	if !latestProcess || len(v.History[0].Processes) != 0 {
		t.Fatalf("process inventory should exist only on the latest sample: %+v", v)
	}
	if _, err := s.Apps("fixture", "invalid", now); err == nil {
		t.Fatal("invalid range accepted")
	}
}

func TestAppTodayIOPersistsAcrossCollectorRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apps.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	appendSample := func(counter float64, at time.Time) {
		t.Helper()
		a := model.AppSample{Name: "build.service", Kind: "unit", CounterSource: "cgroup", DiskReadCounter: counter, DiskWriteCounter: counter * 2}
		if err := s.Ingest(model.Batch{HostID: "fixture", At: at, Apps: []model.AppSample{a}}); err != nil {
			t.Fatal(err)
		}
	}
	appendSample(1000, now)
	appendSample(1500, now.Add(time.Second))
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
	v, err := s.Apps("fixture", "1h", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Apps) < 1 || v.Apps[0].DiskReadBytes != 700 || v.Apps[0].DiskWriteBytes != 1400 {
		t.Fatalf("today after restart: %+v", v.Apps)
	}
}

func TestAppSharesReconcileToHostTotals(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "reconcile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Now().UTC()
	rx := float64(50)
	tx := float64(30)
	b := model.Batch{HostID: "fixture", At: at, Results: []model.Result{
		{Name: "cpu", State: model.OK, Values: map[string]float64{"busy_percent": 50, "core_count": 2}},
		{Name: "memory", State: model.OK, Values: map[string]float64{"used_bytes": 1000}},
		{Name: "drive:one", State: model.OK, Values: map[string]float64{"read_bytes_per_second": 100, "write_bytes_per_second": 100}},
		{Name: "network:eth0", State: model.OK, Values: map[string]float64{"rx_bytes_per_second": 100, "tx_bytes_per_second": 100}},
	}, Apps: []model.AppSample{{Name: "api", Kind: "container", RatesReady: true, CPUPercent: 30, MemoryBytes: 700, DiskReadRate: 120, DiskWriteRate: 50, NetworkRXRate: &rx, NetworkTXRate: &tx}}}
	if err := s.Ingest(b); err != nil {
		t.Fatal(err)
	}
	v, err := s.Apps("fixture", "1h", at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var cpu, mem, read, write, netRX, netTX float64
	for _, a := range v.Apps {
		cpu += a.CPUPercent
		mem += a.MemoryBytes
		read += a.DiskReadRate
		write += a.DiskWriteRate
		if a.NetworkRXRate != nil {
			netRX += *a.NetworkRXRate
		}
		if a.NetworkTXRate != nil {
			netTX += *a.NetworkTXRate
		}
	}
	if math.Abs(cpu-100) > 1e-6 || math.Abs(mem-1000) > 1e-6 || math.Abs(read-100) > 1e-6 || math.Abs(write-100) > 1e-6 || math.Abs(netRX-100) > 1e-6 || math.Abs(netTX-100) > 1e-6 {
		t.Fatalf("unreconciled totals: cpu=%v memory=%v disk=%v/%v network=%v/%v apps=%+v", cpu, mem, read, write, netRX, netTX, v.Apps)
	}
}
