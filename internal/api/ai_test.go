package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/money"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestAIEndpointsAndSSEOverview(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "x.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := New(s, "secret", "host")
	h := a.Handler()
	id := strings.Repeat("a", 64)
	now := time.Now().UTC()
	batch := model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{{ID: id, Kind: "claude", Project: "demo", At: now, StartedAt: now.Add(-time.Hour), FirstToolAt: now.Add(-time.Hour), LastFileEditAt: now.Add(-27 * time.Minute), LastToolAt: now, LastTool: "Bash", State: "working", Reason: "recent activity", Source: "otlp"}}, Usage: []model.AIUsage{{ID: strings.Repeat("b", 64), Session: id, Kind: "claude", Project: "demo", Model: "claude-sonnet-5", At: now, Source: "otlp", Tokens: model.Tokens{Input: 100}, OutputTokensPerSecond: 12, SpeedMethod: "otlp"}}, Quotas: []model.Quota{{Kind: "codex", Window: "primary", Used: 50, ObservedAt: now, ResetsAt: now.Add(time.Hour), Method: "cli_reported"}}}}
	b, _ := json.Marshal(batch)
	r := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("ingest %d: %s", w.Code, w.Body.String())
	}
	for path, match := range map[string]string{"/api/v1/hosts/host/agents": "api_equivalent_usd", "/api/v1/hosts/host/ai/usage?range=24h&by=project": "input", "/api/v1/hosts/host/ai/quotas": "primary", "/api/v1/hosts/host/ai/costs?range=24h": "api_equivalent", "/api/v1/hosts/host/ai/costs?range=24h&by=model": "claude-sonnet-5", "/api/v1/hosts/host/overview": "agents"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), match) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/hosts/host/agents", "/api/v1/hosts/host/agents/" + id} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		for _, field := range []string{`"last_file_edit_at"`, `"last_tool_at"`, `"minutes_since_file_edit"`, `"activity"`, `"flag":true`} {
			if !strings.Contains(w.Body.String(), field) {
				t.Fatalf("%s missing %s: %s", path, field, w.Body.String())
			}
		}
	}
	for _, path := range []string{"/api/v1/hosts/host/overview", "/api/v1/hosts/host/observations"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "possibly looping") {
			t.Fatalf("%s missing warning: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestSubscriptionCurrencyAPI(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "currency.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "secret", "host")
	a.AISubscriptions = []model.AISubscription{{Kind: "claude", Monthly: 150, Currency: "EUR"}}
	a.Money = &money.Service{Static: &money.Snapshot{Date: "2026-10-08", Source: "static", Rates: map[string]float64{"USD": 1.2}}}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hosts/host/ai/subscriptions?display_currency=EUR", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var rows []struct {
		Kind                 string       `json:"kind"`
		SubscriptionAmount   *float64     `json:"subscription_amount"`
		SubscriptionCurrency string       `json:"subscription_currency"`
		SubscriptionMoney    money.Amount `json:"subscription_money"`
		APIMoney             money.Amount `json:"api_money"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Kind == "claude" {
			if row.SubscriptionAmount == nil || *row.SubscriptionAmount != 150 || row.SubscriptionCurrency != "EUR" || row.SubscriptionMoney.Converted == nil || *row.SubscriptionMoney.Converted != 150 || row.APIMoney.RateSource != "static" {
				t.Fatalf("currency response: %+v", row)
			}
			return
		}
	}
	t.Fatal("Claude subscription missing")
}

func TestAICostCoverageInAPIAndDebug(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "costs.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	now := time.Now().UTC()
	session := strings.Repeat("a", 64)
	usage := []model.AIUsage{
		{ID: strings.Repeat("b", 64), Session: session, Kind: "claude", Project: "demo", Model: "claude-opus-5-5", At: now, Source: "otlp", Tokens: model.Tokens{Input: 1_000_000}},
		{ID: strings.Repeat("c", 64), Session: session, Kind: "claude", Project: "demo", Model: "unknown<model>", At: now, Source: "otlp", Tokens: model.Tokens{Output: 12}},
	}
	if e := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Usage: usage}}); e != nil {
		t.Fatal(e)
	}
	h := New(s, "secret", "host").Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hosts/host/ai/costs?range=24h", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var rows []struct {
		Method         string   `json:"method"`
		USD            float64  `json:"usd"`
		Partial        bool     `json:"partial"`
		UnpricedTokens int64    `json:"unpriced_tokens"`
		UnpricedModels []string `json:"unpriced_models"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[0].Method != "api_equivalent" || rows[0].USD != 4 || !rows[0].Partial || rows[0].UnpricedTokens != 12 || len(rows[0].UnpricedModels) != 1 || rows[0].UnpricedModels[0] != "unknown<model>" {
		t.Fatalf("cost coverage: %+v", rows)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hosts/host/ai/cost-series?range=24h", nil))
	var points []store.Point
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &points) != nil || len(points) != 1 || points[0].Value != 4 {
		t.Fatalf("cost series excludes unpriced usage: %d %s", w.Code, w.Body.String())
	}
}
