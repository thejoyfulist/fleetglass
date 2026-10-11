package store

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestAIDedupAndRetention(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "x.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	id := strings.Repeat("a", 64)
	usageID := strings.Repeat("b", 64)
	now := time.Now().UTC()
	old := now.Add(-8 * 24 * time.Hour)
	logBatch := model.Batch{HostID: "host", At: old, AI: &model.AI{Sessions: []model.AgentSession{{ID: id, Kind: "claude", At: old, Source: "log"}}, Usage: []model.AIUsage{{ID: usageID, Session: id, Kind: "claude", Model: "claude-sonnet-5", Project: "demo", At: old, Source: "log", Tokens: model.Tokens{Input: 10}}}}}
	if e = s.Ingest(logBatch); e != nil {
		t.Fatal(e)
	}
	otlpBatch := model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: id, Kind: "claude", At: now, Source: "otlp"}}, Usage: []model.AIUsage{{ID: strings.Repeat("c", 64), Session: id, Kind: "claude", Model: "claude-sonnet-5", Project: "demo", At: now, Source: "otlp", Tokens: model.Tokens{Input: 20}}}}}
	if e = s.Ingest(otlpBatch); e != nil {
		t.Fatal(e)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage`).Scan(&n)
	if n != 1 {
		t.Fatalf("dedup count=%d", n)
	}
	if e = s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	costs, e := s.Costs("host", "24h", nil, nil)
	if e != nil || len(costs) == 0 {
		t.Fatalf("costs %v %v", costs, e)
	}
}

func TestCostsByModelAndSessionSummary(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "costs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	usage := []model.AIUsage{
		{ID: strings.Repeat("b", 64), Session: id, Kind: "codex", Model: "priced-a", Project: "atlas", At: now, Source: "otlp", Tokens: model.Tokens{Input: 1000000}, OutputTokensPerSecond: 12, SpeedMethod: "otlp"},
		{ID: strings.Repeat("c", 64), Session: id, Kind: "codex", Model: "priced-b", Project: "atlas", At: now, Source: "otlp", Tokens: model.Tokens{Input: 2000000}},
		{ID: strings.Repeat("d", 64), Session: id, Kind: "codex", Model: "unpriced", Project: "atlas", At: now, Source: "otlp", Tokens: model.Tokens{Input: 100}},
	}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: id, Kind: "codex", Project: "atlas", Worktree: "/tmp/atlas", At: now, Source: "otlp"}}, Usage: usage}}); err != nil {
		t.Fatal(err)
	}
	prices := map[string]model.AIPrice{"priced-a": {Input: 2}, "priced-b": {Input: 3}}
	rows, err := s.CostsBy("host", "24h", "model", prices, nil)
	if err != nil || len(rows) != 3 {
		t.Fatalf("model costs: %+v %v", rows, err)
	}
	amounts := map[string]float64{}
	for _, row := range rows {
		if row["method"] == "api_equivalent" {
			amounts[row["model"].(string)] = row["usd"].(float64)
		}
	}
	if amounts["priced-a"] != 2 || amounts["priced-b"] != 6 || amounts["unpriced"] != 0 {
		t.Fatalf("amounts: %+v", amounts)
	}
	sessions, err := s.AgentsWithSummary("host", prices)
	if err != nil || len(sessions) != 1 || sessions[0].APIEquivalentUSD != 8 || !sessions[0].UsageMeasured || sessions[0].SpeedTokensPerSecond == nil || *sessions[0].SpeedTokensPerSecond != 12 || sessions[0].Label != "codex · atlas · aaaaaaaa" {
		t.Fatalf("sessions: %+v %v", sessions, err)
	}
	if _, err := s.CostsBy("host", "24h", "invalid", prices, nil); err == nil {
		t.Fatal("invalid grouping accepted")
	}
	if label := SessionLabel(model.AgentSession{ID: id, Kind: "codex", Project: "atlas", Worktree: "/tmp/orbit", Branch: "feature/preview"}); label != "codex · orbit · feature/preview · aaaaaaaa" {
		t.Fatalf("metadata label: %s", label)
	}
	if label := SessionLabel(model.AgentSession{ID: "0b82eee0abcd", Kind: "claude"}); label != "claude · 0b82eee0" {
		t.Fatalf("unknown location label: %s", label)
	}
	if label := SessionLabel(model.AgentSession{ID: "0b82eee0abcd", Kind: "claude", Worktree: "/", Project: "atlas"}); label != "claude · atlas · 0b82eee0" {
		t.Fatalf("project fallback label: %s", label)
	}
}

func TestAgentsSummaryUsesSessionIndex(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "agents-index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var indexed int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_index_info('ai_usage_session') WHERE name IN ('host_id','session_id')`).Scan(&indexed); err != nil || indexed != 2 {
		t.Fatalf("missing session index: count=%d err=%v", indexed, err)
	}
	var id, parent, unused string
	var detail string
	if err := s.DB.QueryRow(`EXPLAIN QUERY PLAN SELECT data FROM ai_usage WHERE host_id=? AND session_id IN (?)`, "host", strings.Repeat("a", 64)).Scan(&id, &parent, &unused, &detail); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail, "ai_usage_session (host_id=? AND session_id=?)") {
		t.Fatalf("query reads outside listed sessions: %s", detail)
	}
	now := time.Now().UTC()
	listed := strings.Repeat("a", 64)
	unlisted := strings.Repeat("b", 64)
	usage := []model.AIUsage{
		{ID: strings.Repeat("c", 64), Session: listed, Kind: "codex", Model: "priced", At: now, Source: "otlp", Tokens: model.Tokens{Input: 1000000}},
		{ID: strings.Repeat("d", 64), Session: unlisted, Kind: "codex", Model: "priced", At: now, Source: "otlp", Tokens: model.Tokens{Input: 9000000}},
	}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: listed, Kind: "codex", At: now, Source: "otlp"}}, Usage: usage}}); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.AgentsWithSummary("host", map[string]model.AIPrice{"priced": {Input: 1}})
	if err != nil || len(sessions) != 1 || sessions[0].APIEquivalentUSD != 1 {
		t.Fatalf("bounded summary: %+v %v", sessions, err)
	}
}

