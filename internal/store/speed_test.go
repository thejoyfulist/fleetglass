package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestSpeedAndSubscriptionKeepMissingMeasurementsUnknown(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "speed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	session := strings.Repeat("a", 64)
	usage := []model.AIUsage{}
	for i, v := range []float64{10, 20, 40} {
		usage = append(usage, model.AIUsage{ID: strings.Repeat(string(rune('b'+i)), 64), Session: session, Kind: "codex", Model: "gpt-6-sol", At: now.Add(-time.Duration(3-i) * time.Minute), Source: "otlp", Tokens: model.Tokens{Output: 100}, OutputTokensPerSecond: v, SpeedMethod: "otlp", RequestDurationMS: 1000})
	}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: session, Kind: "codex", At: now, State: "working", Source: "otlp"}}, Usage: usage}}); err != nil {
		t.Fatal(err)
	}
	speed, err := s.Speed("host", "1h", session)
	if err != nil || len(speed) != 1 || speed[0].MedianTokensPerSecond == nil || *speed[0].MedianTokensPerSecond != 20 || speed[0].P95TokensPerSecond == nil || *speed[0].P95TokensPerSecond != 40 || speed[0].MedianFirstTokenMS != nil {
		t.Fatalf("speed %+v %v", speed, err)
	}
	comparisons, err := s.SubscriptionComparisons("host", now, nil, nil)
	if err != nil || len(comparisons) != 2 || comparisons[1].SubscriptionUSD != nil {
		t.Fatalf("subscriptions %+v %v", comparisons, err)
	}
}
