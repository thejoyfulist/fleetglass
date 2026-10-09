package store

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
)

func TestChangesAndEnergyPersistDeduplicateAndSplitUTC(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	at := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 5, 0, time.UTC)
	event := probe.NewChange("Marker", "guard", "guard", at)
	b := model.Batch{HostID: "h", At: at, Changes: []model.Change{event}, Results: []model.Result{{Name: "cpu_package_power", State: model.OK, Values: map[string]float64{"watts": 360, "energy_kwh": 0.001, "energy_seconds": 10}}}}
	for i := 0; i < 2; i++ {
		if e = s.Ingest(b); e != nil {
			t.Fatal(e)
		}
	}
	rows, e := s.Changes("h", "30d", "")
	if e != nil || len(rows) != 1 || rows[0].Text != "guard" {
		t.Fatal(rows, e)
	}
	energy, e := s.Energy("h", at, 0.2516, "EUR")
	if e != nil {
		t.Fatal(e)
	}
	expectedMonth := 0.001
	if at.Day() == 1 {
		expectedMonth = 0.0005
	}
	if math.Abs(energy.TodayKWh-0.0005) > 1e-9 || math.Abs(energy.MonthKWh-expectedMonth) > 1e-9 || energy.Currency != "EUR" {
		t.Fatal(energy)
	}
	var n int
	if e = s.DB.QueryRow(`SELECT COUNT(*) FROM energy_intervals WHERE host_id='h'`).Scan(&n); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	if _, e = s.Changes("h", "30d", "invalid"); e == nil {
		t.Fatal("accepted invalid type")
	}
}

func TestStoredPresentationCodesReachOverviewAndChanges(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "presentation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Now().UTC()
	batch := model.Batch{HostID: "h", At: at, Results: []model.Result{
		{Name: "tls:example.test", State: model.Warn, Detail: "expires in 20.7 days", Values: map[string]float64{"days_remaining": 20.7}},
		{Name: "systemd:caddy", State: model.OK, Detail: "active (running)"},
	}, Changes: []model.Change{probe.NewChange("Service", "caddy.service", "caddy.service restarted", at)}}
	if err := s.Ingest(batch); err != nil {
		t.Fatal(err)
	}
	o, err := s.Overview("h")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]string{}
	for _, r := range o.Results {
		codes[r.Name] = r.DetailCode
	}
	if codes["tls:example.test"] != "certificate_expires" || codes["systemd:caddy"] != "systemd_active" {
		t.Fatalf("overview codes: %+v", codes)
	}
	changes, err := s.Changes("h", "24h", "")
	if err != nil || len(changes) != 1 || changes[0].ChangeCode != "service_restarted" || changes[0].ChangeParams["name"] != "caddy.service" {
		t.Fatalf("change codes: %+v, %v", changes, err)
	}
}

func TestContainerImageChangeSurvivesAgentRestart(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	for i, hash := range []string{"old", "new", "new"} {
		b := model.Batch{HostID: "h", At: now.Add(time.Duration(i) * time.Second), Results: []model.Result{{Name: "container:0:web", State: model.OK, Fields: map[string]string{"image_hash": hash}}}}
		if e = s.Ingest(b); e != nil {
			t.Fatal(e)
		}
	}
	changes, e := s.Changes("h", "24h", "Container")
	if e != nil || len(changes) != 1 || changes[0].Text != "Image changed: 0:web" {
		t.Fatal(changes, e)
	}
}

func TestChangeRetentionIsOneYear(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	for _, row := range []struct {
		id string
		at time.Time
	}{{"old", now.AddDate(-1, 0, -1)}, {"recent", now.Add(-24 * time.Hour)}} {
		if _, e = s.DB.Exec(`INSERT INTO changes(host_id,id,at_ns,kind,text) VALUES(?,?,?,?,?)`, "h", row.id, row.at.UnixNano(), "Boot", "Machine restarted"); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = s.DB.QueryRow(`SELECT COUNT(*) FROM changes WHERE host_id='h'`).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}
