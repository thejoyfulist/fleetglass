package api

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"testing"
	"time"
)

func TestClaudeQuotaFallback(t *testing.T) {
	now := time.Now().UTC()
	estimate := model.Quota{Kind: "claude", Window: "5h", Method: "estimated"}
	reported := model.Quota{Kind: "claude", Window: "5h", Method: "reported", ResetsAt: now.Add(time.Hour)}
	if got := withClaudeFallback([]model.Quota{reported}, []model.Quota{estimate}, now); len(got) != 1 || got[0].Method != "reported" {
		t.Fatalf("reported: %+v", got)
	}
	if got := withClaudeFallback(nil, []model.Quota{estimate}, now); len(got) != 1 || got[0].Method != "estimated" {
		t.Fatalf("fallback: %+v", got)
	}
	reported.ResetsAt = now.Add(-time.Second)
	if got := withClaudeFallback([]model.Quota{reported}, []model.Quota{estimate}, now); len(got) != 1 || got[0].Method != "estimated" {
		t.Fatalf("expired report: %+v", got)
	}
}
