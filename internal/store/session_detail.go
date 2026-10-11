package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

type SessionDetail struct {
	Session          model.AgentSession      `json:"session"`
	Tokens           model.Tokens            `json:"tokens"`
	TokensBySource   map[string]model.Tokens `json:"tokens_by_source"`
	Sources          []string                `json:"sources"`
	Provenance       model.AgentProvenance   `json:"provenance"`
	UsageMeasured    bool                    `json:"usage_measured"`
	TokensPerMinute  float64                 `json:"tokens_per_minute"`
	APIEquivalentUSD float64                 `json:"api_equivalent_usd"`
	CLIReportedUSD   float64                 `json:"cli_reported_usd"`
	UnpricedTokens   int64                   `json:"unpriced_tokens"`
	Processes        []model.ProjectProcess  `json:"processes"`
	Speed            []SpeedSummary          `json:"speed"`
}

func (s *Store) SessionDetail(host, id string, overrides map[string]model.AIPrice) (SessionDetail, error) {
	var out SessionDetail
	if !hashedID.MatchString(id) {
		return out, sql.ErrNoRows
	}
	var raw string
	if err := s.DB.QueryRow(`SELECT data FROM ai_sessions WHERE host_id=? AND id=?`, host, id).Scan(&raw); err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(raw), &out.Session); err != nil {
		return out, err
	}
	if out.Session.Worktree != "" {
		var treeRaw string
		err := s.DB.QueryRow(`SELECT data FROM worktree_latest WHERE host_id=? AND path=? AND at_ns>=?`, host, out.Session.Worktree, time.Now().Add(-2*time.Hour).UnixNano()).Scan(&treeRaw)
		if err != nil && err != sql.ErrNoRows {
			return out, err
		}
		if err == nil {
			var tree model.WorktreeEntry
			if json.Unmarshal([]byte(treeRaw), &tree) == nil {
				out.Session.Branch = tree.Branch
			}
		}
	}
	ai.NormalizeProvenance(&out.Session)
	out.Session.Label = SessionLabel(out.Session)
	out.TokensBySource = map[string]model.Tokens{}
	rows, err := s.DB.Query(`SELECT u.data FROM ai_usage u WHERE u.host_id=? AND u.session_id=?`, host, id)
	if err != nil {
		return out, err
	}
	prices := effectivePrices(overrides)
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			break
		}
		var usage model.AIUsage
		if json.Unmarshal([]byte(data), &usage) != nil {
			continue
		}
		out.TokensBySource[usage.Source] = addTokens(out.TokensBySource[usage.Source], usage.Tokens)
		if !containsSessionSource(out.Session.Sources, usage.Source) {
			out.Session.Sources = append(out.Session.Sources, usage.Source)
		}
		if usage.Source == "log" {
			if _, measured := out.TokensBySource["otlp"]; measured {
				continue
			}
		} else if usage.Source == "otlp" && out.Session.Provenance.Tokens == "log" {
			out.Tokens = model.Tokens{}
			out.APIEquivalentUSD = 0
			out.CLIReportedUSD = 0
			out.UnpricedTokens = 0
		}
		out.Tokens = addTokens(out.Tokens, usage.Tokens)
		out.Session.Provenance.Tokens = usage.Source
		out.Session.Provenance.Cost = "estimated"
		if usage.Tokens.Input+usage.Tokens.Output+usage.Tokens.CacheWrite+usage.Tokens.CacheRead > 0 {
			out.UsageMeasured = true
		}
		out.CLIReportedUSD += usage.CostUSD
		if price, ok := modelPrice(prices, usage.Model); ok {
			out.APIEquivalentUSD += (float64(usage.Tokens.Input)*price.Input + float64(usage.Tokens.Output)*price.Output + float64(usage.Tokens.CacheWrite)*price.CacheWrite + float64(usage.Tokens.CacheRead)*price.CacheRead) / 1e6
		} else {
			out.UnpricedTokens += usage.Tokens.Input + usage.Tokens.Output + usage.Tokens.CacheWrite + usage.Tokens.CacheRead
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	end := out.Session.EndedAt
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if !out.Session.StartedAt.IsZero() && end.After(out.Session.StartedAt) {
		out.TokensPerMinute = float64(out.Tokens.Input+out.Tokens.Output+out.Tokens.CacheWrite+out.Tokens.CacheRead) / end.Sub(out.Session.StartedAt).Minutes()
	}
	rows, err = s.DB.Query(`SELECT data FROM project_latest WHERE host_id=? AND at_ns>=?`, host, time.Now().Add(-time.Minute).UnixNano())
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			break
		}
		var process model.ProjectProcess
		if json.Unmarshal([]byte(data), &process) == nil && process.AgentSessionID == id {
			out.Processes = append(out.Processes, process)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	if out.Processes == nil {
		out.Processes = []model.ProjectProcess{}
	}
	out.Speed, err = s.Speed(host, "1h", id)
	if err == nil {
		for _, speed := range out.Speed {
			if speed.LiveTokensPerSecond != nil {
				if speed.Method == "estimated" {
					out.Session.Provenance.Speed = "estimated"
					out.Session.Provenance.SpeedFeed = out.Session.Provenance.Tokens
				} else {
					out.Session.Provenance.Speed = "otlp"
					out.Session.Provenance.SpeedFeed = "otlp"
				}
				break
			}
		}
	}
	out.Sources = out.Session.Sources
	out.Provenance = out.Session.Provenance
	return out, err
}
