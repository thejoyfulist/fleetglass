// fleetglass-statusline forwards Claude Code's documented status-line limits to local OTLP.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type limit struct {
	Used  *float64 `json:"used_percentage"`
	Reset *int64   `json:"resets_at"`
}
type input struct {
	Session string `json:"session_id"`
	Version string `json:"version"`
	Limits  struct {
		Five  limit `json:"five_hour"`
		Seven limit `json:"seven_day"`
		Spend limit `json:"spend_limit"`
	} `json:"rate_limits"`
}

func payload(raw []byte, now time.Time) ([]byte, []string) {
	var v input
	if json.Unmarshal(raw, &v) != nil {
		return nil, nil
	}
	session := sha256.Sum256([]byte(v.Session))
	attrs := []any{map[string]any{"key": "session.id", "value": map[string]string{"stringValue": hex.EncodeToString(session[:])}}}
	if v.Version != "" && len(v.Version) <= 64 {
		attrs = append(attrs, map[string]any{"key": "claude_code.version", "value": map[string]string{"stringValue": v.Version}})
	}
	points := []any{}
	labels := []string{}
	for _, w := range []struct {
		name, label string
		value       limit
	}{{"5h", "5h", v.Limits.Five}, {"7d", "7d", v.Limits.Seven}, {"spend", "spend", v.Limits.Spend}} {
		if w.value.Used == nil || w.value.Reset == nil || math.IsNaN(*w.value.Used) || math.IsInf(*w.value.Used, 0) || *w.value.Used < 0 || (w.name != "spend" && *w.value.Used > 100) || *w.value.Reset <= 0 {
			continue
		}
		a := append([]any{}, attrs...)
		a = append(a, map[string]any{"key": "window", "value": map[string]string{"stringValue": w.name}}, map[string]any{"key": "resets_at", "value": map[string]string{"stringValue": strconv.FormatInt(*w.value.Reset, 10)}})
		points = append(points, map[string]any{"timeUnixNano": strconv.FormatInt(now.UnixNano(), 10), "asDouble": *w.value.Used, "attributes": a})
		labels = append(labels, fmt.Sprintf("%s %.0f%%", w.label, *w.value.Used))
	}
	if len(points) == 0 {
		return nil, nil
	}
	b, _ := json.Marshal(map[string]any{"resourceMetrics": []any{map[string]any{"scopeMetrics": []any{map[string]any{"metrics": []any{map[string]any{"name": "claude_code.rate_limit.used_percentage", "gauge": map[string]any{"dataPoints": points}}}}}}}})
	return b, labels
}

func send(endpoint string, body []byte) {
	if len(body) == 0 {
		return
	}
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "http" {
		return
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/metrics"
	client := http.Client{Timeout: 250 * time.Millisecond}
	req, e := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if e != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e == nil {
		resp.Body.Close()
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("fleetglass-statusline", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	endpoint := flags.String("endpoint", os.Getenv("FLEETGLASS_STATUSLINE_ENDPOINT"), "local OTLP/HTTP endpoint")
	if flags.Parse(args) != nil {
		return 2
	}
	if *endpoint == "" {
		*endpoint = "http://127.0.0.1:4318"
	}
	raw, _ := io.ReadAll(stdin)
	command := flags.Args()
	status := 0
	if len(command) > 0 {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Stdin = bytes.NewReader(raw)
		cmd.Stdout = stdout
		cmd.Stderr = io.Discard
		if e := cmd.Run(); e != nil {
			status = 1
			if x, ok := e.(*exec.ExitError); ok {
				status = x.ExitCode()
			}
		}
	}
	body, labels := payload(raw, time.Now().UTC())
	if len(command) == 0 {
		fmt.Fprintln(stdout, strings.Join(labels, " · "))
	}
	send(*endpoint, body)
	return status
}
func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }
