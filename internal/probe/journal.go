package probe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

var secretPair = regexp.MustCompile(`(?i)\b(?:[a-z0-9_]*(?:token|secret|password|passwd|api[_-]?key|credential)[a-z0-9_]*)\s*[:=]\s*\S+`)
var bearer = regexp.MustCompile(`(?i)\bbearer\s+\S+`)
var credentialURL = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s/@]+:[^\s/@]+@\S+`)
var publicIP = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)
var longRun = regexp.MustCompile(`\b(?:[a-fA-F0-9]{32,}|[A-Za-z0-9+/=_-]{40,})\b`)
var oomProcess = regexp.MustCompile(`(?i)(?:Killed process\s+\d+\s+\(|task=)([a-zA-Z0-9_.-]{1,64})`)
var acceptedSSH = regexp.MustCompile(`^Accepted (?:publickey|password|keyboard-interactive/pam) for ([a-zA-Z0-9_.-]{1,64}) from (\S+)`)
var failedSSH = regexp.MustCompile(`^Failed (?:password|publickey) for (?:invalid user )?\S+ from (\S+)`)
var sudoUser = regexp.MustCompile(`^([a-zA-Z0-9_.-]{1,64})\s+:`)

type journalEntry struct {
	Message    string `json:"MESSAGE"`
	Unit       string `json:"_SYSTEMD_UNIT"`
	Identifier string `json:"SYSLOG_IDENTIFIER"`
	Transport  string `json:"_TRANSPORT"`
	Timestamp  string `json:"__REALTIME_TIMESTAMP"`
	Priority   string `json:"PRIORITY"`
}

func cleanLog(s string) string {
	s = credentialURL.ReplaceAllString(s, "[redacted URL]")
	s = bearer.ReplaceAllString(s, "[redacted bearer]")
	s = secretPair.ReplaceAllString(s, "[redacted secret]")
	s = longRun.ReplaceAllString(s, "[redacted value]")
	s = publicIP.ReplaceAllStringFunc(s, func(raw string) string {
		if sourceClass(raw) == "public" {
			return "[public IP]"
		}
		return raw
	})
	s = strings.ToValidUTF8(strings.TrimSpace(s), "")
	if utf8.RuneCountInString(s) > 160 {
		s = string([]rune(s)[:160])
	}
	return s
}
func parseJournal(data []byte) []journalEntry {
	entries := []journalEntry{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var e journalEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}
func sourceClass(raw string) string {
	ip := net.ParseIP(strings.Trim(raw, "[]"))
	if ip == nil {
		return "unknown"
	}
	if ip.IsLoopback() {
		return "loopback"
	}
	if ip.IsPrivate() {
		return "private"
	}
	if v := ip.To4(); v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127 {
		return "tailnet"
	}
	if v := ip.To16(); v != nil && strings.HasPrefix(strings.ToLower(ip.String()), "fd7a:115c:a1e0:") {
		return "tailnet"
	}
	return "public"
}
func journalTime(e journalEntry) int64 { n, _ := strconv.ParseInt(e.Timestamp, 10, 64); return n }

// JournalErrors accepts recorded journalctl -o json output. It never retains raw messages.
func JournalErrors(data []byte) []model.Result {
	type group struct {
		count   int
		last    int64
		message string
	}
	groups := map[string]group{}
	oom := map[string]int{}
	count := 0
	for _, e := range parseJournal(data) {
		p, err := strconv.Atoi(e.Priority)
		if err != nil || p > 3 {
			continue
		}
		unit := e.Unit
		if unit == "" {
			unit = e.Identifier
		}
		if unit == "" {
			unit = "kernel"
		}
		unit = cleanLog(unit)
		if unit == "" {
			unit = "unknown"
		}
		g := groups[unit]
		g.count++
		count++
		if t := journalTime(e); t >= g.last {
			g.last = t
			g.message = cleanLog(e.Message)
		}
		groups[unit] = g
		if e.Transport == "kernel" && (strings.Contains(e.Message, "Out of memory:") || strings.Contains(e.Message, "oom-kill:") || strings.Contains(e.Message, "Killed process")) {
			victim := "unknown"
			if m := oomProcess.FindStringSubmatch(e.Message); len(m) > 1 {
				victim = m[1]
			}
			oom[victim]++
		}
	}
	out := []model.Result{{Name: "recent_errors", State: model.OK, Detail: "journal, last 24 h, priority err and above", Values: map[string]float64{"count": float64(count), "oom_count": 0}}}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		g := groups[k]
		out = append(out, model.Result{Name: fmt.Sprintf("recent_error:%d", i), State: model.Warn, Detail: "journal error group", Values: map[string]float64{"count": float64(g.count), "last_unix_us": float64(g.last)}, Fields: map[string]string{"unit": k, "message": g.message}})
	}
	keys = keys[:0]
	for k := range oom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		out = append(out, model.Result{Name: fmt.Sprintf("oom:%d", i), State: model.Crit, Detail: "kernel out-of-memory kill", Values: map[string]float64{"count": float64(oom[k])}, Fields: map[string]string{"process": k}})
		out[0].Values["oom_count"] += float64(oom[k])
	}
	if count > 0 {
		out[0].State = model.Warn
	}
	return out
}

// JournalSecurity returns only counts, account names and source classes.
func JournalSecurity(data []byte) []model.Result {
	loginCount, sudoCount, failedCount := 0, 0, 0
	lastLogin, lastClass, lastTime := "", "", int64(0)
	sudoUsers := map[string]int{}
	failedClasses := map[string]int{}
	for _, e := range parseJournal(data) {
		msg := e.Message
		if e.Identifier == "sshd" || strings.HasPrefix(e.Unit, "ssh") {
			if m := acceptedSSH.FindStringSubmatch(msg); len(m) == 3 {
				loginCount++
				if t := journalTime(e); t >= lastTime {
					lastTime = t
					lastLogin = m[1]
					lastClass = sourceClass(m[2])
				}
			}
			if m := failedSSH.FindStringSubmatch(msg); len(m) == 2 {
				failedCount++
				failedClasses[sourceClass(m[1])]++
			}
		}
		if e.Identifier == "sudo" {
			if m := sudoUser.FindStringSubmatch(msg); len(m) == 2 {
				sudoCount++
				sudoUsers[m[1]]++
			}
		}
	}
	out := []model.Result{
		{Name: "security:ssh_logins", State: model.OK, Detail: "SSH logins, last 24 h", Values: map[string]float64{"count": float64(loginCount)}, Fields: map[string]string{"last_user": lastLogin, "source_class": lastClass}},
		{Name: "security:sudo", State: model.OK, Detail: "sudo use, last 24 h", Values: map[string]float64{"count": float64(sudoCount)}},
		{Name: "security:failed_logins", State: model.OK, Detail: "failed SSH logins, last 24 h", Values: map[string]float64{"count": float64(failedCount)}},
	}
	if failedCount > 0 {
		out[2].State = model.Warn
	}
	keys := make([]string, 0, len(sudoUsers))
	for k := range sudoUsers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		out = append(out, model.Result{Name: fmt.Sprintf("security:sudo_user:%d", i), State: model.OK, Detail: "sudo user", Values: map[string]float64{"count": float64(sudoUsers[k])}, Fields: map[string]string{"user": k}})
	}
	keys = keys[:0]
	for k := range failedClasses {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		out = append(out, model.Result{Name: fmt.Sprintf("security:failed_source:%d", i), State: model.Warn, Detail: "failed login source class", Values: map[string]float64{"count": float64(failedClasses[k])}, Fields: map[string]string{"source_class": k}})
	}
	return out
}
func Journal(ctx context.Context, run Runner) []model.Result {
	if run == nil {
		run = Exec
	}
	errors, e := run(ctx, "journalctl", "--system", "--since", "24 hours ago", "-p", "err", "-o", "json", "--no-pager")
	out := []model.Result{}
	if e != nil {
		out = append(out, model.Result{Name: "recent_errors", State: model.Unknown, Detail: "journal unreadable"})
	} else {
		out = append(out, JournalErrors(errors)...)
	}
	auth, e := run(ctx, "journalctl", "--system", "--since", "24 hours ago", "-o", "json", "--no-pager", "-t", "sshd", "-t", "sudo")
	if len(bytes.TrimSpace(errors)) == 0 && len(bytes.TrimSpace(auth)) == 0 && e == nil {
		baseline, baselineErr := run(ctx, "journalctl", "--system", "-n", "1", "-o", "json", "--no-pager")
		if baselineErr != nil || len(bytes.TrimSpace(baseline)) == 0 {
			return []model.Result{
				{Name: "recent_errors", State: model.Unknown, Detail: "system journal unreadable"},
				{Name: "security:ssh_logins", State: model.Unknown, Detail: "system journal unreadable"},
				{Name: "security:sudo", State: model.Unknown, Detail: "system journal unreadable"},
				{Name: "security:failed_logins", State: model.Unknown, Detail: "system journal unreadable"},
			}
		}
	}
	if e != nil {
		for _, name := range []string{"security:ssh_logins", "security:sudo", "security:failed_logins"} {
			out = append(out, model.Result{Name: name, State: model.Unknown, Detail: "journal unreadable"})
		}
	} else {
		out = append(out, JournalSecurity(auth)...)
	}
	return out
}
