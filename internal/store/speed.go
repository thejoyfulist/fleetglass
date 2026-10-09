package store

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type SpeedSummary struct {
	Kind                  string   `json:"kind"`
	Model                 string   `json:"model"`
	Method                string   `json:"method"`
	Window                string   `json:"window"`
	LiveTokensPerSecond   *float64 `json:"live_tokens_per_second"`
	MedianTokensPerSecond *float64 `json:"median_tokens_per_second"`
	P95TokensPerSecond    *float64 `json:"p95_tokens_per_second"`
	MedianDurationMS      *float64 `json:"median_duration_ms"`
	MedianFirstTokenMS    *float64 `json:"median_first_token_ms"`
	TokensPerMinute       float64  `json:"tokens_per_minute"`
	RequestsPerMinute     float64  `json:"requests_per_minute"`
	Errors                int      `json:"api_errors"`
	Retries               int      `json:"retries"`
	Session               string   `json:"session,omitempty"`
}

func speedPercentile(values []float64, fraction float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	v := values[int(math.Ceil(float64(len(values))*fraction))-1]
	return &v
}

// Speed uses raw observations only; hourly rollups do not retain request timing.
func (s *Store) Speed(host, window, session string) ([]SpeedSummary, error) {
	d, err := duration(window)
	if err != nil || d > 24*time.Hour {
		return nil, errInvalidSpeedRange
	}
	now := time.Now().UTC()
	rows, err := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=? ORDER BY at_ns`, host, now.Add(-d).UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type group struct {
		summary                   SpeedSummary
		speeds, durations, firsts []float64
		latest                    time.Time
		live                      float64
	}
	groups := map[string]*group{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var u model.AIUsage
		if json.Unmarshal([]byte(raw), &u) != nil || session != "" && u.Session != session {
			continue
		}
		method := u.SpeedMethod
		if method == "" {
			method = "unmeasured"
		}
		key := u.Kind + "\x00" + u.Model + "\x00" + method
		g := groups[key]
		if g == nil {
			g = &group{summary: SpeedSummary{Kind: u.Kind, Model: u.Model, Method: method, Window: window, Session: session}}
			groups[key] = g
		}
		g.summary.TokensPerMinute += float64(u.Tokens.Input+u.Tokens.Output+u.Tokens.CacheWrite+u.Tokens.CacheRead) / d.Minutes()
		if u.Tokens.Output > 0 || u.RequestDurationMS > 0 {
			g.summary.RequestsPerMinute += 1 / d.Minutes()
		}
		if u.APIError {
			g.summary.Errors++
		}
		if u.Retry {
			g.summary.Retries++
		}
		if u.OutputTokensPerSecond > 0 {
			g.speeds = append(g.speeds, u.OutputTokensPerSecond)
			if u.At.After(g.latest) && now.Sub(u.At) <= 5*time.Minute {
				g.latest, g.live = u.At, u.OutputTokensPerSecond
			}
		}
		if u.RequestDurationMS > 0 {
			g.durations = append(g.durations, u.RequestDurationMS)
		}
		if u.TimeToFirstTokenMS > 0 {
			g.firsts = append(g.firsts, u.TimeToFirstTokenMS)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SpeedSummary, 0, len(groups))
	for _, g := range groups {
		g.summary.MedianTokensPerSecond = speedPercentile(g.speeds, .5)
		g.summary.P95TokensPerSecond = speedPercentile(g.speeds, .95)
		g.summary.MedianDurationMS = speedPercentile(g.durations, .5)
		g.summary.MedianFirstTokenMS = speedPercentile(g.firsts, .5)
		if !g.latest.IsZero() {
			v := g.live
			g.summary.LiveTokensPerSecond = &v
		}
		out = append(out, g.summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Model < out[j].Model
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

var errInvalidSpeedRange = &speedRangeError{}

type speedRangeError struct{}

func (*speedRangeError) Error() string { return "invalid speed range" }
