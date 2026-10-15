package ai

import (
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/netguard"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func Hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func attributes(items []*common.KeyValue) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		if item == nil || item.Value == nil {
			continue
		}
		switch item.Key {
		case "session.id", "session_id", "conversation.id", "window", "resets_at", "model", "model_name", "type", "event.name", "event.kind", "event", "tool_name", "tool.name", "status", "status_code", "error.type", "kind", "duration_ms", "request_duration_ms", "time_to_first_token_ms", "ttft_ms", "output_tokens_per_second", "output_tokens_per_sec", "input_token_count", "output_token_count", "cached_token_count", "cache_write_token_count", "reasoning_token_count", "input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens":
			v := item.Value.GetStringValue()
			if v == "" {
				switch item.Value.GetValue().(type) {
				case *common.AnyValue_IntValue:
					v = fmt.Sprint(item.Value.GetIntValue())
				case *common.AnyValue_DoubleValue:
					v = fmt.Sprint(item.Value.GetDoubleValue())
				}
			}
			if len(v) <= 128 {
				out[item.Key] = v
			}
		}
	}
	return out
}
func first(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if m[k] != "" {
			return m[k]
		}
	}
	return ""
}
func safeTag(s string) string {
	if len(s) > 128 {
		return ""
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == ':') {
			return ""
		}
	}
	return s
}
func durationMS(s string) float64 {
	var v float64
	if _, e := fmt.Sscan(s, &v); e != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
func number(v string) int64 {
	var n int64
	fmt.Sscan(v, &n)
	if n < 0 {
		return 0
	}
	return n
}
func kind(name string) string {
	switch name {
	case "api_request", "api_error", "assistant_response", "user_prompt", "tool_result", "tool_use", "api_retry":
		return "claude"
	}
	if strings.HasPrefix(name, "claude_code.") {
		return "claude"
	}
	if strings.HasPrefix(name, "codex.") {
		return "codex"
	}
	return ""
}
func validTime(ns uint64) time.Time {
	if ns == 0 {
		return time.Now().UTC()
	}
	t := time.Unix(0, int64(ns)).UTC()
	if t.Before(time.Now().Add(-7*24*time.Hour)) || t.After(time.Now().Add(5*time.Minute)) {
		return time.Time{}
	}
	return t
}

// Receiver retains only allowlisted scalar fields; bodies, resource attributes and unknown labels are never copied.
type Receiver struct {
	mu          sync.Mutex
	usage       map[string]model.AIUsage
	sessions    map[string]model.AgentSession
	cumulative  map[string]float64
	errors      map[string]int
	turnStart   map[string]time.Time
	firstToken  map[string]time.Time
	turnTTFT    map[string]float64
	metricUsage map[string]string
	claudeLogs  map[string]bool
	quotas      map[string]model.Quota
}

func NewReceiver() *Receiver {
	return &Receiver{usage: map[string]model.AIUsage{}, sessions: map[string]model.AgentSession{}, cumulative: map[string]float64{}, errors: map[string]int{}, turnStart: map[string]time.Time{}, firstToken: map[string]time.Time{}, turnTTFT: map[string]float64{}, metricUsage: map[string]string{}, claudeLogs: map[string]bool{}, quotas: map[string]model.Quota{}}
}
func (x *Receiver) Drain() model.AI {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := model.AI{Sessions: []model.AgentSession{}, Usage: []model.AIUsage{}, Quotas: []model.Quota{}}
	for id, s := range x.sessions {
		if time.Since(s.At) > 24*time.Hour {
			delete(x.sessions, id)
			delete(x.errors, id)
			delete(x.turnStart, id)
			delete(x.firstToken, id)
			delete(x.turnTTFT, id)
			delete(x.claudeLogs, id)
			continue
		}
		out.Sessions = append(out.Sessions, s)
	}
	for _, u := range x.usage {
		out.Usage = append(out.Usage, u)
	}
	for _, q := range x.quotas {
		out.Quotas = append(out.Quotas, q)
	}
	x.quotas = map[string]model.Quota{}
	x.usage = map[string]model.AIUsage{}
	x.metricUsage = map[string]string{}
	if len(x.cumulative) > 10000 {
		x.cumulative = map[string]float64{}
	}
	return out
}
func (x *Receiver) Handler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/metrics", func(w http.ResponseWriter, r *http.Request) { x.receive(w, r, true, token) })
	mux.HandleFunc("POST /v1/logs", func(w http.ResponseWriter, r *http.Request) { x.receive(w, r, false, token) })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" && !netguard.LoopbackHost(r.Host) {
			http.Error(w, "host not allowed", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func ValidateListen(addr, token string) error {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(h)
	if (ip == nil || !ip.IsLoopback()) && token == "" {
		return fmt.Errorf("OTLP non-loopback listener requires a token")
	}
	return nil
}
func (x *Receiver) receive(w http.ResponseWriter, r *http.Request, metrics bool, token string) {
	if token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	body := io.Reader(http.MaxBytesReader(w, r.Body, 2<<20))
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, e := gzip.NewReader(body)
		if e != nil {
			http.Error(w, "invalid gzip", 400)
			return
		}
		defer gz.Close()
		body = io.LimitReader(gz, 2<<20+1)
	} else if r.Header.Get("Content-Encoding") != "" {
		http.Error(w, "unsupported encoding", 415)
		return
	}
	raw, e := io.ReadAll(body)
	if e != nil || len(raw) > 2<<20 {
		http.Error(w, "body too large", 413)
		return
	}
	jsonFormat := strings.HasPrefix(r.Header.Get("Content-Type"), "application/json")
	protobufFormat := strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-protobuf")
	if !jsonFormat && !protobufFormat {
		http.Error(w, "unsupported content type", 415)
		return
	}
	if metrics {
		var values []*metricspb.ResourceMetrics
		values, e = decodeResources(raw, jsonFormat, "resourceMetrics", func() *metricspb.ResourceMetrics { return new(metricspb.ResourceMetrics) })
		if e == nil {
			x.metrics(values)
		}
	} else {
		var values []*logspb.ResourceLogs
		values, e = decodeResources(raw, jsonFormat, "resourceLogs", func() *logspb.ResourceLogs { return new(logspb.ResourceLogs) })
		if e == nil {
			x.logs(values)
		}
	}
	if e != nil {
		http.Error(w, "invalid OTLP payload", 400)
		return
	}
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	if jsonFormat {
		w.Write([]byte("{}"))
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// decodeResources reads only field 1 of the OTLP export envelope. This avoids linking unused gRPC service stubs.
func decodeResources[T proto.Message](raw []byte, jsonFormat bool, key string, newResource func() T) ([]T, error) {
	out := []T{}
	if jsonFormat {
		var envelope map[string]json.RawMessage
		if e := json.Unmarshal(raw, &envelope); e != nil {
			return nil, e
		}
		var items []json.RawMessage
		if e := json.Unmarshal(envelope[key], &items); e != nil {
			return nil, e
		}
		for _, item := range items {
			v := newResource()
			if e := protojson.Unmarshal(item, v); e != nil {
				return nil, e
			}
			out = append(out, v)
		}
		return out, nil
	}
	for len(raw) > 0 {
		number, wire, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return nil, fmt.Errorf("invalid OTLP tag")
		}
		raw = raw[n:]
		if number == 1 && wire == protowire.BytesType {
			part, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return nil, fmt.Errorf("invalid OTLP resource")
			}
			v := newResource()
			if e := proto.Unmarshal(part, v); e != nil {
				return nil, e
			}
			out = append(out, v)
			raw = raw[m:]
		} else {
			m := protowire.ConsumeFieldValue(number, wire, raw)
			if m < 0 {
				return nil, fmt.Errorf("invalid OTLP field")
			}
			raw = raw[m:]
		}
	}
	return out, nil
}
func (x *Receiver) metrics(resources []*metricspb.ResourceMetrics) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, rm := range resources {
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				if m.GetName() == "claude_code.rate_limit.used_percentage" {
					for _, p := range m.GetGauge().GetDataPoints() {
						a := attributes(p.GetAttributes())
						window := a["window"]
						minutes := int64(0)
						switch window {
						case "5h":
							minutes = 300
						case "7d":
							minutes = 10080
						case "spend":
							minutes = 0
						default:
							continue
						}
						reset, e := strconv.ParseInt(a["resets_at"], 10, 64)
						if e != nil || reset <= 0 {
							continue
						}
						at := validTime(p.GetTimeUnixNano())
						if at.IsZero() {
							continue
						}
						used := p.GetAsDouble()
						if _, ok := p.GetValue().(*metricspb.NumberDataPoint_AsInt); ok {
							used = float64(p.GetAsInt())
						}
						if math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || (window != "spend" && used > 100) {
							continue
						}
						q := model.Quota{Kind: "claude", Window: window, WindowMinutes: minutes, Used: used, ResetsAt: time.Unix(reset, 0).UTC(), ObservedAt: at, Method: "reported", Source: "statusline"}
						if old, ok := x.quotas[window]; !ok || at.After(old.ObservedAt) {
							x.quotas[window] = q
						}
					}
					continue
				}
				k := kind(m.GetName())
				if k == "" {
					continue
				}
				points := m.GetSum().GetDataPoints()
				if len(points) == 0 {
					points = m.GetGauge().GetDataPoints()
				}
				for _, p := range points {
					a := attributes(p.GetAttributes())
					sid := first(a, "session.id", "session_id", "conversation.id")
					if sid == "" {
						continue
					}
					id := Hash(sid)
					at := validTime(p.GetTimeUnixNano())
					if at.IsZero() {
						continue
					}
					value := float64(p.GetAsInt())
					if _, ok := p.GetValue().(*metricspb.NumberDataPoint_AsDouble); ok {
						value = p.GetAsDouble()
					}
					if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
						continue
					}
					if m.GetSum().GetAggregationTemporality() == metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE {
						series := Hash(id + m.GetName() + first(a, "type") + first(a, "model", "model_name"))
						previous := x.cumulative[series]
						x.cumulative[series] = value
						if value >= previous {
							value -= previous
						}
					}
					if value == 0 {
						continue
					}
					v := int64(value)
					s := model.AgentSession{ID: id, Kind: k, Model: safeTag(first(a, "model", "model_name")), At: at, StartedAt: at, State: "working", Reason: "recent OTLP activity", Source: "otlp"}
					if old, ok := x.sessions[id]; ok {
						if !old.StartedAt.IsZero() && old.StartedAt.Before(s.StartedAt) {
							s.StartedAt = old.StartedAt
						}
						if s.Model == "" {
							s.Model = old.Model
						}
						if old.At.After(s.At) {
							s.At = old.At
						}
						s.LastTurnEnded = old.LastTurnEnded
						s.LastTool = old.LastTool
						s.LastToolAt = old.LastToolAt
						s.FirstToolAt = old.FirstToolAt
						s.LastFileEditAt = old.LastFileEditAt
						s.Provenance.FileEdit = old.Provenance.FileEdit
						s.Provenance.Tool = old.Provenance.Tool
						s.LastStatus = old.LastStatus
						s.RequestDurationMS = old.RequestDurationMS
						if old.State == "erroring" || old.State == "rate-limited" {
							s.State = old.State
							s.Reason = old.Reason
						}
					}
					x.sessions[id] = s
					key := Hash(id + m.GetName() + first(a, "type") + at.Format(time.RFC3339Nano))
					u := model.AIUsage{ID: key, Session: id, Kind: k, Model: s.Model, At: at, Source: "otlp"}
					switch m.GetName() {
					case "claude_code.token.usage", "codex.token.usage":
						// Claude emits the same request tokens in logs and metrics.
						if k == "claude" && x.claudeLogs[id] {
							continue
						}
						switch a["type"] {
						case "input":
							u.Tokens.Input = v
						case "output":
							u.Tokens.Output = v
						case "cacheCreation", "cache_write":
							u.Tokens.CacheWrite = v
						case "cacheRead", "cache_read":
							u.Tokens.CacheRead = v
						default:
							continue
						}
					case "claude_code.cost.usage":
						u.CostUSD = value
						u.Method = "cli_reported"
					default:
						continue
					}
					x.usage[key] = u
					if k == "claude" && m.GetName() == "claude_code.token.usage" {
						x.metricUsage[key] = id
					}
				}
			}
		}
	}
}
func (x *Receiver) logs(resources []*logspb.ResourceLogs) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, rl := range resources {
		for _, sl := range rl.GetScopeLogs() {
			for _, rec := range sl.GetLogRecords() {
				a := attributes(rec.GetAttributes())
				name := first(a, "event.name", "event")
				if name == "" {
					name = rec.GetEventName()
				}
				k := kind(name)
				if k == "" {
					continue
				}
				sid := first(a, "session.id", "session_id", "conversation.id")
				if sid == "" {
					continue
				}
				id := Hash(sid)
				at := validTime(rec.GetTimeUnixNano())
				if at.IsZero() {
					continue
				}
				state, reason := "working", "recent OTLP activity"
				status := first(a, "status", "status_code")
				if status == "429" {
					state = "rate-limited"
					reason = "HTTP 429"
				} else if strings.Contains(name, "retries_exhausted") {
					state = "erroring"
					reason = "API retries exhausted"
				} else if strings.Contains(name, "api_error") || (strings.Contains(name, "api_request") && status != "" && status != "200") {
					x.errors[id]++
					if x.errors[id] >= 3 {
						state = "erroring"
						reason = "repeated API errors"
					}
				} else if strings.Contains(name, "api_request") && status == "200" {
					x.errors[id] = 0
				}
				if x.errors[id] >= 3 && state == "working" {
					state = "erroring"
					reason = "repeated API errors"
				}
				ended := strings.Contains(name, "turn_complete") || strings.Contains(name, "assistant_response") || (name == "codex.sse_event" && first(a, "event.kind", "kind") == "response.completed" && number(a["output_token_count"]) > 0)
				requestDuration := float64(0)
				if strings.Contains(name, "api_request") {
					requestDuration = durationMS(first(a, "request_duration_ms", "duration_ms"))
				}
				s := model.AgentSession{ID: id, Kind: k, Model: safeTag(first(a, "model", "model_name")), At: at, StartedAt: at, State: state, Reason: reason, Source: "otlp", LastTurnEnded: ended, LastStatus: safeTag(status), LastTool: safeTag(first(a, "tool_name", "tool.name")), RequestDurationMS: requestDuration}
				if s.LastTool != "" {
					s.LastToolAt = at
					s.FirstToolAt = time.Now().UTC()
					s.Provenance.Tool = "otel"
					if IsFileTool(s.LastTool) {
						s.LastFileEditAt = at
						s.Provenance.FileEdit = "otel"
					}
				}
				if old, ok := x.sessions[id]; ok {
					if !old.FirstToolAt.IsZero() && (s.FirstToolAt.IsZero() || old.FirstToolAt.Before(s.FirstToolAt)) {
						s.FirstToolAt = old.FirstToolAt
					}
					if !old.StartedAt.IsZero() && old.StartedAt.Before(s.StartedAt) {
						s.StartedAt = old.StartedAt
					}
					if old.At.After(s.At) {
						s.At = old.At
					}
					if s.Model == "" {
						s.Model = old.Model
					}
					if s.LastStatus == "" {
						s.LastStatus = old.LastStatus
					}
					if s.LastTool == "" {
						s.LastTool = old.LastTool
					}
					if old.LastToolAt.After(s.LastToolAt) {
						s.LastToolAt = old.LastToolAt
						s.LastTool = old.LastTool
						s.Provenance.Tool = old.Provenance.Tool
					}
					if old.LastFileEditAt.After(s.LastFileEditAt) {
						s.LastFileEditAt = old.LastFileEditAt
						s.Provenance.FileEdit = old.Provenance.FileEdit
					}
					if s.RequestDurationMS == 0 {
						s.RequestDurationMS = old.RequestDurationMS
					}
					if old.At.After(at) || !ended && old.LastTurnEnded && name != "codex.user_prompt" && name != "user_prompt" {
						s.LastTurnEnded = old.LastTurnEnded
					}
				}
				x.sessions[id] = s
				if name == "codex.user_prompt" {
					x.turnStart[id] = at
					delete(x.firstToken, id)
					delete(x.turnTTFT, id)
				}
				if name == "codex.turn_ttft" {
					x.firstToken[id] = at
					x.turnTTFT[id] = durationMS(first(a, "ttft_ms", "duration_ms"))
				}
				if name == "codex.sse_event" && first(a, "event.kind", "kind") == "response.completed" {
					u := model.AIUsage{ID: Hash(id + at.Format(time.RFC3339Nano)), Session: id, Kind: k, Model: s.Model, At: at, Source: "otlp"}
					u.Tokens.Output = number(a["output_token_count"])
					u.Tokens.CacheRead = number(a["cached_token_count"])
					u.Tokens.CacheWrite = number(a["cache_write_token_count"])
					u.Tokens.Input = number(a["input_token_count"]) - u.Tokens.CacheRead - u.Tokens.CacheWrite
					if u.Tokens.Input < 0 {
						u.Tokens.Input = 0
					}
					u.RequestDurationMS = durationMS(first(a, "request_duration_ms", "duration_ms"))
					sameRequestTTFT := durationMS(first(a, "time_to_first_token_ms", "ttft_ms"))
					u.TimeToFirstTokenMS = x.turnTTFT[id]
					if u.TimeToFirstTokenMS == 0 {
						u.TimeToFirstTokenMS = sameRequestTTFT
					}
					u.OutputTokensPerSecond = durationMS(first(a, "output_tokens_per_second", "output_tokens_per_sec"))
					if u.OutputTokensPerSecond == 0 && sameRequestTTFT > 0 && u.RequestDurationMS > sameRequestTTFT && u.Tokens.Output > 0 {
						u.OutputTokensPerSecond = float64(u.Tokens.Output) * 1000 / (u.RequestDurationMS - sameRequestTTFT)
					}
					if u.OutputTokensPerSecond > 0 {
						u.SpeedMethod = "otlp"
					} else if firstAt := x.firstToken[id]; u.Tokens.Output > 0 && !firstAt.IsZero() && at.Sub(firstAt) >= time.Millisecond {
						// Event timestamps span a turn, not an identified API request.
						u.OutputTokensPerSecond = float64(u.Tokens.Output) / at.Sub(firstAt).Seconds()
						u.SpeedMethod = "estimated"
					} else if started := x.turnStart[id]; u.Tokens.Output > 0 && u.TimeToFirstTokenMS > 0 && !started.IsZero() && at.Sub(started).Seconds()*1000 > u.TimeToFirstTokenMS {
						u.OutputTokensPerSecond = float64(u.Tokens.Output) * 1000 / (at.Sub(started).Seconds()*1000 - u.TimeToFirstTokenMS)
						u.SpeedMethod = "estimated"
					}
					x.usage[u.ID] = u
				}
				if k == "claude" && (name == "api_request" || name == "claude_code.api_request") {
					x.claudeLogs[id] = true
					for key, metricSession := range x.metricUsage {
						if metricSession == id {
							delete(x.usage, key)
							delete(x.metricUsage, key)
						}
					}
					u := model.AIUsage{ID: Hash(id + name + at.Format(time.RFC3339Nano)), Session: id, Kind: k, Model: s.Model, At: at, Source: "otlp"}
					u.Tokens.Input = number(a["input_tokens"])
					u.Tokens.Output = number(a["output_tokens"])
					u.Tokens.CacheRead = number(a["cache_read_tokens"])
					u.Tokens.CacheWrite = number(a["cache_creation_tokens"])
					u.RequestDurationMS = requestDuration
					u.TimeToFirstTokenMS = durationMS(first(a, "time_to_first_token_ms", "ttft_ms"))
					if u.Tokens.Output > 0 && u.RequestDurationMS > u.TimeToFirstTokenMS && u.TimeToFirstTokenMS > 0 {
						u.OutputTokensPerSecond = float64(u.Tokens.Output) * 1000 / (u.RequestDurationMS - u.TimeToFirstTokenMS)
						u.SpeedMethod = "otlp"
					}
					x.usage[u.ID] = u
					continue
				}
				if strings.Contains(name, "api_request") || strings.Contains(name, "api_error") || strings.Contains(name, "retry") {
					u := model.AIUsage{ID: Hash(id + name + at.Format(time.RFC3339Nano)), Session: id, Kind: k, Model: s.Model, At: at, Source: "otlp"}
					u.RequestDurationMS = durationMS(first(a, "request_duration_ms", "duration_ms"))
					u.TimeToFirstTokenMS = durationMS(first(a, "time_to_first_token_ms", "ttft_ms"))
					u.APIError = strings.Contains(name, "api_error") || status == "429" || strings.HasPrefix(status, "5")
					u.Retry = strings.Contains(name, "retry")
					if u.RequestDurationMS > 0 || u.TimeToFirstTokenMS > 0 || u.APIError || u.Retry {
						x.usage[u.ID] = u
					}
				}
			}
		}
	}
}
