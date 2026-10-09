package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestRetentionAndDownsampling(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC().Truncate(time.Hour)
	for _, age := range []time.Duration{2 * time.Hour, 8 * 24 * time.Hour, 100 * 24 * time.Hour} {
		b := model.Batch{HostID: "h1", At: now.Add(-age), Results: []model.Result{{Name: "cpu", State: model.OK, Detail: "busy", Values: map[string]float64{"busy_percent": 20}}}}
		if e := s.Ingest(b); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var raw int
	if e := s.DB.QueryRow(`SELECT COUNT(*) FROM samples`).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if raw != 1 {
		t.Fatalf("raw samples=%d", raw)
	}
	var hour, day int
	s.DB.QueryRow(`SELECT COUNT(*) FROM aggregates WHERE period='hour'`).Scan(&hour)
	s.DB.QueryRow(`SELECT COUNT(*) FROM aggregates WHERE period='day'`).Scan(&day)
	if hour < 1 || day < 1 {
		t.Fatalf("aggregates hour=%d day=%d", hour, day)
	}
	if e := s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var again int
	s.DB.QueryRow(`SELECT COUNT(*) FROM aggregates`).Scan(&again)
	if again != hour+day {
		t.Fatal("maintenance duplicated aggregates")
	}
}

func TestLegacyProbeCadenceMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE hosts (id TEXT PRIMARY KEY,labels TEXT NOT NULL,at_ns INTEGER NOT NULL)`,
		`CREATE TABLE samples (host_id TEXT NOT NULL, name TEXT NOT NULL, at_ns INTEGER NOT NULL, state TEXT NOT NULL, detail TEXT NOT NULL, vals TEXT NOT NULL, PRIMARY KEY(host_id,name,at_ns))`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Now().UTC().Add(-time.Minute).UnixNano()
	if _, err := db.Exec(`INSERT INTO hosts VALUES('host','{}',?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO samples VALUES('host','tls:example',?,'ok','valid','{}')`, at); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	overview, err := s.Overview("host")
	if err != nil || len(overview.Results) != 1 || overview.Results[0].CadenceSeconds != 300 || overview.Results[0].State != model.OK {
		t.Fatalf("legacy slow probe: %+v, %v", overview.Results, err)
	}
}

func TestOverviewMarksProbesStaleAndExpiresAfterOneDay(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "probes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	for _, sample := range []struct {
		name    string
		age     time.Duration
		cadence int
	}{
		{"docker:1", 46 * time.Second, 15},
		{"container:1:old", 46 * time.Second, 15},
		{"docker:2", 10 * time.Second, 15},
		{"tls:example", 14 * time.Minute, 300},
		{"port:8080:any", 16 * time.Minute, 300},
		{"docker:expired", 25 * time.Hour, 15},
	} {
		batch := model.Batch{HostID: "host", At: now.Add(-sample.age), Results: []model.Result{{Name: sample.name, State: model.OK, Detail: "healthy", CadenceSeconds: sample.cadence, Values: map[string]float64{"count": 1}}}}
		if err := s.Ingest(batch); err != nil {
			t.Fatal(err)
		}
	}
	overview, err := s.Overview("host")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]model.State{"docker:1": model.Stale, "container:1:old": model.Stale, "docker:2": model.OK, "tls:example": model.OK, "port:8080:any": model.Stale}
	if len(overview.Results) != len(want) {
		t.Fatalf("results: %+v", overview.Results)
	}
	for _, result := range overview.Results {
		if result.State != want[result.Name] || result.At.IsZero() {
			t.Fatalf("wrong state or observation time: %+v", result)
		}
	}
	series, err := s.SeriesNamed("host", "count", "24h", "docker:1")
	if err != nil || len(series) != 1 {
		t.Fatalf("stale probe history lost: %+v, %v", series, err)
	}
}
