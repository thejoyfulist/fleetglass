package store

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

var hashedID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ingestAI(tx *sql.Tx, host string, data *model.AI) error {
	if data == nil {
		return nil
	}
	// Process-only sessions are a snapshot. Remove ones that acquired a log
	// session or whose process exited since the preceding sample.
	if _, err := tx.Exec(`DELETE FROM ai_sessions WHERE host_id=? AND source='process'`, host); err != nil {
		return err
	}
	for _, s := range data.Sessions {
		if !hashedID.MatchString(s.ID) || (s.Kind != "claude" && s.Kind != "codex" && s.Kind != "unknown") || (s.Source != "otlp" && s.Source != "log" && s.Source != "worktree" && s.Source != "process") {
			return errors.New("invalid AI session")
		}
		var prevSource string
		var prevAt int64
		var previousRaw string
		e := tx.QueryRow(`SELECT source,at_ns,data FROM ai_sessions WHERE host_id=? AND id=?`, host, s.ID).Scan(&prevSource, &prevAt, &previousRaw)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if prevSource == "otlp" && s.Source == "log" {
			continue
		}
		if prevAt > s.At.UnixNano() && prevSource == s.Source {
			continue
		}
		if s.StartedAt.IsZero() {
			s.StartedAt = s.At
		}
		var previous model.AgentSession
		if json.Unmarshal([]byte(previousRaw), &previous) == nil {
			if !previous.StartedAt.IsZero() && previous.StartedAt.Before(s.StartedAt) {
				s.StartedAt = previous.StartedAt
			}
			if previous.CPUSeconds > s.CPUSeconds {
				s.CPUSeconds = previous.CPUSeconds
			}
			if previous.PeakRSSBytes > s.PeakRSSBytes {
				s.PeakRSSBytes = previous.PeakRSSBytes
			}
		}
		if s.State == "ended" && s.EndedAt.IsZero() {
			s.EndedAt = s.At
		}
		b, _ := json.Marshal(s)
		if _, e = tx.Exec(`INSERT OR REPLACE INTO ai_sessions VALUES(?,?,?,?,?)`, host, s.ID, s.At.UnixNano(), s.Source, string(b)); e != nil {
			return e
		}
		if s.Source == "otlp" {
			if _, e = tx.Exec(`DELETE FROM ai_usage WHERE host_id=? AND session_id=? AND source='log'`, host, s.ID); e != nil {
				return e
			}
		}
	}
	for _, u := range data.Usage {
		if !hashedID.MatchString(u.ID) || !hashedID.MatchString(u.Session) || (u.Kind != "claude" && u.Kind != "codex") || (u.Source != "otlp" && u.Source != "log") {
			return errors.New("invalid AI usage")
		}
		if (u.SpeedMethod != "" && u.SpeedMethod != "otlp" && u.SpeedMethod != "estimated") || u.RequestDurationMS < 0 || u.TimeToFirstTokenMS < 0 || u.OutputTokensPerSecond < 0 || math.IsNaN(u.RequestDurationMS) || math.IsNaN(u.TimeToFirstTokenMS) || math.IsNaN(u.OutputTokensPerSecond) || math.IsInf(u.RequestDurationMS, 0) || math.IsInf(u.TimeToFirstTokenMS, 0) || math.IsInf(u.OutputTokensPerSecond, 0) {
			return errors.New("invalid AI speed measurement")
		}
		if u.Source == "log" {
			var n int
			e := tx.QueryRow(`SELECT COUNT(*) FROM ai_sessions WHERE host_id=? AND id=? AND source='otlp'`, host, u.Session).Scan(&n)
			if e != nil {
				return e
			}
			if n > 0 {
				continue
			}
		}
		b, _ := json.Marshal(u)
		if _, e := tx.Exec(`INSERT OR REPLACE INTO ai_usage VALUES(?,?,?,?,?,?)`, host, u.ID, u.Session, u.At.UnixNano(), u.Source, string(b)); e != nil {
			return e
		}
	}
	for _, q := range data.Quotas {
		if q.Kind != "codex" && q.Kind != "claude" {
			return errors.New("invalid quota kind")
		}
		b, _ := json.Marshal(q)
		if _, e := tx.Exec(`INSERT OR REPLACE INTO ai_quotas VALUES(?,?,?,?,?)`, host, q.Kind, q.Window, q.ObservedAt.UnixNano(), string(b)); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Agents(host string) ([]model.AgentSession, error) {
	return s.agentsAt(host, time.Now())
}

func SessionLabel(v model.AgentSession) string {
	location := v.Worktree
	if location == "" {
		location = v.Project
	}
	location = filepath.Base(strings.TrimRight(location, "/"))
	if location == "." || location == "/" || location == "" {
		location = "session"
	}
	short := v.ID
	if len(short) > 8 {
		short = short[:8]
	}
	label := v.Kind + " · " + location
	if v.Branch != "" {
		label += " · " + v.Branch
	}
	return label + " · " + short
}

// AgentsWithSummary reads usage once for the full list, avoiding per-session API requests.
func (s *Store) AgentsWithSummary(host string, overrides map[string]model.AIPrice) ([]model.AgentSession, error) {
	sessions, err := s.Agents(host)
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(sessions))
	for i := range sessions {
		index[sessions[i].ID] = i
	}
	if len(sessions) == 0 {
		return sessions, nil
	}
	prices := effectivePrices(overrides)
	latest := map[string]time.Time{}
	now := time.Now()
	ids := make([]string, 0, len(index))
	for id := range index {
		ids = append(ids, id)
	}
	const chunkSize = 400
	for start := 0; start < len(ids); start += chunkSize {
		end := min(start+chunkSize, len(ids))
		args := make([]any, end-start+1)
		args[0] = host
		for i, id := range ids[start:end] {
			args[i+1] = id
		}
		query := `SELECT data FROM ai_usage WHERE host_id=? AND session_id IN (` + strings.TrimSuffix(strings.Repeat("?,", end-start), ",") + `)`
		rows, err := s.DB.Query(query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var raw string
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return nil, err
			}
			var u model.AIUsage
			if json.Unmarshal([]byte(raw), &u) != nil {
				continue
			}
			i, ok := index[u.Session]
			if !ok {
				continue
			}
			v := &sessions[i]
			if u.Tokens.Input+u.Tokens.Output+u.Tokens.CacheRead+u.Tokens.CacheWrite > 0 {
				v.UsageMeasured = true
			}
			if price, ok := modelPrice(prices, u.Model); ok {
				v.APIEquivalentUSD += (float64(u.Tokens.Input)*price.Input + float64(u.Tokens.Output)*price.Output + float64(u.Tokens.CacheWrite)*price.CacheWrite + float64(u.Tokens.CacheRead)*price.CacheRead) / 1e6
			}
			if u.OutputTokensPerSecond > 0 && now.Sub(u.At) <= time.Hour && u.At.After(latest[u.Session]) {
				rate := u.OutputTokensPerSecond
				v.SpeedTokensPerSecond = &rate
				v.SpeedMethod = u.SpeedMethod
				latest[u.Session] = u.At
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func (s *Store) agentsAt(host string, now time.Time) ([]model.AgentSession, error) {
	rows, e := s.DB.Query(`SELECT data FROM ai_sessions WHERE host_id=? AND (at_ns>=? OR source='process') ORDER BY at_ns DESC LIMIT 1000`, host, now.Add(-24*time.Hour).UnixNano())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.AgentSession{}
	for rows.Next() {
		var b string
		var v model.AgentSession
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if json.Unmarshal([]byte(b), &v) == nil {
			v.Label = SessionLabel(v)
			// A persisted session can outlive its last agent report. Without a
			// matched PID, its age alone is enough to establish that it ended.
			if (v.Source == "log" || v.Source == "otlp") && v.PID == 0 && now.Sub(v.At) > 30*time.Minute {
				current := []model.AgentSession{v}
				ai.ApplyHealth(current, nil, nil, now)
				v = current[0]
				if v.State == "ended" && v.EndedAt.IsZero() {
					v.EndedAt = v.At
				}
			}
			out = append(out, v)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	hasWorktree := false
	for _, session := range out {
		if session.Worktree != "" {
			hasWorktree = true
			break
		}
	}
	if !hasWorktree {
		return out, nil
	}
	trees, err := s.Worktrees(host, now)
	if err != nil {
		return nil, err
	}
	branches := make(map[string]string, len(trees.Worktrees))
	for _, tree := range trees.Worktrees {
		branches[tree.Path] = tree.Branch
	}
	for i := range out {
		if branch := branches[out[i].Worktree]; branch != "" {
			out[i].Branch = branch
			out[i].Label = SessionLabel(out[i])
		}
	}
	return out, nil
}
func duration(name string) (time.Duration, error) {
	d, ok := map[string]time.Duration{"1h": time.Hour, "12h": 12 * time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}[name]
	if !ok {
		return 0, fmt.Errorf("invalid range")
	}
	return d, nil
}
func (s *Store) Usage(host, rangeName, by string) ([]map[string]any, error) {
	d, e := duration(rangeName)
	if e != nil {
		return nil, e
	}
	if by != "project" && by != "model" && by != "agent" && by != "session" {
		return nil, fmt.Errorf("invalid grouping")
	}
	rows, e := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=? UNION ALL SELECT data FROM ai_usage_hourly WHERE host_id=? AND bucket_ns>=?`, host, time.Now().Add(-d).UnixNano(), host, time.Now().Add(-d).UnixNano())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	groups := map[string]model.Tokens{}
	hourly := map[string]map[int64]model.Tokens{}
	daily := map[string]map[int64]model.Tokens{}
	for rows.Next() {
		var b string
		var u model.AIUsage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if json.Unmarshal([]byte(b), &u) != nil {
			continue
		}
		key := u.Project
		if by == "model" {
			key = u.Model
		}
		if by == "agent" {
			key = u.Kind
		}
		if by == "session" {
			key = u.Session
		}
		if key == "" {
			key = "unknown"
		}
		groups[key] = addTokens(groups[key], u.Tokens)
		if hourly[key] == nil {
			hourly[key] = map[int64]model.Tokens{}
		}
		if daily[key] == nil {
			daily[key] = map[int64]model.Tokens{}
		}
		h := u.At.Truncate(time.Hour).UnixNano()
		day := u.At.Truncate(24 * time.Hour).UnixNano()
		hourly[key][h] = addTokens(hourly[key][h], u.Tokens)
		daily[key][day] = addTokens(daily[key][day], u.Tokens)
	}
	out := []map[string]any{}
	for k, v := range groups {
		out = append(out, map[string]any{"name": k, "tokens": v, "hourly": tokenBuckets(hourly[k]), "daily": tokenBuckets(daily[k])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	return out, rows.Err()
}
func addTokens(a, b model.Tokens) model.Tokens {
	return model.Tokens{Input: a.Input + b.Input, Output: a.Output + b.Output, CacheWrite: a.CacheWrite + b.CacheWrite, CacheRead: a.CacheRead + b.CacheRead}
}
func tokenBuckets(m map[int64]model.Tokens) []map[string]any {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := []map[string]any{}
	for _, k := range keys {
		out = append(out, map[string]any{"at": time.Unix(0, k).UTC(), "tokens": m[k]})
	}
	return out
}
func (s *Store) Quotas(host string) ([]model.Quota, error) {
	rows, e := s.DB.Query(`SELECT kind,window,at_ns,data FROM ai_quotas WHERE host_id=? UNION ALL SELECT kind,window,at_ns,data FROM ai_quotas_hourly WHERE host_id=? UNION ALL SELECT kind,window,at_ns,data FROM ai_quotas_daily WHERE host_id=?`, host, host, host)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	latest := map[string]model.Quota{}
	times := map[string]int64{}
	for rows.Next() {
		var kind, window, b string
		var at int64
		if e = rows.Scan(&kind, &window, &at, &b); e != nil {
			return nil, e
		}
		key := kind + ":" + window
		if at <= times[key] {
			continue
		}
		var q model.Quota
		if json.Unmarshal([]byte(b), &q) == nil {
			times[key] = at
			latest[key] = q
		}
	}
	out := []model.Quota{}
	for _, q := range latest {
		out = append(out, q)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Window < out[j].Window
		}
		return out[i].Kind < out[j].Kind
	})
	return out, rows.Err()
}

//go:embed prices.json
var pricesJSON []byte

func effectivePrices(overrides map[string]model.AIPrice) map[string]model.AIPrice {
	var file struct {
		Models map[string]model.AIPrice `json:"models"`
	}
	json.Unmarshal(pricesJSON, &file)
	for k, v := range overrides {
		file.Models[k] = v
	}
	return file.Models
}

// Match an exact model first, then the longest model ID followed by a hyphen.
// This covers dated snapshots and variants without matching a different version.
func modelPrice(prices map[string]model.AIPrice, modelID string) (model.AIPrice, bool) {
	if price, ok := prices[modelID]; ok {
		return price, true
	}
	var matched string
	for name := range prices {
		if name != "" && len(name) > len(matched) && strings.HasPrefix(modelID, name+"-") {
			matched = name
		}
	}
	price, ok := prices[matched]
	return price, ok
}

func (s *Store) Costs(host, rangeName string, overrides map[string]model.AIPrice, subs []model.AISubscription) ([]map[string]any, error) {
	return s.CostsBy(host, rangeName, "project", overrides, subs)
}

func (s *Store) CostsBy(host, rangeName, by string, overrides map[string]model.AIPrice, subs []model.AISubscription) ([]map[string]any, error) {
	if by != "project" && by != "model" {
		return nil, fmt.Errorf("invalid cost grouping")
	}
	d, e := duration(rangeName)
	if e != nil {
		return nil, e
	}
	rows, e := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=? UNION ALL SELECT data FROM ai_usage_hourly WHERE host_id=? AND bucket_ns>=?`, host, time.Now().Add(-d).UnixNano(), host, time.Now().Add(-d).UnixNano())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	prices := effectivePrices(overrides)
	groups := map[string]map[string]float64{}
	unpricedTokens := map[string]int64{}
	unpricedModels := map[string]map[string]bool{}
	shares := map[string]map[string]float64{}
	totals := map[string]float64{}
	for rows.Next() {
		var b string
		var u model.AIUsage
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if json.Unmarshal([]byte(b), &u) != nil {
			continue
		}
		if u.Model == "<synthetic>" {
			continue
		}
		project := u.Project
		if by == "model" {
			project = u.Model
		}
		if project == "" {
			project = "unknown"
		}
		if groups[project] == nil {
			groups[project] = map[string]float64{}
		}
		if shares[u.Kind] == nil {
			shares[u.Kind] = map[string]float64{}
		}
		tokens := float64(u.Tokens.Input + u.Tokens.Output + u.Tokens.CacheWrite + u.Tokens.CacheRead)
		shares[u.Kind][project] += tokens
		totals[u.Kind] += tokens
		if u.CostUSD > 0 {
			groups[project]["cli_reported"] += u.CostUSD
		}
		if price, ok := modelPrice(prices, u.Model); ok {
			groups[project]["api_equivalent"] += (float64(u.Tokens.Input)*price.Input + float64(u.Tokens.Output)*price.Output + float64(u.Tokens.CacheWrite)*price.CacheWrite + float64(u.Tokens.CacheRead)*price.CacheRead) / 1e6
		} else if tokens > 0 {
			unpricedTokens[project] += u.Tokens.Input + u.Tokens.Output + u.Tokens.CacheWrite + u.Tokens.CacheRead
			if unpricedModels[project] == nil {
				unpricedModels[project] = map[string]bool{}
			}
			name := u.Model
			if name == "" {
				name = "unknown"
			}
			unpricedModels[project][name] = true
		}
	}
	for _, sub := range subs {
		monthly, currency := sub.Amount()
		if monthly < 0 || math.IsNaN(monthly) {
			continue
		}
		for project, tokens := range shares[sub.Kind] {
			if totals[sub.Kind] > 0 {
				if groups[project] == nil {
					groups[project] = map[string]float64{}
				}
				groups[project]["subscription_allocated:"+currency] += monthly * float64(d) / (30 * 24 * float64(time.Hour)) * tokens / totals[sub.Kind]
			}
		}
	}
	out := []map[string]any{}
	for project, methods := range groups {
		// Emit a zero-valued API equivalent for projects with only unpriced usage.
		if unpricedTokens[project] > 0 {
			methods["api_equivalent"] += 0
		}
		for method, usd := range methods {
			currency := "USD"
			if strings.HasPrefix(method, "subscription_allocated:") {
				currency = strings.TrimPrefix(method, "subscription_allocated:")
				method = "subscription_allocated"
			}
			row := map[string]any{"project": project, "amount": usd, "currency": currency, "method": method}
			if by == "model" {
				row["model"] = project
			}
			if currency == "USD" {
				row["usd"] = usd
			}
			if method == "api_equivalent" {
				models := []string{}
				for name := range unpricedModels[project] {
					models = append(models, name)
				}
				sort.Strings(models)
				row["partial"] = unpricedTokens[project] > 0
				row["unpriced_tokens"] = unpricedTokens[project]
				row["unpriced_models"] = models
			}
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i]["project"] == out[j]["project"] {
			return out[i]["method"].(string) < out[j]["method"].(string)
		}
		return out[i]["project"].(string) < out[j]["project"].(string)
	})
	return out, rows.Err()
}
func (s *Store) ClaudeEstimates(host string, budget5h, budgetWeek int64) []model.Quota {
	now := time.Now().UTC()
	if budget5h <= 0 && budgetWeek <= 0 {
		return []model.Quota{}
	}
	rows, e := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=?`, host, now.Add(-7*24*time.Hour).UnixNano())
	if e != nil {
		return nil
	}
	defer rows.Close()
	var five, week int64
	for rows.Next() {
		var b string
		var u model.AIUsage
		if rows.Scan(&b) != nil || json.Unmarshal([]byte(b), &u) != nil || u.Kind != "claude" {
			continue
		}
		n := u.Tokens.Input + u.Tokens.Output + u.Tokens.CacheWrite + u.Tokens.CacheRead
		week += n
		if u.At.After(now.Add(-5 * time.Hour)) {
			five += n
		}
	}
	out := []model.Quota{}
	for _, v := range []struct {
		name         string
		used, budget int64
		dur          time.Duration
	}{{"5h", five, budget5h, 5 * time.Hour}, {"week", week, budgetWeek, 7 * 24 * time.Hour}} {
		if v.budget <= 0 {
			continue
		}
		used := 100 * float64(v.used) / float64(v.budget)
		if used > 100 {
			used = 100
		}
		quota := model.Quota{Kind: "claude", Window: v.name, WindowMinutes: int64(v.dur.Minutes()), Used: used, ResetsAt: now.Add(v.dur), ObservedAt: now, Method: "estimated"}
		if v.used > 0 && v.used < v.budget {
			projected := now.Add(time.Duration(float64(v.budget-v.used) / float64(v.used) * float64(v.dur)))
			if projected.Before(quota.ResetsAt) {
				quota.Projected = projected
			}
		}
		out = append(out, quota)
	}
	return out
}

func maintainAI(tx *sql.Tx, now time.Time) error {
	cutoff := now.Add(-7 * 24 * time.Hour).UnixNano()
	rows, e := tx.Query(`SELECT host_id,data FROM ai_usage WHERE at_ns<?`, cutoff)
	if e != nil {
		return e
	}
	type key struct {
		host                 string
		bucket               int64
		kind, model, project string
	}
	group := map[key]model.AIUsage{}
	for rows.Next() {
		var host, b string
		var u model.AIUsage
		if e = rows.Scan(&host, &b); e != nil {
			rows.Close()
			return e
		}
		if json.Unmarshal([]byte(b), &u) != nil {
			continue
		}
		bucket := u.At.Truncate(time.Hour).UnixNano()
		k := key{host, bucket, u.Kind, u.Model, u.Project}
		v := group[k]
		v.Kind = u.Kind
		v.Model = u.Model
		v.Project = u.Project
		v.At = time.Unix(0, bucket).UTC()
		v.Tokens.Input += u.Tokens.Input
		v.Tokens.Output += u.Tokens.Output
		v.Tokens.CacheWrite += u.Tokens.CacheWrite
		v.Tokens.CacheRead += u.Tokens.CacheRead
		v.CostUSD += u.CostUSD
		group[k] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for k, v := range group {
		var oldRaw string
		e = tx.QueryRow(`SELECT data FROM ai_usage_hourly WHERE host_id=? AND bucket_ns=? AND kind=? AND model=? AND project=?`, k.host, k.bucket, k.kind, k.model, k.project).Scan(&oldRaw)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if e == nil {
			var old model.AIUsage
			json.Unmarshal([]byte(oldRaw), &old)
			v.Tokens.Input += old.Tokens.Input
			v.Tokens.Output += old.Tokens.Output
			v.Tokens.CacheWrite += old.Tokens.CacheWrite
			v.Tokens.CacheRead += old.Tokens.CacheRead
			v.CostUSD += old.CostUSD
		}
		b, _ := json.Marshal(v)
		if _, e = tx.Exec(`INSERT OR REPLACE INTO ai_usage_hourly VALUES(?,?,?,?,?,?)`, k.host, k.bucket, k.kind, k.model, k.project, string(b)); e != nil {
			return e
		}
	}
	for _, q := range []string{`DELETE FROM ai_usage WHERE at_ns<?`, `DELETE FROM ai_sessions WHERE at_ns<? AND source!='process'`} {
		if _, e = tx.Exec(q, cutoff); e != nil {
			return e
		}
	}
	if e = downsampleAIDaily(tx, now); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM ai_usage_hourly WHERE bucket_ns<?`, now.Add(-90*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if e = downsampleQuotas(tx, "ai_quotas", "ai_quotas_hourly", now.Add(-7*24*time.Hour), time.Hour); e != nil {
		return e
	}
	if e = downsampleQuotas(tx, "ai_quotas_hourly", "ai_quotas_daily", now.Add(-90*24*time.Hour), 24*time.Hour); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM ai_quotas_daily WHERE bucket_ns<?`, now.Add(-2*365*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM ai_usage_daily WHERE bucket_ns<?`, now.Add(-2*365*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	return nil
}

func downsampleQuotas(tx *sql.Tx, source, target string, cutoff time.Time, period time.Duration) error {
	timeCol := "at_ns"
	if source == "ai_quotas_hourly" {
		timeCol = "bucket_ns"
	}
	rows, e := tx.Query(`SELECT host_id,kind,window,at_ns,data FROM `+source+` WHERE `+timeCol+`<?`, cutoff.UnixNano())
	if e != nil {
		return e
	}
	type key struct {
		host, kind, window string
		bucket             int64
	}
	type sample struct {
		at   int64
		data string
	}
	group := map[key]sample{}
	for rows.Next() {
		var host, kind, window, data string
		var at int64
		if e = rows.Scan(&host, &kind, &window, &at, &data); e != nil {
			rows.Close()
			return e
		}
		bucket := time.Unix(0, at).Truncate(period).UnixNano()
		k := key{host, kind, window, bucket}
		if at > group[k].at {
			group[k] = sample{at, data}
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for k, v := range group {
		var oldAt int64
		e = tx.QueryRow(`SELECT at_ns FROM `+target+` WHERE host_id=? AND kind=? AND window=? AND bucket_ns=?`, k.host, k.kind, k.window, k.bucket).Scan(&oldAt)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if oldAt >= v.at {
			continue
		}
		if _, e = tx.Exec(`INSERT OR REPLACE INTO `+target+` VALUES(?,?,?,?,?,?)`, k.host, k.kind, k.window, k.bucket, v.at, v.data); e != nil {
			return e
		}
	}
	_, e = tx.Exec(`DELETE FROM `+source+` WHERE `+timeCol+`<?`, cutoff.UnixNano())
	return e
}

func downsampleAIDaily(tx *sql.Tx, now time.Time) error {
	cutoff := now.Add(-90 * 24 * time.Hour).UnixNano()
	rows, e := tx.Query(`SELECT host_id,data FROM ai_usage_hourly WHERE bucket_ns<?`, cutoff)
	if e != nil {
		return e
	}
	type key struct {
		host                 string
		bucket               int64
		kind, model, project string
	}
	group := map[key]model.AIUsage{}
	for rows.Next() {
		var host, b string
		var u model.AIUsage
		if e = rows.Scan(&host, &b); e != nil {
			rows.Close()
			return e
		}
		if json.Unmarshal([]byte(b), &u) != nil {
			continue
		}
		bucket := u.At.Truncate(24 * time.Hour).UnixNano()
		k := key{host, bucket, u.Kind, u.Model, u.Project}
		v := group[k]
		v.Kind = u.Kind
		v.Model = u.Model
		v.Project = u.Project
		v.At = time.Unix(0, bucket).UTC()
		v.Tokens = addTokens(v.Tokens, u.Tokens)
		v.CostUSD += u.CostUSD
		group[k] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for k, v := range group {
		var oldRaw string
		e = tx.QueryRow(`SELECT data FROM ai_usage_daily WHERE host_id=? AND bucket_ns=? AND kind=? AND model=? AND project=?`, k.host, k.bucket, k.kind, k.model, k.project).Scan(&oldRaw)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if e == nil {
			var old model.AIUsage
			json.Unmarshal([]byte(oldRaw), &old)
			v.Tokens = addTokens(v.Tokens, old.Tokens)
			v.CostUSD += old.CostUSD
		}
		b, _ := json.Marshal(v)
		if _, e = tx.Exec(`INSERT OR REPLACE INTO ai_usage_daily VALUES(?,?,?,?,?,?)`, k.host, k.bucket, k.kind, k.model, k.project, string(b)); e != nil {
			return e
		}
	}
	return nil
}

// CostSeries reports priced API-equivalent usage per hour. Unpriced models are
// excluded, matching the coverage rule used by Costs.
func (s *Store) CostSeries(host, rangeName string, overrides map[string]model.AIPrice) ([]Point, error) {
	d, err := duration(rangeName)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-d).UnixNano()
	rows, err := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=? UNION ALL SELECT data FROM ai_usage_hourly WHERE host_id=? AND bucket_ns>=?`, host, cutoff, host, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	prices := effectivePrices(overrides)
	buckets := map[int64]float64{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var usage model.AIUsage
		if json.Unmarshal([]byte(raw), &usage) != nil || usage.Model == "<synthetic>" {
			continue
		}
		price, ok := modelPrice(prices, usage.Model)
		if !ok {
			continue
		}
		at := usage.At.Truncate(time.Hour).UnixNano()
		buckets[at] += (float64(usage.Tokens.Input)*price.Input + float64(usage.Tokens.Output)*price.Output + float64(usage.Tokens.CacheWrite)*price.CacheWrite + float64(usage.Tokens.CacheRead)*price.CacheRead) / 1e6
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys := make([]int64, 0, len(buckets))
	for at := range buckets {
		keys = append(keys, at)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]Point, 0, len(keys))
	for _, at := range keys {
		out = append(out, Point{At: time.Unix(0, at).UTC(), Value: buckets[at]})
	}
	return out, nil
}
