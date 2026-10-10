package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type Root struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}
type cursor struct {
	Inode  uint64
	Offset int64
	Skip   bool
}
type LogReader struct {
	mu             sync.Mutex
	offsets        map[string]cursor
	seen           map[string]model.AIUsage
	fileSession    map[string]string
	fileProject    map[string]string
	fileWorktree   map[string]string
	fileModel      map[string]string
	latestQuotas   map[string]model.Quota
	latestSessions map[string]model.AgentSession
	messageStart   map[string]time.Time
	messageLast    map[string]time.Time
	knownProjects  map[string][]string
	Roots          []Root
	WorktreeRoots  []string
	ByteLimit      int64
	TimeLimit      time.Duration
}

func NewLogReader(roots []Root) *LogReader {
	known := map[string][]string{}
	for _, root := range roots {
		home := filepath.Dir(filepath.Dir(root.Path))
		for _, dir := range []string{filepath.Join(home, "projects"), filepath.Join(home, "orca", "workspaces")} {
			entries, e := os.ReadDir(dir)
			if e != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					known[root.Path] = append(known[root.Path], entry.Name())
				}
			}
		}
		sort.Slice(known[root.Path], func(i, j int) bool { return len(known[root.Path][i]) > len(known[root.Path][j]) })
	}
	return &LogReader{offsets: map[string]cursor{}, seen: map[string]model.AIUsage{}, fileSession: map[string]string{}, fileProject: map[string]string{}, fileWorktree: map[string]string{}, fileModel: map[string]string{}, latestQuotas: map[string]model.Quota{}, latestSessions: map[string]model.AgentSession{}, messageStart: map[string]time.Time{}, messageLast: map[string]time.Time{}, knownProjects: known, Roots: roots, ByteLimit: 1 << 20, TimeLimit: 4 * time.Second}
}
func safePath(p string) bool {
	p = filepath.Clean(p)
	for {
		info, e := os.Lstat(p)
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		q := filepath.Dir(p)
		if q == p {
			return true
		}
		p = q
	}
}

type sessionFile struct {
	path  string
	entry os.DirEntry
	mtime time.Time
}

