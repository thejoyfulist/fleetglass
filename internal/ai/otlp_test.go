package ai

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"

	common "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func attr(k, v string) *common.KeyValue {
	return &common.KeyValue{Key: k, Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: v}}}
}
func sendOTLP(t *testing.T, h http.Handler, path, key string, v proto.Message, jsonMode, gzipMode bool) {
	t.Helper()
	var b []byte
	var e error
	ct := "application/x-protobuf"
	if jsonMode {
		item, err := protojson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		b, e = json.Marshal(map[string][]json.RawMessage{key: {item}})
		ct = "application/json"
	} else {
		item, err := proto.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		b = protowire.AppendTag(nil, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, item)
	}
	if e != nil {
		t.Fatal(e)
	}
	if gzipMode {
		var buf bytes.Buffer
		z := gzip.NewWriter(&buf)
		z.Write(b)
		z.Close()
		b = buf.Bytes()
	}
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Header.Set("Content-Type", ct)
	if gzipMode {
		r.Header.Set("Content-Encoding", "gzip")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("%s returned %d: %s", path, w.Code, w.Body.String())
	}
}
func TestOTLPFormatsAndAllowlist(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		for _, gzipMode := range []bool{false, true} {
			x := NewReceiver()
			h := x.Handler("")
			now := uint64(time.Now().UnixNano())
			a := []*common.KeyValue{attr("session.id", "session-private"), attr("model", "claude-sonnet-4-5-20250929"), attr("type", "input"), attr("prompt", "secret prompt"), attr("user.email", "secret@example.org")}
			m := &metricspb.ResourceMetrics{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "claude_code.token.usage", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{DataPoints: []*metricspb.NumberDataPoint{{TimeUnixNano: now, Attributes: a, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 42}}}}}}}}}}
			sendOTLP(t, h, "/v1/metrics", "resourceMetrics", m, jsonMode, gzipMode)
			l := &logspb.ResourceLogs{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{TimeUnixNano: now, EventName: "codex.sse_event", Body: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "secret body"}}, Attributes: []*common.KeyValue{attr("conversation.id", "codex-private"), attr("model", "gpt-6-sol"), attr("event.kind", "response.completed"), attr("input_token_count", "11"), attr("output_token_count", "2"), attr("tool_input", "secret"), attr("tool_name", "Read"), attr("duration_ms", "12"), attr("status_code", "200")}}}}}}
			sendOTLP(t, h, "/v1/logs", "resourceLogs", l, jsonMode, gzipMode)
			got := x.Drain()
			sanitized, _ := json.Marshal(got)
			for _, secret := range []string{"secret prompt", "secret@example.org", "secret body"} {
				if bytes.Contains(sanitized, []byte(secret)) {
					t.Fatalf("forbidden value retained: %s", secret)
				}
			}
			if len(got.Usage) != 2 || got.Usage[0].Session == "session-private" {
				t.Fatalf("usage: %+v", got.Usage)
			}
			foundTool := false
			for _, s := range got.Sessions {
				if s.Kind == "codex" && s.Model == "gpt-6-sol" && s.LastTool == "Read" && s.RequestDurationMS == 0 && s.LastStatus == "200" {
					foundTool = true
				}
			}
			if !foundTool {
				t.Fatalf("diagnostic fields: %+v", got.Sessions)
			}
			for _, u := range got.Usage {
				if u.Kind == "codex" && u.OutputTokensPerSecond != 0 {
					t.Fatalf("request duration alone cannot establish output speed: %+v", u)
				}
				if u.Kind == "codex" && u.Model != "gpt-6-sol" {
					t.Fatalf("Codex OTLP model: %+v", u)
				}
				if u.Tokens.Input != 42 && u.Tokens.Input != 11 {
					t.Fatalf("unexpected tokens %+v", u)
				}
			}
		}
	}
}

func TestOTLPFileToolTimestamps(t *testing.T) {
	x := NewReceiver()
	now := uint64(time.Now().UnixNano())
	attrs := []*common.KeyValue{attr("session.id", "synthetic-session"), attr("tool_name", "apply_patch")}
	l := &logspb.ResourceLogs{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{TimeUnixNano: now, EventName: "codex.tool_result", Attributes: attrs}}}}}
	sendOTLP(t, x.Handler(""), "/v1/logs", "resourceLogs", l, false, false)
	got := x.Drain()
	if len(got.Sessions) != 1 || got.Sessions[0].LastFileEditAt.IsZero() || got.Sessions[0].LastToolAt.IsZero() || got.Sessions[0].FirstToolAt.IsZero() || got.Sessions[0].Provenance.FileEdit != "otel" {
		t.Fatalf("%+v", got.Sessions)
	}
}

