package store

import (
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestObservationsProjectsStorageHeatAndInsight(t *testing.T) {
	s, e := Open(t.TempDir() + "/test.db")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	old := now.Add(-8 * 24 * time.Hour)
	batch := model.Batch{HostID: "host", At: old, Storage: []model.StorageEntry{{Root: "/tmp/root", Path: "/tmp/root/project", Depth: 1, SizeBytes: 100, Complete: true}}}
	if e = s.Ingest(batch); e != nil {
		t.Fatal(e)
	}
	for i := 0; i <= 16; i++ {
		at := now.Add(-time.Duration(16-i) * time.Minute)
		b := model.Batch{HostID: "host", At: at, Apps: []model.AppSample{{Name: "compiler.service", Kind: "unit", RatesReady: true, CPUPercent: 91, MemoryBytes: 300 * 1024 * 1024, CounterSource: "proc"}}}
		if i == 16 {
			b.Apps = append(b.Apps, model.AppSample{Name: "node.service", Kind: "unit", RatesReady: true, MemoryBytes: 100000000, Processes: []model.AppProcess{{Name: "node", PID: 321}}})
			b.Projects = []model.ProjectProcess{{PID: 321, Name: "node", Project: "project", Kind: "dev_server", CPUPercent: 0, MemoryBytes: 100000000, IdleMinutes: 181, ConnectionIdleMinutes: 181, Idle: true, Ports: []int{3000}, PortsMeasured: true, ConnectionsMeasured: true, StopCommand: "kill -TERM -- 321"}}
			b.Storage = []model.StorageEntry{{Root: "/tmp/root", Path: "/tmp/root/project", Depth: 1, SizeBytes: 300, Complete: true}}
			b.Results = []model.Result{{Name: "sensor:cpu:Package", State: model.OK, Values: map[string]float64{"celsius": 85}}}
		}
		if e = s.Ingest(b); e != nil {
			t.Fatal(e)
		}
	}
	got, e := s.Observations("host", now)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Projects) != 1 || got.Projects[0].Ports[0] != 3000 {
		t.Fatalf("projects: %+v", got.Projects)
	}
	if len(got.Storage) != 1 || got.Storage[0].Growth7D == nil || *got.Storage[0].Growth7D != 200 {
		t.Fatalf("storage: %+v", got.Storage)
	}
	if len(got.Heat) != 1 || got.Heat[0].Type != "Heat" {
		t.Fatalf("heat: %+v", got.Heat)
	}
	foundCPU, foundIdle := false, false
	for _, in := range got.Insights {
		if in.Kind == "cpu" && in.App == "compiler.service" && in.DetailCode == "cpu" {
			foundCPU = true
		}
		if in.Kind == "idle_dev_server" && in.App == "node.service" && in.AppKind == "unit" && in.DetailCode == "idle_dev_server" {
			foundIdle = true
		}
	}
	if !foundCPU || !foundIdle {
		t.Fatalf("insights: %+v", got.Insights)
	}
}