func TestAgentBranchFromCachedWorktree(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "branch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	root := filepath.Join(t.TempDir(), "trees")
	tree := filepath.Join(root, "demo", "feature")
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: id, Kind: "codex", Worktree: tree, At: now, Source: "otlp"}}}, Worktrees: []model.WorktreeEntry{{Root: root, Project: "demo", Name: "feature", Path: tree, Branch: "feature/preview"}}}); err != nil {
		t.Fatal(err)
	}
	sessions, err := s.AgentsWithSummary("host", nil)
	if err != nil || len(sessions) != 1 || sessions[0].Branch != "feature/preview" || !strings.Contains(sessions[0].Label, "feature/preview") {
		t.Fatalf("session branch: %+v %v", sessions, err)
	}
	detail, err := s.SessionDetail("host", id, nil)
	if err != nil || detail.Session.Branch != "feature/preview" || detail.Session.Label != sessions[0].Label {
		t.Fatalf("detail branch: %+v %v", detail.Session, err)
	}
}

func TestProcessSessionSnapshot(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "process.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	process := model.AgentSession{ID: strings.Repeat("a", 64), Kind: "codex", Source: "process", PID: 42, At: now.Add(-48 * time.Hour), State: "running"}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{process}}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Agents("host")
	if err != nil || len(got) != 1 || got[0].Source != "process" {
		t.Fatalf("process session: %+v, %v", got, err)
	}
	logSession := model.AgentSession{ID: strings.Repeat("b", 64), Kind: "codex", Source: "log", PID: 42, At: now, State: "working"}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{logSession}}}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Agents("host")
	if err != nil || len(got) != 1 || got[0].Source != "log" {
		t.Fatalf("stale process session: %+v, %v", got, err)
	}
}