func TestCumulativeMetricsUseDelta(t *testing.T) {
	x := NewReceiver()
	h := x.Handler("")
	for i, n := range []int64{10, 15, 15} {
		now := uint64(time.Now().Add(time.Duration(i) * time.Second).UnixNano())
		m := &metricspb.ResourceMetrics{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "claude_code.token.usage", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, DataPoints: []*metricspb.NumberDataPoint{{TimeUnixNano: now, Attributes: []*common.KeyValue{attr("session.id", "s"), attr("type", "input")}, Value: &metricspb.NumberDataPoint_AsInt{AsInt: n}}}}}}}}}}
		sendOTLP(t, h, "/v1/metrics", "resourceMetrics", m, false, false)
	}
	got := x.Drain()
	var total int64
	for _, u := range got.Usage {
		total += u.Tokens.Input
	}
	if total != 15 {
		t.Fatalf("cumulative total %d", total)
	}
}

func TestOTLPReceiverBoundaries(t *testing.T) {
	if ValidateListen("0.0.0.0:4318", "") == nil {
		t.Fatal("public listener without token accepted")
	}
	h := NewReceiver().Handler("secret")
	r := httptest.NewRequest("POST", "/v1/logs", bytes.NewReader([]byte(`{"resourceLogs":[]}`)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("missing token: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "/v1/logs", bytes.NewReader(bytes.Repeat([]byte("x"), 2<<20+1)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatalf("oversized body: %d", w.Code)
	}
}

func TestRepeatedAPIErrors(t *testing.T) {
	x := NewReceiver()
	h := x.Handler("")
	for i := 0; i < 3; i++ {
		rec := &logspb.LogRecord{TimeUnixNano: uint64(time.Now().Add(time.Duration(i) * time.Second).UnixNano()), EventName: "claude_code.api_error", Attributes: []*common.KeyValue{attr("session.id", "s")}}
		v := &logspb.ResourceLogs{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{rec}}}}
		sendOTLP(t, h, "/v1/logs", "resourceLogs", v, false, false)
	}
	got := x.Drain()
	if len(got.Sessions) != 1 || got.Sessions[0].State != "erroring" {
		t.Fatalf("health: %+v", got.Sessions)
	}
}

func TestOTLPSpeedRequiresGenerationTime(t *testing.T) {
	x := NewReceiver()
	h := x.Handler("")
	now := uint64(time.Now().UnixNano())
	rec := &logspb.LogRecord{TimeUnixNano: now, EventName: "codex.sse_event", Attributes: []*common.KeyValue{attr("conversation.id", "speed-session"), attr("event.kind", "response.completed"), attr("model", "gpt-6-sol"), attr("output_token_count", "100"), attr("duration_ms", "4000"), attr("ttft_ms", "2000")}}
	sendOTLP(t, h, "/v1/logs", "resourceLogs", &logspb.ResourceLogs{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{rec}}}}, false, false)
	got := x.Drain()
	if len(got.Usage) != 1 || got.Usage[0].OutputTokensPerSecond != 50 || got.Usage[0].SpeedMethod != "otlp" {
		t.Fatalf("generation speed %+v", got.Usage)
	}
}

func TestCodexRequestDurationStaysSeparateFromResponseTokens(t *testing.T) {
	x := NewReceiver()
	h := x.Handler("")
	now := time.Now().Add(-time.Second)
	makeRecord := func(offset time.Duration, name string, extra ...*common.KeyValue) *logspb.LogRecord {
		attrs := append([]*common.KeyValue{attr("conversation.id", "same-session"), attr("event.name", name), attr("model", "gpt-6-sol")}, extra...)
		return &logspb.LogRecord{TimeUnixNano: uint64(now.Add(offset).UnixNano()), EventName: "event otel/src/events/session_telemetry.rs", Attributes: attrs}
	}
	records := []*logspb.LogRecord{
		makeRecord(0, "codex.user_prompt"),
		makeRecord(100*time.Millisecond, "codex.api_request", attr("duration_ms", "4000")),
		makeRecord(300*time.Millisecond, "codex.turn_ttft", attr("duration_ms", "200")),
		makeRecord(800*time.Millisecond, "codex.sse_event", attr("event.kind", "response.completed"), attr("output_token_count", "10")),
	}
	sendOTLP(t, h, "/v1/logs", "resourceLogs", &logspb.ResourceLogs{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: records}}}, false, false)
	got := x.Drain()
	if len(got.Sessions) != 1 || got.Sessions[0].RequestDurationMS != 4000 {
		t.Fatalf("request duration %+v", got.Sessions)
	}
	var response *model.AIUsage
	for i := range got.Usage {
		if got.Usage[i].Tokens.Output == 10 {
			response = &got.Usage[i]
		}
	}
	if response == nil || response.TimeToFirstTokenMS != 200 || response.RequestDurationMS != 0 || response.SpeedMethod != "estimated" || response.OutputTokensPerSecond != 20 {
		t.Fatalf("unrelated duration was used as exact speed: %+v", got.Usage)
	}
}
