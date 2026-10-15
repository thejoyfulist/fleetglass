package model

import "time"

type State string

const (
	OK      State = "ok"
	Warn    State = "warn"
	Crit    State = "crit"
	Unknown State = "unknown"
	Stale   State = "stale"
)

type Result struct {
	Name           string             `json:"name"`
	State          State              `json:"state"`
	Detail         string             `json:"detail"`
	DetailCode     string             `json:"detail_code,omitempty"`
	DetailParams   map[string]string  `json:"detail_params,omitempty"`
	Values         map[string]float64 `json:"values,omitempty"`
	Fields         map[string]string  `json:"fields,omitempty"`
	At             time.Time          `json:"at,omitempty"`
	CadenceSeconds int                `json:"cadence_seconds,omitempty"`
}

type Batch struct {
	HostID     string            `json:"host_id"`
	Labels     map[string]string `json:"labels,omitempty"`
	At         time.Time         `json:"at"`
	Results    []Result          `json:"results"`
	AI         *AI               `json:"ai,omitempty"`
	Changes    []Change          `json:"changes,omitempty"`
	Probes     []ProbeSample     `json:"probes,omitempty"`
	ProbeNames *[]string         `json:"probe_names,omitempty"`
	Apps       []AppSample       `json:"apps,omitempty"`
	Projects   []ProjectProcess  `json:"projects,omitempty"`
	Storage    []StorageEntry    `json:"storage,omitempty"`
	Worktrees  []WorktreeEntry   `json:"worktrees,omitempty"`
}

type WorktreeEntry struct {
	Root           string    `json:"root"`
	Project        string    `json:"project"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Branch         string    `json:"branch,omitempty"`
	Head           string    `json:"head,omitempty"`
	DirtyFiles     int       `json:"dirty_files"`
	StatusComplete bool      `json:"status_complete"`
	IndexMTime     time.Time `json:"index_mtime,omitzero"`
	LastChangeAt   time.Time `json:"last_change_at,omitzero"`
	SizeBytes      int64     `json:"size_bytes"`
	SizeComplete   bool      `json:"size_complete"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	LastCommitAt   time.Time `json:"last_commit_at,omitempty"`
	Merged         *bool     `json:"merged,omitempty"`
	AgentProcesses int       `json:"agent_processes"`
	Processes      int       `json:"processes"`
	Stale          bool      `json:"stale"`
	Reclaimable    bool      `json:"reclaimable"`
}

// ProjectProcess exposes configured-root processes and witnessed agent descendants.
type ProjectProcess struct {
	PID                   int       `json:"pid"`
	Name                  string    `json:"name"`
	Project               string    `json:"project"`
	Worktree              string    `json:"worktree"`
	Kind                  string    `json:"kind"`
	CPUPercent            float64   `json:"cpu_percent"`
	MemoryBytes           float64   `json:"memory_bytes"`
	Ports                 []int     `json:"ports"`
	PortsMeasured         bool      `json:"ports_measured"`
	ConnectionsMeasured   bool      `json:"connections_measured"`
	IdleMinutes           float64   `json:"idle_minutes"`
	ConnectionIdleMinutes float64   `json:"connection_idle_minutes"`
	Idle                  bool      `json:"idle"`
	StopCommand           string    `json:"stop_command"`
	AgentSessionID        string    `json:"agent_session_id,omitempty"`
	AgentKind             string    `json:"agent_kind,omitempty"`
	LeftBehind            bool      `json:"left_behind,omitempty"`
	StartedAt             time.Time `json:"started_at,omitempty"`
}

type StorageEntry struct {
	Root      string `json:"root"`
	Path      string `json:"path"`
	Depth     int    `json:"depth"`
	SizeBytes int64  `json:"size_bytes"`
	Complete  bool   `json:"complete"`
}

// AppProcess exposes only the kernel process name and PID.
type AppProcess struct {
	Name string `json:"name"`
	PID  int    `json:"pid"`
}