func TestEndedSessionLeavesAgentsAfterOneDay(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "ended.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	for i, age := range []time.Duration{time.Hour, 25 * time.Hour} {
		at := now.Add(-age)
		id := strings.Repeat(string(rune('a'+i)), 64)
		session := model.AgentSession{ID: id, Kind: "claude", At: at, State: "ended", Source: "log"}
		if err := s.Ingest(model.Batch{HostID: "host", At: at, AI: &model.AI{Sessions: []model.AgentSession{session}}}); err != nil {
			t.Fatal(err)
		}
	}
	agents, err := s.Agents("host")
	if err != nil || len(agents) != 1 || agents[0].State != "ended" {
		t.Fatalf("visible sessions: %+v, %v", agents, err)
	}
}

func TestUnreportedSessionEndsAsTimePasses(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "aging.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	at := now.Add(-29 * time.Minute)
	session := model.AgentSession{ID: strings.Repeat("a", 64), Kind: "claude", At: at, State: "unknown", Reason: "no recent activity", Source: "log"}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{session}}}); err != nil {
		t.Fatal(err)
	}
	before, err := s.agentsAt("host", now)
	if err != nil || len(before) != 1 || before[0].State != "unknown" {
		t.Fatalf("before threshold: %+v, %v", before, err)
	}
	after, err := s.agentsAt("host", now.Add(2*time.Minute))
	if err != nil || len(after) != 1 || after[0].State != "ended" || after[0].Reason != "no process matched; no activity for over 30 minutes" {
		t.Fatalf("after threshold without another report: %+v, %v", after, err)
	}
	var persisted string
	if err := s.DB.QueryRow(`SELECT data FROM ai_sessions WHERE host_id='host'`).Scan(&persisted); err != nil || !strings.Contains(persisted, `"state":"unknown"`) {
		t.Fatalf("read changed stored history: %v, %s", err, persisted)
	}
}

func TestStaleWorktreeIsNotAnEndedSession(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "worktree.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	worktree := model.AgentSession{ID: strings.Repeat("a", 64), Kind: "unknown", At: now.Add(-time.Hour), State: "stale-worktree", Source: "worktree"}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{worktree}}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.agentsAt("host", now)
	if err != nil || len(got) != 1 || got[0].State != "stale-worktree" {
		t.Fatalf("worktree state: %+v, %v", got, err)
	}
}

func TestAIHourlyRetention(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "x.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	at := now.Add(-8 * 24 * time.Hour)
	id := strings.Repeat("a", 64)
	b := model.Batch{HostID: "host", At: at, AI: &model.AI{Usage: []model.AIUsage{{ID: strings.Repeat("b", 64), Session: id, Kind: "claude", Project: "demo", At: at, Source: "log", Tokens: model.Tokens{Input: 13}}}}}
	if e = s.Ingest(b); e != nil {
		t.Fatal(e)
	}
	if e = s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var raw, agg int
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage`).Scan(&raw)
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage_hourly`).Scan(&agg)
	if raw != 0 || agg != 1 {
		t.Fatalf("raw=%d agg=%d", raw, agg)
	}
	rows, e := s.Usage("host", "30d", "project")
	if e != nil || len(rows) != 1 {
		t.Fatalf("usage=%v err=%v", rows, e)
	}
}

func TestCostMethods(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "cost.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	session := strings.Repeat("a", 64)
	u := model.AIUsage{ID: strings.Repeat("b", 64), Session: session, Kind: "claude", Model: "claude-sonnet-5", Project: "demo", At: now, Source: "otlp", Tokens: model.Tokens{Input: 1000000}, CostUSD: 2, Method: "cli_reported"}
	if e = s.Ingest(model.Batch{HostID: "h", At: now, AI: &model.AI{Usage: []model.AIUsage{u}}}); e != nil {
		t.Fatal(e)
	}
	rows, e := s.Costs("h", "30d", nil, []model.AISubscription{{Kind: "claude", MonthlyUSD: 30}})
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r["method"].(string)] = true
		if r["method"] == "api_equivalent" && (r["usd"] != float64(2) || r["partial"] != false || r["unpriced_tokens"] != int64(0) || len(r["unpriced_models"].([]string)) != 0) {
			t.Fatalf("priced cost coverage: %+v", r)
		}
	}
	for _, method := range []string{"cli_reported", "api_equivalent", "subscription_allocated"} {
		if !seen[method] {
			t.Fatalf("missing %s: %+v", method, rows)
		}
	}
}

