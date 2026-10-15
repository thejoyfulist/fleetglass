package forecast

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"testing"
	"time"
)

func TestQuota(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	base := model.Quota{Kind: "codex", Window: "7d", WindowMinutes: 10080, ResetsAt: now.Add(24 * time.Hour), Method: "reported"}
	q := func(hours int, used float64) model.Quota {
		v := base
		v.ObservedAt = now.Add(time.Duration(hours) * time.Hour)
		v.Used = used
		return v
	}
	for _, tc := range []struct {
		name    string
		history []model.Quota
		current model.Quota
		unknown bool
		pace    float64
	}{
		{"steady", []model.Quota{q(-2, 40), q(0, 50)}, q(0, 50), false, 5},
		{"accelerating", []model.Quota{q(-4, 30), q(-2, 34), q(0, 50)}, q(0, 50), false, 6.5},
		{"exhausted", []model.Quota{q(-2, 90), q(0, 100)}, q(0, 100), false, 5},
		{"single", []model.Quota{q(0, 50)}, q(0, 50), true, 0},
		{"past reset", []model.Quota{q(-2, 40), q(0, 50)}, func() model.Quota { v := q(0, 50); v.ResetsAt = now.Add(-time.Hour); return v }(), true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Quota(tc.current, tc.history, now)
			if tc.unknown {
				if f != nil {
					t.Fatal(f)
				}
				return
			}
			if f == nil || f.PacePPH != tc.pace || f.RunsOutAt.IsZero() {
				t.Fatalf("forecast: %+v", f)
			}
		})
	}
}