// AppSample is an aggregate with a name-only process inventory for the latest sample.
type AppSample struct {
	Processes        []AppProcess `json:"processes,omitempty"`
	Name             string       `json:"name"`
	Kind             string       `json:"kind"`
	RatesReady       bool         `json:"rates_ready"`
	CPUPercent       float64      `json:"cpu_percent"`
	MemoryBytes      float64      `json:"memory_bytes"`
	PageCacheBytes   float64      `json:"page_cache_bytes"`
	DiskReadRate     float64      `json:"disk_read_bytes_per_second"`
	DiskWriteRate    float64      `json:"disk_write_bytes_per_second"`
	DiskReadBytes    float64      `json:"disk_read_bytes"`
	DiskWriteBytes   float64      `json:"disk_write_bytes"`
	DiskReadCounter  float64      `json:"disk_read_counter_bytes,omitempty"`
	DiskWriteCounter float64      `json:"disk_write_counter_bytes,omitempty"`
	CounterSource    string       `json:"counter_source,omitempty"`
	NetworkRXRate    *float64     `json:"network_rx_bytes_per_second,omitempty"`
	NetworkTXRate    *float64     `json:"network_tx_bytes_per_second,omitempty"`
	EnergyWatts      *float64     `json:"estimated_energy_watts,omitempty"`
}

// ProbeSample contains only configured display metadata and a bounded observation.
type ProbeSample struct {
	Name            string    `json:"name"`
	DisplayName     string    `json:"display_name"`
	Public          bool      `json:"public"`
	Kind            string    `json:"kind"`
	State           State     `json:"state"`
	StatusCode      int       `json:"status_code,omitempty"`
	LatencyMS       float64   `json:"latency_ms,omitempty"`
	DNSMS           float64   `json:"dns_ms,omitempty"`
	ConnectMS       float64   `json:"connect_ms,omitempty"`
	TLSMS           float64   `json:"tls_ms,omitempty"`
	FirstByteMS     float64   `json:"first_byte_ms,omitempty"`
	CertificateDays *int      `json:"certificate_days,omitempty"`
	IntervalSeconds int       `json:"interval_seconds"`
	At              time.Time `json:"at"`
}

// Change contains selected metadata only; source lines and command output never enter this type.
type Change struct {
	ID           string            `json:"id"`
	At           time.Time         `json:"at"`
	Type         string            `json:"type"`
	Text         string            `json:"text"`
	ChangeCode   string            `json:"change_code,omitempty"`
	ChangeParams map[string]string `json:"change_params,omitempty"`
}

type Overview struct {
	HostID   string            `json:"host_id"`
	Labels   map[string]string `json:"labels,omitempty"`
	At       time.Time         `json:"at"`
	Liveness string            `json:"liveness"`
	Results  []Result          `json:"results"`
	Agents   []AgentSession    `json:"agents,omitempty"`
}

type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheWrite int64 `json:"cache_write"`
	CacheRead  int64 `json:"cache_read"`
}

type AIPrice struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
}

type AISubscription struct {
	Kind       string  `json:"kind"`
	Monthly    float64 `json:"monthly,omitempty"`
	Currency   string  `json:"currency,omitempty"`
	MonthlyUSD float64 `json:"monthly_usd"`
}

func (s AISubscription) Amount() (float64, string) {
	if s.Currency != "" {
		return s.Monthly, s.Currency
	}
	return s.MonthlyUSD, "USD"
}

type AgentProvenance struct {
	FileEdit  string `json:"file_edit,omitempty"`
	Tool      string `json:"tool,omitempty"`
	State     string `json:"state,omitempty"`
	Tokens    string `json:"tokens,omitempty"`
	Cost      string `json:"cost,omitempty"`
	Speed     string `json:"speed,omitempty"`
	SpeedFeed string `json:"speed_feed,omitempty"`
	Resources string `json:"resources,omitempty"`
	Model     string `json:"model,omitempty"`
	Project   string `json:"project,omitempty"`
}