func TestCostPricingCoverageAndModelMatching(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "coverage.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	session := strings.Repeat("a", 64)
	usage := []model.AIUsage{
		{ID: strings.Repeat("b", 64), Session: session, Kind: "claude", Project: "mixed", Model: "claude-opus-5-5-20261001", At: now, Source: "otlp", Tokens: model.Tokens{Input: 1_000_000}},
		{ID: strings.Repeat("c", 64), Session: session, Kind: "claude", Project: "mixed", Model: "claude-opus-5-5x", At: now, Source: "otlp", Tokens: model.Tokens{Output: 20}},
		{ID: strings.Repeat("d", 64), Session: session, Kind: "claude", Project: "mixed", Model: "unknown-b", At: now, Source: "otlp", Tokens: model.Tokens{Input: 3, CacheRead: 4}},
		{ID: strings.Repeat("e", 64), Session: session, Kind: "claude", Project: "other", Model: "unknown-a", At: now, Source: "otlp", Tokens: model.Tokens{Output: 6}},
		{ID: strings.Repeat("f", 64), Session: session, Kind: "claude", Project: "mixed", Model: "<synthetic>", At: now, Source: "otlp", Tokens: model.Tokens{Input: 100}},
	}
	if e := s.Ingest(model.Batch{HostID: "h", At: now, AI: &model.AI{Usage: usage}}); e != nil {
		t.Fatal(e)
	}
	rows, e := s.Costs("h", "24h", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 2 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	for _, row := range rows {
		if row["method"] != "api_equivalent" || row["partial"] != true {
			t.Fatalf("missing partial coverage: %+v", row)
		}
		switch row["project"] {
		case "mixed":
			if math.Abs(row["usd"].(float64)-4) > 1e-9 || row["unpriced_tokens"] != int64(27) || strings.Join(row["unpriced_models"].([]string), ",") != "claude-opus-5-5x,unknown-b" {
				t.Fatalf("mixed: %+v", row)
			}
		case "other":
			if row["usd"] != float64(0) || row["unpriced_tokens"] != int64(6) || strings.Join(row["unpriced_models"].([]string), ",") != "unknown-a" {
				t.Fatalf("other: %+v", row)
			}
		default:
			t.Fatalf("unknown project: %+v", row)
		}
	}
	price, ok := modelPrice(effectivePrices(nil), "claude-sonnet-5-20261001")
	if !ok || price.Input != 2 {
		t.Fatalf("dated Sonnet price: %+v, %v", price, ok)
	}
	price, ok = modelPrice(effectivePrices(map[string]model.AIPrice{"claude-opus-5-5-20261001": {Input: 7}}), "claude-opus-5-5-20261001")
	if !ok || price.Input != 7 {
		t.Fatalf("exact override: %+v, %v", price, ok)
	}
}

func TestAIDailyRetention(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "daily.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	at := now.Add(-100 * 24 * time.Hour)
	id := strings.Repeat("a", 64)
	if e = s.Ingest(model.Batch{HostID: "h", At: at, AI: &model.AI{Usage: []model.AIUsage{{ID: strings.Repeat("b", 64), Session: id, Kind: "claude", At: at, Source: "log", Tokens: model.Tokens{Output: 3}}}}}); e != nil {
		t.Fatal(e)
	}
	if e = s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var daily int
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage_daily`).Scan(&daily)
	if daily != 1 {
		t.Fatalf("daily=%d", daily)
	}
}

func TestQuotaRetention(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "quotas.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	for i, age := range []time.Duration{8 * 24 * time.Hour, 100 * 24 * time.Hour} {
		at := now.Add(-age)
		q := model.Quota{Kind: "codex", Window: "primary", Used: float64(i+1) * 20, ObservedAt: at, ResetsAt: at.Add(5 * time.Hour), Method: "cli_reported"}
		if e = s.Ingest(model.Batch{HostID: "h", At: at, AI: &model.AI{Quotas: []model.Quota{q}}}); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Maintain(now); e != nil {
		t.Fatal(e)
	}
	var hourly, daily int
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_quotas_hourly`).Scan(&hourly)
	s.DB.QueryRow(`SELECT COUNT(*) FROM ai_quotas_daily`).Scan(&daily)
	if hourly != 1 || daily != 1 {
		t.Fatalf("hourly=%d daily=%d", hourly, daily)
	}
	quotas, e := s.Quotas("h")
	if e != nil || len(quotas) != 1 || quotas[0].Used != 20 {
		t.Fatalf("quotas=%v err=%v", quotas, e)
	}
}