func orderedFiles(root string) []sessionFile {
	files := []sessionFile{}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		info, e := d.Info()
		if e == nil && info.Mode().IsRegular() {
			files = append(files, sessionFile{path, d, info.ModTime()})
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.After(files[j].mtime) })
	return files
}
func (r *LogReader) Scan(now time.Time) model.AI {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := model.AI{Sessions: []model.AgentSession{}, Usage: []model.AIUsage{}, Quotas: []model.Quota{}}
	sessions := map[string]model.AgentSession{}
	deadline := time.Now().Add(r.TimeLimit)
	remaining := r.ByteLimit
	if remaining <= 0 {
		remaining = 1 << 20
	}
	for _, root := range r.Roots {
		remaining = r.ByteLimit
		if remaining <= 0 {
			remaining = 1 << 20
		}
		if (root.Kind != "claude" && root.Kind != "codex") || !safePath(root.Path) {
			continue
		}
		process := func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if time.Now().After(deadline) || remaining <= 0 || len(out.Usage) >= 2000 {
				return filepath.SkipAll
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jsonl") || !d.Type().IsRegular() {
				return nil
			}
			f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
			if e != nil {
				return nil
			}
			defer f.Close()
			info, e := f.Stat()
			if e != nil || !info.Mode().IsRegular() {
				return nil
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return nil
			}
			old := r.offsets[path]
			if old.Inode != stat.Ino || old.Offset > info.Size() {
				old = cursor{Inode: stat.Ino}
			}
			if info.ModTime().Before(now.Add(-8*24*time.Hour)) && old.Offset == 0 {
				return nil
			}
			f.Seek(old.Offset, io.SeekStart)
			br := bufio.NewReaderSize(f, 64<<10)
			pos := old.Offset
			skip := old.Skip
			for remaining > 0 && time.Now().Before(deadline) && len(out.Usage) < 2000 {
				line, consumed, complete, oversized := boundedLine(br, remaining)
				remaining -= consumed
				if skip {
					pos += consumed
					if complete {
						skip = false
					}
					if !complete {
						break
					}
					continue
				}
				if !complete {
					if oversized {
						pos += consumed
						skip = true
					}
					break
				}
				pos += consumed
				if oversized {
					continue
				}
				var event map[string]json.RawMessage
				if json.Unmarshal(line, &event) != nil {
					continue
				}
				var ts string
				json.Unmarshal(event["timestamp"], &ts)
				at, err := time.Parse(time.RFC3339Nano, ts)
				if err != nil || at.After(now.Add(5*time.Minute)) || at.Before(now.Add(-7*24*time.Hour)) {
					continue
				}
				if root.Kind == "codex" {
					var meta struct {
						Type    string `json:"type"`
						Payload struct {
							ID    string `json:"id"`
							CWD   string `json:"cwd"`
							Model string `json:"model"`
						} `json:"payload"`
					}
					if json.Unmarshal(line, &meta) == nil && meta.Type == "session_meta" && meta.Payload.ID != "" {
						r.fileSession[path] = Hash(meta.Payload.ID)
						if filepath.IsAbs(meta.Payload.CWD) {
							project, worktree := projectForRoots(meta.Payload.CWD, r.WorktreeRoots)
							r.fileProject[path] = project
							r.fileWorktree[path] = worktree
						}
					}
					if meta.Type == "session_meta" || meta.Type == "turn_context" {
						if model := bounded(meta.Payload.Model); model != "" {
							r.fileModel[path] = model
						}
					}
				}
				sid := sessionFromEvent(event, path)
				session := Hash(sid)
				if sid == path && r.fileSession[path] != "" {
					session = r.fileSession[path]
				}
				project := projectFromLog(root.Path, path, r.knownProjects[root.Path])
				if root.Kind == "codex" {
					project = r.fileProject[path]
					if project == "" {
						project = "unknown"
					}
				}
				var eventType string
				json.Unmarshal(event["type"], &eventType)
				ended := false
				if root.Kind == "claude" && eventType == "assistant" {
					var m struct {
						StopReason string `json:"stop_reason"`
					}
					json.Unmarshal(event["message"], &m)
					ended = m.StopReason == "end_turn"
				}
				if root.Kind == "codex" {
					var p struct {
						Type string `json:"type"`
					}
					json.Unmarshal(event["payload"], &p)
					ended = p.Type == "turn_complete"
				}
				if old, ok := sessions[session]; !ok || !at.Before(old.At) {
					modelName := ""
					if root.Kind == "codex" {
						modelName = r.fileModel[path]
					}
					started := at
					if old, ok := sessions[session]; ok && !old.StartedAt.IsZero() && old.StartedAt.Before(started) {
						started = old.StartedAt
					}
					if old, ok := r.latestSessions[session]; ok && !old.StartedAt.IsZero() && old.StartedAt.Before(started) {
						started = old.StartedAt
					}
					sessions[session] = model.AgentSession{ID: session, Kind: root.Kind, Model: modelName, Project: project, Worktree: r.fileWorktree[path], At: at, StartedAt: started, State: "working", Reason: "recent log activity", Source: "log", LastTurnEnded: ended}
				}
				if root.Kind == "claude" {
					var msg struct {
						ID    string `json:"id"`
						Model string `json:"model"`
						Usage struct {
							Input  int64 `json:"input_tokens"`
							Output int64 `json:"output_tokens"`
							Write  int64 `json:"cache_creation_input_tokens"`
							Read   int64 `json:"cache_read_input_tokens"`
						} `json:"usage"`
					}
					if json.Unmarshal(event["message"], &msg) != nil || msg.ID == "" {
						continue
					}
					u := model.AIUsage{ID: Hash("claude:" + session + ":" + msg.ID), Session: session, Kind: "claude", Model: bounded(msg.Model), Project: project, At: at, Source: "log", Tokens: model.Tokens{Input: positive(msg.Usage.Input), Output: positive(msg.Usage.Output), CacheWrite: positive(msg.Usage.Write), CacheRead: positive(msg.Usage.Read)}}
					prev := r.seen[u.ID]
					if !prev.At.IsZero() && at.After(prev.At) && u.Tokens.Output > prev.Tokens.Output {
						u.OutputTokensPerSecond = float64(u.Tokens.Output-prev.Tokens.Output) / at.Sub(prev.At).Seconds()
						u.SpeedMethod = "estimated"
					}
					u.Tokens.Input = max(u.Tokens.Input, prev.Tokens.Input)
					u.Tokens.Output = max(u.Tokens.Output, prev.Tokens.Output)
					u.Tokens.CacheWrite = max(u.Tokens.CacheWrite, prev.Tokens.CacheWrite)
					u.Tokens.CacheRead = max(u.Tokens.CacheRead, prev.Tokens.CacheRead)
					r.seen[u.ID] = u
					out.Usage = append(out.Usage, u)
				} else {
					var payload struct {
						Type string `json:"type"`
						Info struct {
							Last struct {
								Input  int64 `json:"input_tokens"`
								Cached int64 `json:"cached_input_tokens"`
								Output int64 `json:"output_tokens"`
							} `json:"last_token_usage"`
						} `json:"info"`
						RateLimits struct {
							LimitID   string  `json:"limit_id"`
							Primary   *window `json:"primary"`
							Secondary *window `json:"secondary"`
						} `json:"rate_limits"`
					}
					json.Unmarshal(event["payload"], &payload)
					if payload.Type == "agent_message" {
						if r.messageStart[session].IsZero() {
							r.messageStart[session] = at
						}
						r.messageLast[session] = at
					}
					if payload.Type == "token_count" && (payload.Info.Last.Input > 0 || payload.Info.Last.Output > 0) {
						u := model.AIUsage{ID: Hash("codex:" + session + ":" + at.Format(time.RFC3339Nano)), Session: session, Kind: "codex", Model: r.fileModel[path], Project: project, At: at, Source: "log", Tokens: model.Tokens{Input: positive(payload.Info.Last.Input - payload.Info.Last.Cached), Output: positive(payload.Info.Last.Output), CacheRead: positive(payload.Info.Last.Cached)}}
						if first, last := r.messageStart[session], r.messageLast[session]; !first.IsZero() && last.After(first) && u.Tokens.Output > 0 {
							u.OutputTokensPerSecond = float64(u.Tokens.Output) / last.Sub(first).Seconds()
							u.SpeedMethod = "estimated"
						}
						delete(r.messageStart, session)
						delete(r.messageLast, session)
						out.Usage = append(out.Usage, u)
					}
					if payload.RateLimits.LimitID == "" || payload.RateLimits.LimitID == "codex" {
						for _, w := range []struct {
							Name  string
							Value *window
						}{{"primary", payload.RateLimits.Primary}, {"secondary", payload.RateLimits.Secondary}} {
							if w.Value != nil && w.Value.WindowMinutes > 0 && w.Value.UsedPercent >= 0 && w.Value.UsedPercent <= 100 {
								q := model.Quota{Kind: "codex", Window: w.Name, WindowMinutes: w.Value.WindowMinutes, Used: w.Value.UsedPercent, ResetsAt: time.Unix(w.Value.ResetsAt, 0).UTC(), ObservedAt: at, Method: "cli_reported"}
								q.Projected = QuotaProjection(q, w.Value.WindowMinutes)
								key := q.Kind + ":" + q.Window
								if q.ObservedAt.After(r.latestQuotas[key].ObservedAt) {
									r.latestQuotas[key] = q
								}
								out.Quotas = append(out.Quotas, q)
							}
						}
					}
				}
			}
			r.offsets[path] = cursor{Inode: stat.Ino, Offset: pos, Skip: skip}
			return nil
		}
		files := orderedFiles(root.Path)
		if root.Kind == "codex" {
			for i, file := range files {
				if i >= 3 {
					break
				}
				r.quotaTail(file.path, now)
			}
		}
		for _, file := range files {
			if process(file.path, file.entry, nil) == filepath.SkipAll {
				break
			}
		}
	}
	for id, u := range r.seen {
		if u.At.Before(now.Add(-7 * 24 * time.Hour)) {
			delete(r.seen, id)
		}
	}
	if len(r.seen) > 40000 {
		ids := make([]string, 0, len(r.seen))
		for id := range r.seen {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return r.seen[ids[i]].At.After(r.seen[ids[j]].At) })
		for _, id := range ids[40000:] {
			delete(r.seen, id)
		}
	}
	for id, session := range sessions {
		r.latestSessions[id] = session
	}
	for id, session := range r.latestSessions {
		if now.Sub(session.At) > 24*time.Hour {
			delete(r.latestSessions, id)
			continue
		}
		out.Sessions = append(out.Sessions, session)
	}
	out.Quotas = out.Quotas[:0]
	for key, q := range r.latestQuotas {
		if !q.ResetsAt.After(now) {
			delete(r.latestQuotas, key)
			continue
		}
		out.Quotas = append(out.Quotas, q)
	}
	return out
}

