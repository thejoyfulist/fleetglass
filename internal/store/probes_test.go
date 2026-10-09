package store

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestProbeAvailabilityAndPublicProjection(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "probes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, v := range []struct {
		name   string
		public bool
		minute int
		state  model.State
	}{{"public", true, -3, model.OK}, {"public", true, -2, model.Crit}, {"secret", false, -1, model.OK}} {
		at := now.Add(time.Duration(v.minute) * time.Minute)
		sample := model.ProbeSample{Name: v.name, DisplayName: v.name, Kind: "http", Public: v.public, State: v.state, IntervalSeconds: 60, At: at, LatencyMS: 42}
		if err := s.Ingest(model.Batch{HostID: "host-private", At: at, Probes: []model.ProbeSample{sample}}); err != nil {
			t.Fatal(err)
		}
	}
	probes, err := s.Probes("host-private", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) != 2 {
		t.Fatalf("probes: %+v", probes)
	}
	var p ProbeSummary
	for _, v := range probes {
		if v.Name == "public" {
			p = v
		}
	}
	want := 50.0 // one known up minute and one known down minute
	if p.Uptime24H == nil || math.Abs(*p.Uptime24H-want) > .001 {
		t.Fatalf("uptime %v want %v", p.Uptime24H, want)
	}
	if math.Abs(p.Coverage24H-2.0/1440) > 1e-9 || math.Abs(p.Coverage30D-2.0/(30*1440)) > 1e-9 {
		t.Fatalf("coverage 24h=%v 30d=%v", p.Coverage24H, p.Coverage30D)
	}
	if p.State != model.Unknown {
		t.Fatalf("stale state: %s", p.State)
	}
	if p.Days[len(p.Days)-1].State != model.Crit || p.Days[len(p.Days)-2].State != model.Unknown {
		t.Fatalf("daily states: %+v", p.Days[len(p.Days)-2:])
	}
	if math.Abs(p.Days[len(p.Days)-1].Coverage-2.0/720) > 1e-9 {
		t.Fatalf("partial current day coverage: %+v", p.Days[len(p.Days)-1])
	}
	public, err := s.PublicStatus(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 1 || public[0].DisplayName != "public" {
		t.Fatalf("public projection: %+v", public)
	}
	if public[0].Coverage30D != p.Coverage30D {
		t.Fatalf("public coverage: %+v", public[0])
	}
	empty := []string{}
	if err := s.Ingest(model.Batch{HostID: "host-private", At: now.Add(time.Second), ProbeNames: &empty}); err != nil {
		t.Fatal(err)
	}
	public, err = s.PublicStatus(now.Add(time.Second))
	if err != nil || len(public) != 0 {
		t.Fatalf("removed public probe remains: %+v %v", public, err)
	}
}

func TestAvailabilityKnownTimeAndGaps(t *testing.T) {
	day := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		samples  []storedProbe
		end      time.Time
		want     *float64
		coverage float64
	}{
		{"no measurements", nil, day.Add(24 * time.Hour), nil, 0},
		{"first minutes", []storedProbe{{at: day, state: model.OK, interval: time.Minute}, {at: day.Add(time.Minute), state: model.OK, interval: time.Minute}}, day.Add(24 * time.Hour), floatPtr(100), 2.0 / 1440},
		{"agent gap", []storedProbe{{at: day, state: model.OK, interval: time.Minute}, {at: day.Add(12 * time.Hour), state: model.OK, interval: time.Minute}}, day.Add(24 * time.Hour), floatPtr(100), 2.0 / 1440},
		{"brief outage", []storedProbe{{at: day, state: model.OK, interval: time.Hour}, {at: day.Add(time.Hour), state: model.Crit, interval: time.Minute}, {at: day.Add(time.Hour + time.Minute), state: model.OK, interval: time.Hour}}, day.Add(24 * time.Hour), floatPtr(100 * 120.0 / 121), 121.0 / 1440},
		{"partial current day", []storedProbe{{at: day, state: model.OK, interval: time.Minute}}, day.Add(2 * time.Minute), floatPtr(100), .5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, coverage := availability(tc.samples, day, tc.end)
			if (got == nil) != (tc.want == nil) || got != nil && math.Abs(*got-*tc.want) > 1e-9 || math.Abs(coverage-tc.coverage) > 1e-9 {
				t.Fatalf("availability=%v coverage=%v; want %v and %v", got, coverage, tc.want, tc.coverage)
			}
		})
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestPartialCurrentDayIsOperational(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "partial.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 10, 8, 12, 2, 0, 0, time.UTC)
	for _, at := range []time.Time{now.Add(-2 * time.Minute), now.Add(-time.Minute)} {
		sample := model.ProbeSample{Name: "site", DisplayName: "Site", Kind: "http", State: model.OK, IntervalSeconds: 60, At: at}
		if err := s.Ingest(model.Batch{HostID: "host", At: at, Probes: []model.ProbeSample{sample}}); err != nil {
			t.Fatal(err)
		}
	}
	probes, err := s.Probes("host", now)
	if err != nil {
		t.Fatal(err)
	}
	today := probes[0].Days[29]
	if today.State != model.OK || today.Uptime == nil || *today.Uptime != 100 || today.Coverage <= 0 || today.Coverage >= 1 {
		t.Fatalf("partial current day: %+v", today)
	}
	if probes[0].Days[28].State != model.Unknown || probes[0].Days[28].Uptime != nil {
		t.Fatalf("unmeasured prior day: %+v", probes[0].Days[28])
	}
}