func TestClaudeQuotaEstimateLabel(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "estimate.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	if e = s.Ingest(model.Batch{HostID: "h", At: now, AI: &model.AI{Usage: []model.AIUsage{{ID: strings.Repeat("b", 64), Session: id, Kind: "claude", At: now, Source: "log", Tokens: model.Tokens{Input: 25}}}}}); e != nil {
		t.Fatal(e)
	}
	q := s.ClaudeEstimates("h", 100, 200)
	if len(q) != 2 || q[0].Method != "estimated" || q[0].Used != 25 {
		t.Fatalf("estimates: %+v", q)
	}
}

func TestMixedSessionProvenanceInListAndDetail(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "provenance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	logSession := model.AgentSession{ID: id, Kind: "codex", Source: "log", At: now, State: "waiting-for-input", Project: "demo", Model: "model-a"}
	otlpSession := model.AgentSession{ID: id, Kind: "codex", Source: "otlp", At: now.Add(-time.Minute), State: "working"}
	usage := model.AIUsage{ID: strings.Repeat("b", 64), Session: id, Kind: "codex", Model: "model-a", Source: "otlp", At: now, Tokens: model.Tokens{Input: 42}, OutputTokensPerSecond: 8, SpeedMethod: "otlp"}
	for _, batch := range []model.Batch{{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{otlpSession}, Usage: []model.AIUsage{usage}}}, {HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{logSession}}}} {
		if err := s.Ingest(batch); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.AgentsWithSummary("host", map[string]model.AIPrice{"model-a": {Input: 2}})
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	row := list[0]
	if row.Source != "otlp" || row.Provenance.State != "log" || row.Provenance.Tokens != "otlp" || row.Provenance.Cost != "estimated" || row.Provenance.Speed != "otlp" || !containsSessionSource(row.Sources, "log") {
		t.Fatalf("list provenance: %+v", row)
	}
	detail, err := s.SessionDetail("host", id, map[string]model.AIPrice{"model-a": {Input: 2}})
	if err != nil || detail.Tokens.Input != 42 || detail.TokensBySource["otlp"].Input != 42 || detail.Provenance.State != "log" || detail.Provenance.Tokens != "otlp" {
		t.Fatalf("detail: %+v %v", detail, err)
	}
}

func TestLogTokensRemainUntilOTLPUsageArrives(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	logUsage := model.AIUsage{ID: strings.Repeat("b", 64), Session: id, Kind: "codex", Model: "model-a", Source: "log", At: now, Tokens: model.Tokens{Input: 12}}
	otlp := model.AgentSession{ID: id, Kind: "codex", Source: "otlp", At: now, State: "working"}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{otlp}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Usage: []model.AIUsage{logUsage}}}); err != nil {
		t.Fatal(err)
	}
	list, err := s.AgentsWithSummary("host", nil)
	if err != nil || len(list) != 1 || list[0].Provenance.Tokens != "log" {
		t.Fatalf("fallback: %+v %v", list, err)
	}
	otlpUsage := logUsage
	otlpUsage.ID = strings.Repeat("c", 64)
	otlpUsage.Source = "otlp"
	otlpUsage.Tokens.Input = 24
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Usage: []model.AIUsage{otlpUsage}}}); err != nil {
		t.Fatal(err)
	}
	detail, err := s.SessionDetail("host", id, nil)
	if err != nil || detail.Tokens.Input != 24 || detail.Provenance.Tokens != "otlp" {
		t.Fatalf("preferred: %+v %v", detail, err)
	}
}