type AgentSession struct {
	Sources              []string        `json:"sources,omitempty"`
	Provenance           AgentProvenance `json:"provenance"`
	ID                   string          `json:"id"`
	Label                string          `json:"label,omitempty"`
	Branch               string          `json:"branch,omitempty"`
	APIEquivalentUSD     float64         `json:"api_equivalent_usd,omitempty"`
	UsageMeasured        bool            `json:"usage_measured,omitempty"`
	SpeedTokensPerSecond *float64        `json:"speed_tokens_per_second,omitempty"`
	SpeedMethod          string          `json:"speed_method,omitempty"`
	Kind                 string          `json:"kind"`
	Model                string          `json:"model,omitempty"`
	Project              string          `json:"project,omitempty"`
	Worktree             string          `json:"worktree,omitempty"`
	User                 string          `json:"user,omitempty"`
	PID                  int             `json:"pid,omitempty"`
	CPU                  float64         `json:"cpu_percent,omitempty"`
	RSS                  int64           `json:"rss_bytes,omitempty"`
	At                   time.Time       `json:"at"`
	StartedAt            time.Time       `json:"started_at,omitempty"`
	EndedAt              time.Time       `json:"ended_at,omitempty"`
	CPUSeconds           float64         `json:"cpu_seconds,omitempty"`
	PeakRSSBytes         int64           `json:"peak_rss_bytes,omitempty"`
	State                string          `json:"state"`
	Reason               string          `json:"reason"`
	ReasonCode           string          `json:"reason_code,omitempty"`
	Source               string          `json:"source"`
	LastTurnEnded        bool            `json:"last_turn_ended,omitempty"`
	LastStatus           string          `json:"last_status,omitempty"`
	LastTool             string          `json:"last_tool,omitempty"`
	LastToolAt           time.Time       `json:"last_tool_at,omitzero"`
	FirstToolAt          time.Time       `json:"first_tool_at,omitzero"`
	LastUsageAt          time.Time       `json:"last_usage_at,omitzero"`
	LastFileEditAt       time.Time       `json:"last_file_edit_at,omitzero"`
	MinutesSinceFileEdit *int            `json:"minutes_since_file_edit,omitempty"`
	Activity             string          `json:"activity"`
	Flag                 bool            `json:"flag"`
	RequestDurationMS    float64         `json:"request_duration_ms,omitempty"`
}

type AIUsage struct {
	ID                    string    `json:"id"`
	Session               string    `json:"session"`
	Kind                  string    `json:"kind"`
	Model                 string    `json:"model,omitempty"`
	Project               string    `json:"project,omitempty"`
	At                    time.Time `json:"at"`
	Tokens                Tokens    `json:"tokens"`
	CostUSD               float64   `json:"cost_usd,omitempty"`
	Method                string    `json:"method,omitempty"`
	Source                string    `json:"source"`
	RequestDurationMS     float64   `json:"request_duration_ms,omitempty"`
	TimeToFirstTokenMS    float64   `json:"time_to_first_token_ms,omitempty"`
	OutputTokensPerSecond float64   `json:"output_tokens_per_second,omitempty"`
	SpeedMethod           string    `json:"speed_method,omitempty"`
	APIError              bool      `json:"api_error,omitempty"`
	Retry                 bool      `json:"retry,omitempty"`
}

type Quota struct {
	Kind          string    `json:"kind"`
	Window        string    `json:"window"`
	WindowMinutes int64     `json:"window_minutes,omitempty"`
	Used          float64   `json:"used_percent"`
	ResetsAt      time.Time `json:"resets_at"`
	Projected     time.Time `json:"projected_at,omitempty"`
	Method        string    `json:"method"`
	Source        string    `json:"source,omitempty"`
	ObservedAt    time.Time `json:"observed_at"`
}

type AI struct {
	Sessions []AgentSession `json:"sessions"`
	Usage    []AIUsage      `json:"usage"`
	Quotas   []Quota        `json:"quotas"`
}
