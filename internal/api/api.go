package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/money"
	"github.com/thejoyfulist/fleetglass/internal/netguard"
	"github.com/thejoyfulist/fleetglass/internal/store"
	"github.com/thejoyfulist/fleetglass/web"
)

type API struct {
	DisplayDefaults    config.DisplayDefaults
	Store              *store.Store
	Token              string
	LegacyHostID       string
	ReadToken          string
	AllowedHosts       []string
	MaxSubscribers     int
	KeepAlive          time.Duration
	AIPrices           map[string]model.AIPrice
	AgentNoEditMinutes int
	AISubscriptions    []model.AISubscription
	ClaudeBudget5H     int64
	ClaudeBudgetWeek   int64
	EnergyPricePerKWh  float64
	EnergyCurrency     string
	DisplayCurrency    string
	Money              *money.Service
	Version            string
	Commit             string
	UpdateCheckURL     string
	updateAt           time.Time
	updateTag          string
	updateMu           sync.Mutex
	mu                 sync.Mutex
	subs               map[chan model.Overview]struct{}
	sessions           map[string]time.Time
	loginAttempts      map[string][]time.Time
}

func New(s *store.Store, token, legacyHostID string) *API {
	return &API{Store: s, Money: &money.Service{}, Token: token, LegacyHostID: legacyHostID, AgentNoEditMinutes: 20, MaxSubscribers: 32, KeepAlive: 25 * time.Second, subs: map[chan model.Overview]struct{}{}, sessions: map[string]time.Time{}, loginAttempts: map[string][]time.Time{}}
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func loopDetail(s model.AgentSession) string {
	minutes := noEditMinutes(s)
	window := fmt.Sprintf("no file edit for %d min", minutes)
	if (s.LastFileEditAt.IsZero() || s.FirstToolAt.After(s.LastFileEditAt)) && s.FirstToolAt.After(s.StartedAt) {
		window = fmt.Sprintf("no file changes since tracking began %d min ago", minutes)
	}
	if s.Worktree == "" {
		window += "; file changes via shell are not visible without a worktree"
	}
	if s.SpeedTokensPerSecond != nil && *s.SpeedTokensPerSecond > 0 && !s.LastUsageAt.IsZero() && time.Since(s.LastUsageAt) <= 2*time.Minute {
		return fmt.Sprintf("%s generating ~%.1f tok/s, %s - possibly looping", s.Label, *s.SpeedTokensPerSecond, window)
	}
	return fmt.Sprintf("%s active, %s - possibly looping", s.Label, window)
}
func noEditMinutes(s model.AgentSession) int {
	if s.MinutesSinceFileEdit != nil {
		return *s.MinutesSinceFileEdit
	}
	base := s.StartedAt
	if s.FirstToolAt.After(base) {
		base = s.FirstToolAt
	}
	return max(0, int(time.Since(base).Minutes()))
}
func readTokenMatches(got, expected string) bool {
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", a.status)
	mux.HandleFunc("GET /api/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"required": a.ReadToken != ""})
	})
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("GET /api/v1/display-defaults", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"language": a.DisplayDefaults.Language, "temperature": a.DisplayDefaults.Temperature, "network": a.DisplayDefaults.Network, "cpu": a.DisplayDefaults.CPU, "currency": a.targetCurrency(r)})
	})
	mux.HandleFunc("GET /api/v1/version", a.version)
	mux.HandleFunc("POST /api/v1/auth", a.signIn)
	mux.HandleFunc("POST /api/v1/auth/signout", a.signOut)
	mux.HandleFunc("POST /api/v1/ingest", a.ingest)
	mux.HandleFunc("GET /api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		h, e := a.Store.Hosts()
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		writeJSON(w, h)
	})
	mux.HandleFunc("GET /api/v1/fleet", a.fleet)
	mux.HandleFunc("GET /api/v1/hosts/{id}/overview", func(w http.ResponseWriter, r *http.Request) {
		h, e := a.Store.Overview(r.PathValue("id"))
		if e != nil {
			http.Error(w, "host not found", 404)
			return
		}
		if sessions, err := a.Store.AgentsWithSummary(h.HostID, a.AIPrices); err == nil {
			h.Agents = sessions
		}
		for i := range h.Agents {
			ai.AssessActivity(&h.Agents[i], time.Now().UTC(), a.AgentNoEditMinutes)
			if h.Agents[i].Flag {
				h.Results = append(h.Results, model.Result{Name: "agent_loop:" + h.Agents[i].ID, State: model.Warn, Detail: loopDetail(h.Agents[i]), At: time.Now().UTC()})
			}
		}
		writeJSON(w, h)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/probes", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Probes(r.PathValue("id"), time.Now().UTC())
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/apps", func(w http.ResponseWriter, r *http.Request) {
		rangeName := r.URL.Query().Get("range")
		if rangeName == "" {
			rangeName = "24h"
		}
		v, e := a.Store.Apps(r.PathValue("id"), rangeName, time.Now().UTC())
		if e != nil {
			http.Error(w, "invalid app query", http.StatusBadRequest)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/observations", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Observations(r.PathValue("id"), time.Now().UTC())
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		if sessions, err := a.Store.AgentsWithSummary(r.PathValue("id"), a.AIPrices); err == nil {
			for _, session := range sessions {
				ai.AssessActivity(&session, time.Now().UTC(), a.AgentNoEditMinutes)
				if session.Flag {
					since := session.LastFileEditAt
					if since.IsZero() {
						since = session.StartedAt
					}
					if session.FirstToolAt.After(since) {
						since = session.FirstToolAt
					}
					v.Insights = append(v.Insights, store.Insight{Kind: "agent_loop", App: session.Label, Detail: loopDetail(session), Since: since})
				}
			}
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/worktrees", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Worktrees(r.PathValue("id"), time.Now().UTC())
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/traffic", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Traffic(r.PathValue("id"), time.Now().UTC())
		if e != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/changes", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Changes(r.PathValue("id"), r.URL.Query().Get("range"), r.URL.Query().Get("type"))
		if e != nil {
			http.Error(w, "invalid change query", 400)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/energy", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Energy(r.PathValue("id"), time.Now(), a.EnergyPricePerKWh, a.EnergyCurrency)
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		writeJSON(w, struct {
			store.Energy
			TodayMoney money.Amount `json:"today_money"`
			MonthMoney money.Amount `json:"month_money"`
			PriceMoney money.Amount `json:"price_money"`
		}{v, a.convert(r, v.TodayCost, v.Currency), a.convert(r, v.MonthCost, v.Currency), a.convert(r, v.PricePerKWh, v.Currency)})
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/series", func(w http.ResponseWriter, r *http.Request) {
		metric := r.URL.Query().Get("metric")
		if metric == "" {
			http.Error(w, "metric required", 400)
			return
		}
		points, e := a.Store.SeriesNamed(r.PathValue("id"), metric, r.URL.Query().Get("range"), r.URL.Query().Get("probe"))
		if e != nil {
			http.Error(w, "invalid range", 400)
			return
		}
		writeJSON(w, points)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/agents", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.AgentsWithSummary(r.PathValue("id"), a.AIPrices)
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		type listedSession struct {
			model.AgentSession
			APIMoney money.Amount `json:"api_money"`
		}
		out := make([]listedSession, 0, len(v))
		for _, session := range v {
			ai.AssessActivity(&session, time.Now().UTC(), a.AgentNoEditMinutes)
			out = append(out, listedSession{session, a.convert(r, session.APIEquivalentUSD, "USD")})
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/agents/{session}", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.SessionDetail(r.PathValue("id"), r.PathValue("session"), a.AIPrices)
		if e != nil {
			http.Error(w, "session not found", 404)
			return
		}
		ai.AssessActivity(&v.Session, time.Now().UTC(), a.AgentNoEditMinutes)
		writeJSON(w, struct {
			store.SessionDetail
			APIMoney money.Amount `json:"api_money"`
			CLIMoney money.Amount `json:"cli_money"`
		}{v, a.convert(r, v.APIEquivalentUSD, "USD"), a.convert(r, v.CLIReportedUSD, "USD")})
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/usage", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Usage(r.PathValue("id"), r.URL.Query().Get("range"), r.URL.Query().Get("by"))
		if e != nil {
			http.Error(w, "invalid query", 400)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/speed", func(w http.ResponseWriter, r *http.Request) {
		window := r.URL.Query().Get("range")
		if window == "" {
			window = "1h"
		}
		v, e := a.Store.Speed(r.PathValue("id"), window, r.URL.Query().Get("session"))
		if e != nil {
			http.Error(w, "invalid speed query", 400)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/quotas", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.Quotas(r.PathValue("id"))
		if e == nil {
			v = append(v, a.Store.ClaudeEstimates(r.PathValue("id"), a.ClaudeBudget5H, a.ClaudeBudgetWeek)...)
		}
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/costs", func(w http.ResponseWriter, r *http.Request) {
		by := r.URL.Query().Get("by")
		if by == "" {
			by = "project"
		}
		v, e := a.Store.CostsBy(r.PathValue("id"), r.URL.Query().Get("range"), by, a.AIPrices, a.AISubscriptions)
		if e != nil {
			http.Error(w, "invalid range", 400)
			return
		}
		for _, row := range v {
			a.decorateCost(r, row)
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.SubscriptionComparisons(r.PathValue("id"), time.Now().UTC(), a.AIPrices, a.AISubscriptions)
		if e != nil {
			http.Error(w, "storage error", 500)
			return
		}
		type comparison struct {
			store.SubscriptionComparison
			APIMoney          money.Amount  `json:"api_money"`
			SubscriptionMoney *money.Amount `json:"subscription_money"`
		}
		out := make([]comparison, 0, len(v))
		for _, row := range v {
			item := comparison{SubscriptionComparison: row, APIMoney: a.convert(r, row.APIEquivalentUSD, "USD")}
			if row.SubscriptionAmount != nil {
				converted := a.convert(r, *row.SubscriptionAmount, row.SubscriptionCurrency)
				item.SubscriptionMoney = &converted
			}
			out = append(out, item)
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("GET /api/v1/hosts/{id}/ai/cost-series", func(w http.ResponseWriter, r *http.Request) {
		v, e := a.Store.CostSeries(r.PathValue("id"), r.URL.Query().Get("range"), a.AIPrices)
		if e != nil {
			http.Error(w, "invalid range", 400)
			return
		}
		type moneyPoint struct {
			store.Point
			Money money.Amount `json:"money"`
		}
		out := make([]moneyPoint, 0, len(v))
		for _, point := range v {
			out = append(out, moneyPoint{point, a.convert(r, point.Value, "USD")})
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("GET /api/v1/live", a.live)
	mux.Handle("GET /", web.Handler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !netguard.AllowedHost(r.Host, a.AllowedHosts) {
			http.Error(w, "host not allowed", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}
		if (strings.HasPrefix(r.URL.Path, "/api/v1/") && r.URL.Path != "/api/v1/status" && r.URL.Path != "/api/v1/ingest" && r.URL.Path != "/api/v1/auth" && r.URL.Path != "/api/v1/auth/signout" || r.URL.Path == "/metrics") && a.ReadToken != "" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !readTokenMatches(token, a.ReadToken) && !a.validSession(r) {
				http.Error(w, "unauthorized", 401)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	v, err := a.Store.PublicStatus(time.Now().UTC())
	if err != nil {
		http.Error(w, "storage error", 500)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, v)
}

// StatusHandler exposes only the public page, its assets and the minimal status API.
func (a *API) StatusHandler() http.Handler {
	page := web.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		switch {
		case r.URL.Path == "/api/v1/status":
			a.status(w, r)
		case r.URL.Path == "/status" || r.URL.Path == "/favicon.svg" || strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasSuffix(r.URL.Path, ".woff2"):
			page.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

const sessionLifetime = 12 * time.Hour
const maxSessions = 256
const maxLoginAttempts = 10
const loginWindow = time.Minute
const maxLoginAddresses = 1024

func (a *API) allowLogin(remote string, now time.Time) bool {
	address, _, err := net.SplitHostPort(remote)
	if err != nil {
		address = remote
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for ip, attempts := range a.loginAttempts {
		if len(attempts) == 0 || !now.Before(attempts[len(attempts)-1].Add(loginWindow)) {
			delete(a.loginAttempts, ip)
		}
	}
	attempts := a.loginAttempts[address]
	valid := attempts[:0]
	for _, attempt := range attempts {
		if now.Before(attempt.Add(loginWindow)) {
			valid = append(valid, attempt)
		}
	}
	if len(valid) >= maxLoginAttempts {
		a.loginAttempts[address] = valid
		return false
	}
	if len(a.loginAttempts) >= maxLoginAddresses && len(valid) == 0 {
		var oldest string
		var at time.Time
		for ip, records := range a.loginAttempts {
			if oldest == "" || records[0].Before(at) {
				oldest, at = ip, records[0]
			}
		}
		delete(a.loginAttempts, oldest)
	}
	a.loginAttempts[address] = append(valid, now)
	return true
}

func (a *API) cookie(r *http.Request, value string, age int) *http.Cookie {
	return &http.Cookie{Name: "fleetglass_session", Value: value, Path: "/", HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode, MaxAge: age}
}

func (a *API) signIn(w http.ResponseWriter, r *http.Request) {
	if a.ReadToken == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !a.allowLogin(r.RemoteAddr, time.Now()) {
		http.Error(w, "too many sign-in attempts", http.StatusTooManyRequests)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || !readTokenMatches(input.Token, a.ReadToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	id := hex.EncodeToString(key[:])
	a.mu.Lock()
	now := time.Now()
	for key, expires := range a.sessions {
		if !now.Before(expires) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= maxSessions {
		var oldest string
		var expiry time.Time
		for key, expires := range a.sessions {
			if oldest == "" || expires.Before(expiry) {
				oldest, expiry = key, expires
			}
		}
		delete(a.sessions, oldest)
	}
	a.sessions[id] = now.Add(sessionLifetime)
	a.mu.Unlock()
	http.SetCookie(w, a.cookie(r, id, int(sessionLifetime.Seconds())))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) validSession(r *http.Request) bool {
	c, err := r.Cookie("fleetglass_session")
	if err != nil {
		return false
	}
	a.mu.Lock()
	expires, ok := a.sessions[c.Value]
	if ok && time.Now().After(expires) {
		delete(a.sessions, c.Value)
		ok = false
	}
	a.mu.Unlock()
	return ok
}

func (a *API) signOut(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("fleetglass_session"); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, a.cookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		http.Error(w, "unauthorized", 401)
		return
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	var b model.Batch
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(&b); e != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	allowed, err := a.Store.AuthorizeHost(b.HostID, token)
	if err != nil {
		http.Error(w, "storage error", 500)
		return
	}
	if !allowed && a.Token != "" && token != "" && a.LegacyHostID != "" && b.HostID == a.LegacyHostID {
		allowed = readTokenMatches(token, a.Token)
	}
	if !allowed {
		http.Error(w, "unauthorized", 401)
		return
	}
	if b.At.After(time.Now().Add(5 * time.Minute)) {
		http.Error(w, "future sample", 400)
		return
	}
	if e := a.Store.Ingest(b); e != nil {
		http.Error(w, "invalid batch", 400)
		return
	}
	o, e := a.Store.Overview(b.HostID)
	if e == nil {
		a.mu.Lock()
		for ch := range a.subs {
			select {
			case ch <- o:
			default:
			}
		}
		a.mu.Unlock()
	}
	w.WriteHeader(204)
}
func (a *API) live(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", 500)
		return
	}
	ch := make(chan model.Overview, 8)
	a.mu.Lock()
	if len(a.subs) >= a.MaxSubscribers {
		a.mu.Unlock()
		http.Error(w, "too many live subscribers", http.StatusServiceUnavailable)
		return
	}
	a.subs[ch] = struct{}{}
	a.mu.Unlock()
	defer func() { a.mu.Lock(); delete(a.subs, ch); a.mu.Unlock() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, ": connected\n\n")
	f.Flush()
	ticker := time.NewTicker(a.KeepAlive)
	defer ticker.Stop()
	for {
		select {
		case o := <-ch:
			b, _ := json.Marshal(o)
			fmt.Fprintf(w, "event: overview\ndata: %s\n\n", b)
			f.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			f.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