func boundedLine(r *bufio.Reader, budget int64) ([]byte, int64, bool, bool) {
	var b bytes.Buffer
	var used int64
	tooLarge := false
	for used < budget {
		part, err := r.ReadSlice('\n')
		used += int64(len(part))
		if (used >= 1<<20 && err != nil) || used > 1<<20 {
			tooLarge = true
		} else if !tooLarge {
			b.Write(part)
		}
		if err == nil {
			return b.Bytes(), used, true, tooLarge
		}
		if err == io.EOF {
			return nil, used, false, tooLarge
		}
		if err != bufio.ErrBufferFull {
			return nil, used, false, tooLarge
		}
	}
	return nil, used, false, tooLarge
}

type window struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int64   `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"`
}

func positive(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
func bounded(s string) string {
	return safeTag(s)
}
func sessionFromEvent(e map[string]json.RawMessage, path string) string {
	var s string
	json.Unmarshal(e["sessionId"], &s)
	if s == "" {
		json.Unmarshal(e["session_id"], &s)
	}
	if s == "" {
		s = path
	}
	return s
}
func projectFromLog(root, path string, known []string) string {
	rel, e := filepath.Rel(root, path)
	if e != nil {
		return "unknown"
	}
	p := strings.Split(rel, string(filepath.Separator))[0]
	if p == "" || p == "." {
		return "unknown"
	}
	homePrefix := strings.ReplaceAll(filepath.Dir(filepath.Dir(root)), string(filepath.Separator), "-")
	if strings.HasPrefix(p, homePrefix+"-orca-workspaces-") {
		tail := strings.TrimPrefix(p, homePrefix+"-orca-workspaces-")
		match := ""
		for _, name := range known {
			if (tail == name || strings.HasPrefix(tail, name+"-")) && len(name) > len(match) {
				match = name
			}
		}
		if match != "" {
			return match
		}
		parts := strings.Split(tail, "-")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	if strings.HasPrefix(p, homePrefix+"-projects-") {
		return strings.TrimPrefix(p, homePrefix+"-projects-")
	}
	if strings.HasPrefix(p, homePrefix+"-") {
		tail := strings.TrimPrefix(p, homePrefix+"-")
		if !strings.Contains(tail, "-") {
			return tail
		}
	}
	if len(p) > 100 {
		return "unknown"
	}
	return p
}
func QuotaProjection(q model.Quota, minutes int64) time.Time {
	start := q.ResetsAt.Add(-time.Duration(minutes) * time.Minute)
	elapsed := q.ObservedAt.Sub(start)
	if elapsed <= 0 || !q.ObservedAt.Before(q.ResetsAt) || q.Used <= 0 {
		return time.Time{}
	}
	return start.Add(time.Duration(float64(elapsed) * 100 / q.Used))
}

func (r *LogReader) quotaTail(path string, now time.Time) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return
	}
	start := info.Size() - (256 << 10)
	if start < 0 {
		start = 0
	}
	f.Seek(start, io.SeekStart)
	br := bufio.NewReader(f)
	if start > 0 {
		br.ReadBytes('\n')
	}
	for {
		line, e := br.ReadBytes('\n')
		if e != nil {
			break
		}
		if len(line) > 1<<20 {
			continue
		}
		var event struct {
			Timestamp string `json:"timestamp"`
			Payload   struct {
				RateLimits struct {
					LimitID   string  `json:"limit_id"`
					Primary   *window `json:"primary"`
					Secondary *window `json:"secondary"`
				} `json:"rate_limits"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil || at.After(now.Add(5*time.Minute)) || at.Before(now.Add(-24*time.Hour)) {
			continue
		}
		for _, w := range []struct {
			Name  string
			Value *window
		}{{"primary", event.Payload.RateLimits.Primary}, {"secondary", event.Payload.RateLimits.Secondary}} {
			if w.Value == nil || w.Value.WindowMinutes <= 0 || w.Value.UsedPercent < 0 || w.Value.UsedPercent > 100 {
				continue
			}
			q := model.Quota{Kind: "codex", Window: w.Name, WindowMinutes: w.Value.WindowMinutes, Used: w.Value.UsedPercent, ResetsAt: time.Unix(w.Value.ResetsAt, 0).UTC(), ObservedAt: at, Method: "cli_reported"}
			q.Projected = QuotaProjection(q, w.Value.WindowMinutes)
			key := q.Kind + ":" + q.Window
			if q.ObservedAt.After(r.latestQuotas[key].ObservedAt) {
				r.latestQuotas[key] = q
			}
		}
	}
}
