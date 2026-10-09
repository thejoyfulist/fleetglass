package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestSystemFieldsAndDisappearingEvents(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "system.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Now().UTC().Add(-time.Minute)
	first := model.Batch{HostID: "host", At: at, Results: []model.Result{
		{Name: "recent_errors", State: model.Warn, Detail: "journal", CadenceSeconds: 300, Values: map[string]float64{"count": 1}},
		{Name: "recent_error:0", State: model.Warn, Detail: "group", CadenceSeconds: 300, Fields: map[string]string{"unit": "unit.service", "message": "redacted"}},
	}}
	if err := s.Ingest(first); err != nil {
		t.Fatal(err)
	}
	current, err := s.Overview("host")
	if err != nil || len(current.Results) != 2 || current.Results[0].Fields["unit"] != "unit.service" {
		t.Fatalf("roundtrip: %+v, %v", current.Results, err)
	}
	second := model.Batch{HostID: "host", At: at.Add(time.Second), Results: []model.Result{{Name: "recent_errors", State: model.OK, Detail: "journal", CadenceSeconds: 300, Values: map[string]float64{"count": 0}}}}
	if err := s.Ingest(second); err != nil {
		t.Fatal(err)
	}
	current, err = s.Overview("host")
	if err != nil || len(current.Results) != 1 {
		t.Fatalf("stale event retained: %+v, %v", current.Results, err)
	}
}

func TestCompleteSetRemovesAbsentMembersImmediately(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "sets.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	start := time.Now().UTC().Add(-time.Minute)
	sets := []struct{ parent, member string }{
		{"ports:tcp", "port:tcp:ipv4:443:any"},
		{"docker:0", "container:0:web"},
		{"security:sudo", "security:sudo_user:agent"},
		{"recent_errors", "recent_error:0"},
		{"repositories:0", "repository:0:app"},
	}
	first := []model.Result{}
	second := []model.Result{}
	for _, set := range sets {
		first = append(first, model.Result{Name: set.parent, State: model.OK, CadenceSeconds: 300}, model.Result{Name: set.member, State: model.Warn, CadenceSeconds: 300})
		second = append(second, model.Result{Name: set.parent, State: model.OK, CadenceSeconds: 300})
	}
	first = append(first, model.Result{Name: "tls:example", State: model.OK, CadenceSeconds: 300})
	if e = s.Ingest(model.Batch{HostID: "host", At: start, Results: first}); e != nil {
		t.Fatal(e)
	}
	if e = s.Ingest(model.Batch{HostID: "host", At: start.Add(time.Second), Results: second}); e != nil {
		t.Fatal(e)
	}
	overview, e := s.Overview("host")
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range overview.Results {
		for _, set := range sets {
			if r.Name == set.member {
				t.Fatalf("absent member retained: %s", r.Name)
			}
		}
	}
	found := false
	for _, r := range overview.Results {
		if r.Name == "tls:example" {
			found = true
		}
	}
	if !found {
		t.Fatal("individual probe was removed")
	}
}
