package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixture(s string) []byte { return []byte(s) }

func TestPayloadWindows(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		count     int
	}{
		{"both", `{"session_id":"synthetic-session","version":"2.1.0","rate_limits":{"five_hour":{"used_percentage":31,"resets_at":2000000000},"seven_day":{"used_percentage":52,"resets_at":2000000100}}}`, 2},
		{"one", `{"rate_limits":{"five_hour":{"used_percentage":31,"resets_at":2000000000}}}`, 1},
		{"none", `{"rate_limits":{}}`, 0},
		{"malformed", `{`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := payload(fixture(tc.raw), time.Now())
			if tc.count == 0 && b != nil {
				t.Fatal("unexpected metric")
			}
			if tc.count > 0 {
				var v map[string]any
				if json.Unmarshal(b, &v) != nil {
					t.Fatal("invalid OTLP")
				}
				if strings.Count(string(b), `"timeUnixNano"`) != tc.count {
					t.Fatalf("got %s", b)
				}
				if strings.Contains(string(b), "synthetic-session") {
					t.Fatal("raw session leaked")
				}
			}
		})
	}
}

func TestChainAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	raw := fixture(`{"rate_limits":{"five_hour":{"used_percentage":31,"resets_at":2000000000}}}`)
	var out bytes.Buffer
	if code := run([]string{"--endpoint", server.URL, "--", "cat"}, bytes.NewReader(raw), &out); code != 0 || !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("code=%d output=%q", code, out.String())
	}
	out.Reset()
	if code := run([]string{"--endpoint", server.URL, "--", "sh", "-c", "cat >/dev/null; printf exact; exit 7"}, bytes.NewReader(raw), &out); code != 7 || out.String() != "exact" {
		t.Fatalf("code=%d output=%q", code, out.String())
	}
	server.Close()
	start := time.Now()
	out.Reset()
	run([]string{"--endpoint", server.URL}, bytes.NewReader(raw), &out)
	if time.Since(start) > 300*time.Millisecond {
		t.Fatal("receiver timeout exceeded")
	}
	if !strings.Contains(out.String(), "5h 31%") {
		t.Fatal(out.String())
	}
}
