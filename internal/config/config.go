package config

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/money"
)

// JSON keeps configuration within the Go standard library and rejects unknown keys.
type Config struct {
	DisplayDefaults    DisplayDefaults   `json:"display_defaults"`
	HostID             string            `json:"host_id"`
	Labels             map[string]string `json:"labels"`
	ServerURL          string            `json:"server_url"`
	Listen             string            `json:"listen"`
	Token              string            `json:"token"`
	TokenFile          string            `json:"token_file"`
	CAFile             string            `json:"ca_file"`
	BufferDir          string            `json:"buffer_dir"`
	ReadToken          string            `json:"read_token"`
	ReadTokenFile      string            `json:"read_token_file"`
	MCPListen          string            `json:"mcp_listen"`
	MCPTokenFile       string            `json:"mcp_token_file"`
	MCPToken           string            `json:"-"`
	LiveMaxSubscribers int               `json:"live_max_subscribers"`
	ProcRoot           string            `json:"proc_root"`
	SysRoot            string            `json:"sys_root"`
	SystemdUnits       []string          `json:"systemd_units"`
	DockerSockets      []string          `json:"docker_sockets"`
	TLSDomains         []string          `json:"tls_domains"`
	DNSProbeNames      []string          `json:"dns_probe_names"`
	SMARTDevices       []string          `json:"smart_devices"`
	HelperSocket       string            `json:"helper_socket"`
	DBPath             string            `json:"db_path"`
	OTLPListen         string            `json:"otlp_listen"`
	OTLPToken          string            `json:"otlp_token"`
	AILogRoots         []struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"ai_log_roots"`
	AgentCommands         []string                 `json:"agent_commands"`
	WorktreeRoots         []string                 `json:"worktree_roots"`
	ProjectRoots          []string                 `json:"project_roots"`
	StorageRoots          []string                 `json:"storage_roots"`
	BackupSets            []BackupSet              `json:"backup_sets"`
	StatusFiles           []StatusFile             `json:"status_files"`
	LogMarkers            []LogMarker              `json:"log_markers"`
	RepositoryRoots       []string                 `json:"repository_roots"`
	LocalOnlyRepositories []string                 `json:"local_only_repositories"`
	ExcludedRepositories  []string                 `json:"excluded_repositories"`
	InternalProbes        []InternalProbe          `json:"internal_probes"`
	HTTPProbes            []HTTPProbe              `json:"http_probes"`
	StatusListen          string                   `json:"status_listen"`
	AIPrices              map[string]model.AIPrice `json:"ai_prices"`
	AISubscriptions       []model.AISubscription   `json:"ai_subscriptions"`
	ClaudeBudget5H        int64                    `json:"claude_budget_5h"`
	ClaudeBudgetWeek      int64                    `json:"claude_budget_week"`
	EnergyPricePerKWh     float64                  `json:"energy_price_per_kwh"`
	EnergyCurrency        string                   `json:"energy_currency"`
	DisplayCurrency       string                   `json:"display_currency"`
	ExchangeRates         *ExchangeRates           `json:"exchange_rates"`
	ExchangeRateSource    string                   `json:"exchange_rate_source"`
	APTHistoryPath        string                   `json:"apt_history_path"`
	BackupDir             string                   `json:"backup_dir"`
	BackupRetentionDays   int                      `json:"backup_retention_days"`
	UpdateCheckURL        string                   `json:"update_check_url"`
}

// Rates are units of each currency per one EUR, matching the ECB feed.
type ExchangeRates struct {
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

type DisplayDefaults struct {
	Language    string `json:"language"`
	Temperature string `json:"temperature"`
	Network     string `json:"network"`
	CPU         string `json:"cpu"`
}

type BackupSet struct {
	Name           string   `json:"name"`
	Pattern        string   `json:"pattern"`
	Suffixes       []string `json:"suffixes"`
	MaxAgeSeconds  int64    `json:"max_age_seconds"`
	WarnAgeSeconds int64    `json:"warn_age_seconds"`
	MinSizeBytes   int64    `json:"min_size_bytes"`
}
type StatusFile struct {
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	FieldPath     string      `json:"field_path"`
	OKValues      []string    `json:"ok_values"`
	WarnValues    []string    `json:"warn_values"`
	CritValues    []string    `json:"crit_values"`
	MissingState  model.State `json:"missing_state"`
	MaxAgeSeconds int64       `json:"max_age_seconds"`
}
type LogMarker struct {
	Name          string      `json:"name"`
	DisplayName   string      `json:"display_name,omitempty"`
	Path          string      `json:"path"`
	Contains      string      `json:"contains"`
	MaxAgeSeconds int64       `json:"max_age_seconds"`
	PresentState  model.State `json:"present_state"`
	AbsentState   model.State `json:"absent_state"`
	ChangeEvent   bool        `json:"change_event"`
}
type InternalProbe struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Public      bool   `json:"public"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	TimeoutMS   int    `json:"timeout_ms"`
}

