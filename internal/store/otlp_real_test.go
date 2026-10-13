package store

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/model"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type capturedEvent struct {
	EventName  string            `json:"event_name"`
	Attributes map[string]string `json:"attributes"`
}

type capturedMetric struct {
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

func sendCapturedClaudeMetrics(t *testing.T, receiver *ai.Receiver, at time.Time) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "ai", "testdata", "claude-real-anonymized-metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var captured []capturedMetric
	if err := json.Unmarshal(b, &captured); err != nil {
		t.Fatal(err)
	}
	metrics := make([]*metricspb.Metric, 0, len(captured))
	for _, item := range captured {
		attrs := make([]*common.KeyValue, 0, len(item.Attributes))
		value, err := strconv.ParseFloat(item.Attributes["value"], 64)
		if err != nil {
			t.Fatal(err)
		}
		for key, raw := range item.Attributes {
			if key != "value" {
				attrs = append(attrs, &common.KeyValue{Key: key, Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: raw}}})
			}
		}
		point := &metricspb.NumberDataPoint{TimeUnixNano: uint64(at.UnixNano()), Attributes: attrs, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: value}}
		metrics = append(metrics, &metricspb.Metric{Name: item.Name, Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{DataPoints: []*metricspb.NumberDataPoint{point}}}})
	}
	part, err := proto.Marshal(&metricspb.ResourceMetrics{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}}})
	if err != nil {
		t.Fatal(err)
	}
	wire := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), part)
	r := httptest.NewRequest("POST", "/v1/metrics", bytes.NewReader(wire))
	r.Host = "127.0.0.1:4318"
	r.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()
	receiver.Handler("").ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("OTLP metrics status %d", w.Code)
	}
}

func TestRealAnonymizedOTLPSessions(t *testing.T) {
	for _, tc := range []struct {
		kind, fixture, method string
		output                int64
	}{
		{"codex", "codex-real-anonymized.json", "estimated", 5},
		{"claude", "claude-real-anonymized.json", "otlp", 6},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "ai", "testdata", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			var events []capturedEvent
			if err := json.Unmarshal(b, &events); err != nil {
				t.Fatal(err)
			}
			receiver := ai.NewReceiver()
			now := time.Now().UTC().Add(-time.Minute)
			if tc.kind == "claude" {
				sendCapturedClaudeMetrics(t, receiver, now)
			}
			records := make([]*logspb.LogRecord, 0, len(events))
			for i, event := range events {
				attrs := make([]*common.KeyValue, 0, len(event.Attributes))
				for key, value := range event.Attributes {
					attrs = append(attrs, &common.KeyValue{Key: key, Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: value}}})
				}
				records = append(records, &logspb.LogRecord{TimeUnixNano: uint64(now.Add(time.Duration(i) * 300 * time.Millisecond).UnixNano()), EventName: event.EventName, Attributes: attrs, Body: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "private body"}}})
			}
			payload := &logspb.ResourceLogs{Resource: &resource.Resource{Attributes: []*common.KeyValue{{Key: "user.email", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "resource-private@example.invalid"}}}}}, ScopeLogs: []*logspb.ScopeLogs{{LogRecords: records}}}
			part, err := proto.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			wire := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), part)
			r := httptest.NewRequest("POST", "/v1/logs", bytes.NewReader(wire))
			r.Host = "127.0.0.1:4318"
			r.Header.Set("Content-Type", "application/x-protobuf")
			w := httptest.NewRecorder()
			receiver.Handler("").ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("OTLP status %d", w.Code)
			}
			data := receiver.Drain()
			id := ai.Hash("fixture-session")
			if len(data.Sessions) != 1 || data.Sessions[0].ID != id || data.Sessions[0].Kind != tc.kind || data.Sessions[0].Source != "otlp" {
				t.Fatalf("session %+v", data.Sessions)
			}
			var measured *model.AIUsage
			for i := range data.Usage {
				if data.Usage[i].Tokens.Output == tc.output {
					measured = &data.Usage[i]
				}
			}
			if measured == nil || measured.TimeToFirstTokenMS <= 0 || measured.OutputTokensPerSecond <= 0 || measured.SpeedMethod != tc.method {
				t.Fatalf("speed %+v", data.Usage)
			}
			if tc.kind == "claude" && (measured.RequestDurationMS != 1358 || measured.Tokens.CacheRead != 14090) {
				t.Fatalf("Claude request duration %+v", measured)
			}
			if tc.kind == "claude" {
				var outputTotal int64
				var reportedCost float64
				for _, usage := range data.Usage {
					outputTotal += usage.Tokens.Output
					if usage.Method == "cli_reported" {
						reportedCost += usage.CostUSD
					}
				}
				if outputTotal != tc.output || reportedCost <= 0 {
					t.Fatalf("Claude metric and log usage: output=%d, cost=%f", outputTotal, reportedCost)
				}
			}
			if tc.kind == "codex" && measured.RequestDurationMS != 0 {
				t.Fatalf("unrelated Codex request duration was joined: %+v", measured)
			}
			data.Sessions = ai.MergeSessions(append(data.Sessions, model.AgentSession{ID: id, Kind: tc.kind, Project: "fixture-project", At: now, StartedAt: now, State: "working", Source: "log"}))
			store, err := Open(filepath.Join(t.TempDir(), "real-otlp.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := store.Ingest(model.Batch{HostID: "fixture-host", At: now, AI: &data}); err != nil {
				t.Fatal(err)
			}
			sessions, err := store.Agents("fixture-host")
			if err != nil || len(sessions) != 1 || sessions[0].Source != "otlp" || sessions[0].Project != "fixture-project" {
				t.Fatalf("merged session %+v, %v", sessions, err)
			}
			speed, err := store.Speed("fixture-host", "1h", id)
			var measuredSpeed *SpeedSummary
			for i := range speed {
				if speed[i].Method == tc.method {
					measuredSpeed = &speed[i]
				}
			}
			if err != nil || measuredSpeed == nil || measuredSpeed.MedianTokensPerSecond == nil || measuredSpeed.MedianFirstTokenMS == nil {
				t.Fatalf("stored speed %+v, %v", speed, err)
			}
			serialized, _ := json.Marshal(data)
			for _, forbidden := range []string{"private@example.invalid", "private-account", "private prompt", "private unknown", "private body", "resource-private@example.invalid"} {
				if strings.Contains(string(serialized), forbidden) {
					t.Fatalf("private value retained: %q", forbidden)
				}
				var count int
				if err := store.DB.QueryRow(`SELECT COUNT(*) FROM ai_sessions WHERE data LIKE '%' || ? || '%'`, forbidden).Scan(&count); err != nil || count != 0 {
					t.Fatalf("private session value stored: %q", forbidden)
				}
				if err := store.DB.QueryRow(`SELECT COUNT(*) FROM ai_usage WHERE data LIKE '%' || ? || '%'`, forbidden).Scan(&count); err != nil || count != 0 {
					t.Fatalf("private usage value stored: %q", forbidden)
				}
			}
		})
	}
}