type HTTPProbe struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	URL             string `json:"url"`
	Method          string `json:"method"`
	ExpectedStatus  []int  `json:"expected_status"`
	BodyKeyword     string `json:"body_keyword"`
	TimeoutMS       int    `json:"timeout_ms"`
	IntervalSeconds int    `json:"interval_seconds"`
	FollowRedirects bool   `json:"follow_redirects"`
	Public          bool   `json:"public"`
}

func validDNSName(name string) bool {
	label := regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	for _, part := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if !label.MatchString(part) {
			return false
		}
	}
	return name != ""
}

func Load(path string) (Config, error) {
	host, _ := os.Hostname()
	c := Config{HostID: host, ServerURL: "http://127.0.0.1:7878", Listen: "127.0.0.1:7878", OTLPListen: "127.0.0.1:4318", ProcRoot: "/proc", SysRoot: "/sys", DockerSockets: []string{"/var/run/docker.sock"}, HelperSocket: "/run/fleetglass/helper.sock", DBPath: "fleetglass.db", LiveMaxSubscribers: 32, AgentCommands: []string{"claude", "codex"}, StorageRoots: []string{"/", "/home", "/var"}, EnergyCurrency: "EUR", DisplayCurrency: "USD", APTHistoryPath: "/var/log/apt"}
	c.BufferDir = "/var/lib/fleetglass/buffer"
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return c, err
		}
		defer f.Close()
		d := json.NewDecoder(f)
		d.DisallowUnknownFields()
		if err := d.Decode(&c); err != nil {
			return c, err
		}
	}
	if c.TokenFile != "" {
		b, err := os.ReadFile(c.TokenFile)
		if err != nil {
			return c, err
		}
		c.Token = strings.TrimSpace(string(b))
	}
	u, err := url.Parse(c.ServerURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return c, fmt.Errorf("invalid server_url")
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return c, fmt.Errorf("remote server_url requires HTTPS")
		}
	}
	if c.CAFile != "" && (u.Scheme != "https" || !filepath.IsAbs(c.CAFile)) {
		return c, fmt.Errorf("ca_file requires HTTPS and an absolute path")
	}
	if c.Token == "" {
		c.Token = os.Getenv("FLEETGLASS_TOKEN")
	}
	if c.ReadTokenFile != "" {
		b, err := os.ReadFile(c.ReadTokenFile)
		if err != nil {
			return c, err
		}
		c.ReadToken = strings.TrimSpace(string(b))
	}
	if c.MCPListen != "" {
		if c.MCPTokenFile == "" || !filepath.IsAbs(c.MCPTokenFile) {
			return c, fmt.Errorf("mcp_token_file must be an absolute path when MCP is enabled")
		}
		b, err := os.ReadFile(c.MCPTokenFile)
		if err != nil {
			return c, err
		}
		c.MCPToken = strings.TrimSpace(string(b))
		if len(c.MCPToken) < 32 {
			return c, fmt.Errorf("MCP token must have at least 32 characters")
		}
		host, _, err := net.SplitHostPort(c.MCPListen)
		if err != nil || host == "" {
			return c, fmt.Errorf("invalid mcp_listen")
		}
		// Remote exposure requires an operator-managed TLS reverse proxy.
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return c, fmt.Errorf("mcp_listen must use a loopback IP")
		}
	}
	if c.DisplayDefaults.Language == "" {
		c.DisplayDefaults.Language = "en"
	}
	if c.DisplayDefaults.Temperature == "" {
		c.DisplayDefaults.Temperature = "C"
	}
	if c.DisplayDefaults.Network == "" {
		c.DisplayDefaults.Network = "bytes"
	}
	if c.DisplayDefaults.CPU == "" {
		c.DisplayDefaults.CPU = "core"
	}
	if (c.DisplayDefaults.Language != "en" && c.DisplayDefaults.Language != "fr") || (c.DisplayDefaults.Temperature != "C" && c.DisplayDefaults.Temperature != "F") || (c.DisplayDefaults.Network != "bytes" && c.DisplayDefaults.Network != "bits") || (c.DisplayDefaults.CPU != "core" && c.DisplayDefaults.CPU != "machine") {
		return c, fmt.Errorf("invalid display_defaults")
	}
	if c.LiveMaxSubscribers <= 0 {
		return c, fmt.Errorf("live_max_subscribers must be positive")
	}
	if c.ClaudeBudget5H < 0 || c.ClaudeBudgetWeek < 0 {
		return c, fmt.Errorf("Claude budgets must be nonnegative")
	}
	if c.EnergyPricePerKWh < 0 || math.IsNaN(c.EnergyPricePerKWh) || math.IsInf(c.EnergyPricePerKWh, 0) || !money.ValidCurrency(c.EnergyCurrency) {
		return c, fmt.Errorf("invalid energy price or currency")
	}
	if !money.ValidCurrency(c.DisplayCurrency) || c.ExchangeRateSource != "" && c.ExchangeRateSource != "ecb" {
		return c, fmt.Errorf("invalid display currency or exchange rate source")
	}
	if c.ExchangeRates != nil {
		if _, err := time.Parse("2006-01-02", c.ExchangeRates.Date); err != nil {
			return c, fmt.Errorf("invalid exchange rate date")
		}
		for code, rate := range c.ExchangeRates.Rates {
			if !money.ValidCurrency(code) || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) || code == "EUR" && rate != 1 {
				return c, fmt.Errorf("invalid exchange rate")
			}
		}
	}
	if !filepath.IsAbs(c.APTHistoryPath) || filepath.Clean(c.APTHistoryPath) != c.APTHistoryPath {
		return c, fmt.Errorf("invalid apt history path")
	}
	if c.BackupDir != "" && (!filepath.IsAbs(c.BackupDir) || filepath.Clean(c.BackupDir) != c.BackupDir || c.BackupRetentionDays < 1) {
		return c, fmt.Errorf("backup_dir requires an absolute path and positive backup_retention_days")
	}
	if c.UpdateCheckURL != "" {
		u, err := url.Parse(c.UpdateCheckURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return c, fmt.Errorf("update_check_url must be an HTTPS URL without credentials")
		}
	}
	for _, p := range c.AIPrices {
		for _, v := range []float64{p.Input, p.Output, p.CacheWrite, p.CacheRead} {
			if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return c, fmt.Errorf("AI prices must be finite and nonnegative")
			}
		}
	}
	for _, s := range c.AISubscriptions {
		amount, code := s.Amount()
		if (s.Kind != "claude" && s.Kind != "codex") || !money.ValidCurrency(code) || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || s.MonthlyUSD < 0 || s.Currency != "" && s.MonthlyUSD != 0 || s.Currency == "" && s.Monthly != 0 {
			return c, fmt.Errorf("invalid AI subscription")
		}
	}
	identifier := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	for _, name := range c.DNSProbeNames {
		if len(name) > 253 || net.ParseIP(name) != nil || strings.ContainsAny(name, "/:@ ") || !validDNSName(name) {
			return c, fmt.Errorf("invalid dns_probe_names entry")
		}
	}
	seen := map[string]bool{}
	checkName := func(kind, name string) error {
		if !identifier.MatchString(name) || seen[kind+":"+name] {
			return fmt.Errorf("invalid or duplicate %s name", kind)
		}
		seen[kind+":"+name] = true
		return nil
	}
	validPath := func(p string) bool { return filepath.IsAbs(p) && filepath.Clean(p) == p && !strings.Contains(p, "..") }
	for _, b := range c.BackupSets {
		if err := checkName("backup", b.Name); err != nil {
			return c, err
		}
		if !validPath(b.Pattern) || b.MaxAgeSeconds <= 0 || b.MinSizeBytes < 0 || b.WarnAgeSeconds < 0 || b.WarnAgeSeconds >= b.MaxAgeSeconds && b.WarnAgeSeconds > 0 {
			return c, fmt.Errorf("invalid backup set")
		}
		for _, suffix := range b.Suffixes {
			if !strings.HasPrefix(suffix, ".") || strings.ContainsAny(suffix, "/\\*?[]") {
				return c, fmt.Errorf("invalid backup suffix")
			}
		}
	}
	for _, s := range c.StatusFiles {
		if err := checkName("status", s.Name); err != nil {
			return c, err
		}
		if !validPath(s.Path) || s.FieldPath == "" || s.MaxAgeSeconds <= 0 {
			return c, fmt.Errorf("invalid status file")
		}
		for _, part := range strings.Split(s.FieldPath, ".") {
			if !identifier.MatchString(part) {
				return c, fmt.Errorf("invalid status field path")
			}
		}
		if s.MissingState != "" && s.MissingState != model.Crit && s.MissingState != model.Warn && s.MissingState != model.Unknown {
			return c, fmt.Errorf("invalid missing state")
		}
	}
	for _, m := range c.LogMarkers {
		if err := checkName("log marker", m.Name); err != nil {
			return c, err
		}
		if !validPath(m.Path) || m.Contains == "" || len(m.Contains) > 80 || m.MaxAgeSeconds <= 0 || len(m.DisplayName) > 80 || strings.ContainsAny(m.DisplayName, "\r\n") {
			return c, fmt.Errorf("invalid log marker")
		}
		for _, state := range []model.State{m.PresentState, m.AbsentState} {
			if state != "" && state != model.OK && state != model.Warn && state != model.Crit && state != model.Unknown {
				return c, fmt.Errorf("invalid log marker state")
			}
		}
	}
	for _, root := range c.RepositoryRoots {
		if !validPath(root) {
			return c, fmt.Errorf("invalid repository root")
		}
	}
	for _, roots := range [][]string{c.ProjectRoots, c.WorktreeRoots, c.StorageRoots} {
		if len(roots) > 32 {
			return c, fmt.Errorf("too many configured roots")
		}
		for _, root := range roots {
			if !validPath(root) {
				return c, fmt.Errorf("invalid configured root")
			}
		}
	}
	for _, name := range c.LocalOnlyRepositories {
		if !identifier.MatchString(name) {
			return c, fmt.Errorf("invalid local-only repository")
		}
	}
	for _, name := range c.ExcludedRepositories {
		if !identifier.MatchString(name) {
			return c, fmt.Errorf("invalid excluded repository")
		}
	}
	for _, p := range c.InternalProbes {
		if err := checkName("internal probe", p.Name); err != nil {
			return c, err
		}
		if p.TimeoutMS <= 0 || p.TimeoutMS > 8000 {
			return c, fmt.Errorf("invalid probe timeout")
		}
		if len(p.DisplayName) > 80 || strings.ContainsAny(p.DisplayName, "\r\n") {
			return c, fmt.Errorf("invalid internal probe display name")
		}
		if p.Kind == "tcp" {
			if _, _, err := net.SplitHostPort(p.Target); err != nil {
				return c, fmt.Errorf("invalid TCP target")
			}
		} else if p.Kind == "http" {
			u, err := url.Parse(p.Target)
			if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" || u.User != nil {
				return c, fmt.Errorf("invalid HTTP target")
			}
		} else {
			return c, fmt.Errorf("invalid internal probe kind")
		}
	}
	for i := range c.HTTPProbes {
		p := &c.HTTPProbes[i]
		if err := checkName("HTTP probe", p.Name); err != nil {
			return c, err
		}
		if p.DisplayName == "" || len(p.DisplayName) > 80 || strings.ContainsAny(p.DisplayName, "\r\n") {
			return c, fmt.Errorf("invalid HTTP probe display name")
		}
		u, err := url.Parse(p.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return c, fmt.Errorf("invalid HTTP probe URL")
		}
		if p.Method == "" {
			p.Method = "GET"
		}
		if p.Method != "GET" && p.Method != "HEAD" {
			return c, fmt.Errorf("invalid HTTP probe method")
		}
		if p.Method == "HEAD" && p.BodyKeyword != "" {
			return c, fmt.Errorf("HEAD probe cannot inspect a body")
		}
		if len(p.BodyKeyword) > 256 {
			return c, fmt.Errorf("HTTP probe keyword too long")
		}
		if p.TimeoutMS == 0 {
			p.TimeoutMS = 5000
		}
		if p.TimeoutMS < 100 || p.TimeoutMS > 30000 {
			return c, fmt.Errorf("invalid HTTP probe timeout")
		}
		if p.IntervalSeconds == 0 {
			p.IntervalSeconds = 60
		}
		if p.IntervalSeconds < 30 {
			return c, fmt.Errorf("HTTP probe interval must be at least 30 seconds")
		}
		for _, code := range p.ExpectedStatus {
			if code < 100 || code > 599 {
				return c, fmt.Errorf("invalid HTTP probe status")
			}
		}
	}
	if c.StatusListen != "" {
		host, _, err := net.SplitHostPort(c.StatusListen)
		if err != nil || net.ParseIP(host) == nil || c.StatusListen == c.Listen || c.StatusListen == c.OTLPListen {
			return c, fmt.Errorf("invalid status listener")
		}
	}
	return c, nil
}

// OptionalKeysAbsent reports recent settings that an older config may add.
func OptionalKeysAbsent(path string) []string {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		return nil
	}
	keys := []string{"backup_dir", "backup_retention_days", "update_check_url", "mcp_listen", "mcp_token_file", "project_roots", "storage_roots", "display_defaults"}
	var missing []string
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// ValidateReadAccess permits unauthenticated reads only on an explicit loopback IP.
func (c Config) ValidateReadAccess() error {
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && c.ReadToken == "" {
		return fmt.Errorf("read_token required for non-loopback listen address")
	}
	return nil
}
