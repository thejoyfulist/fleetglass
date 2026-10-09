package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

// Backup reads metadata only. A directory pattern includes its regular direct children;
// a glob selects matching regular files. Neither path nor file names enter the result.
func Backup(set config.BackupSet, now time.Time) model.Result {
	r := model.Result{Name: "backup:" + set.Name, State: model.Unknown, Detail: "backup source unreadable"}
	paths, err := filepath.Glob(set.Pattern)
	if err != nil {
		return r
	}
	if strings.ContainsAny(set.Pattern, "*?[") {
		if _, e := os.ReadDir(filepath.Dir(set.Pattern)); e != nil {
			return r
		}
	}
	if len(paths) == 1 && paths[0] == set.Pattern {
		info, e := os.Lstat(set.Pattern)
		if e != nil {
			if os.IsNotExist(e) {
				r.State = model.Crit
				r.Detail = "no backup"
				r.Values = map[string]float64{"count": 0}
			}
			return r
		}
		if info.IsDir() {
			entries, e := os.ReadDir(set.Pattern)
			if e != nil {
				return r
			}
			paths = nil
			for _, entry := range entries {
				paths = append(paths, filepath.Join(set.Pattern, entry.Name()))
			}
		}
	}
	var newest time.Time
	var size int64
	count := 0
	for _, path := range paths {
		if len(set.Suffixes) > 0 {
			match := false
			for _, suffix := range set.Suffixes {
				if strings.HasSuffix(path, suffix) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		info, e := os.Lstat(path)
		if e != nil {
			return r
		}
		if !info.Mode().IsRegular() {
			continue
		}
		count++
		if info.ModTime().After(newest) {
			newest = info.ModTime()
			size = info.Size()
		}
	}
	r.Values = map[string]float64{"count": float64(count)}
	if count == 0 {
		r.State = model.Crit
		r.Detail = "no backup"
		return r
	}
	age := now.Sub(newest).Seconds()
	if age < 0 {
		age = 0
	}
	r.Values["age_seconds"] = age
	r.Values["size_bytes"] = float64(size)
	r.State = model.OK
	r.Detail = "latest backup within limits"
	if set.WarnAgeSeconds > 0 && age > float64(set.WarnAgeSeconds) {
		r.State = model.Warn
		r.Detail = "latest backup getting old"
	}
	if age > float64(set.MaxAgeSeconds) || size < set.MinSizeBytes {
		r.State = model.Crit
		r.Detail = "latest backup outside limits"
	}
	return r
}

func Status(file config.StatusFile, now time.Time) model.Result {
	r := model.Result{Name: "status:" + file.Name, State: model.Unknown, Detail: "status file unreadable"}
	info, e := os.Lstat(file.Path)
	if e != nil {
		if os.IsNotExist(e) {
			r.State = file.MissingState
			if r.State == "" {
				r.State = model.Unknown
			}
			r.Detail = "status file missing"
		}
		return r
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		r.Detail = "invalid status file"
		return r
	}
	f, e := openRegular(file.Path)
	if e != nil {
		return r
	}
	defer f.Close()
	var data any
	if e = json.NewDecoder(f).Decode(&data); e != nil {
		r.Detail = "invalid status JSON"
		return r
	}
	current := data
	for _, part := range strings.Split(file.FieldPath, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			current = nil
			break
		}
		current = m[part]
	}
	if current == nil {
		r.State = file.MissingState
		if r.State == "" {
			r.State = model.Unknown
		}
		r.Detail = "field missing"
		return r
	}
	var raw string
	switch v := current.(type) {
	case string:
		raw = v
	case bool:
		raw = strconv.FormatBool(v)
	case float64:
		raw = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		r.Detail = "unsupported status value"
		return r
	}
	age := now.Sub(info.ModTime()).Seconds()
	if stamp, e := time.Parse(time.RFC3339, raw); e == nil {
		age = now.Sub(stamp).Seconds()
	}
	if age < -300 {
		r.Detail = "status timestamp in future"
		return r
	}
	if age < 0 {
		age = 0
	}
	r.Values = map[string]float64{"age_seconds": age}
	if age > float64(file.MaxAgeSeconds) {
		r.Detail = "status record too old"
		return r
	}
	for _, pair := range []struct {
		values []string
		state  model.State
	}{{file.OKValues, model.OK}, {file.WarnValues, model.Warn}, {file.CritValues, model.Crit}} {
		for _, v := range pair.values {
			if raw == v {
				r.State = pair.state
				r.Detail = "configured status value"
				return r
			}
		}
	}
	if _, e := time.Parse(time.RFC3339, raw); e == nil {
		r.State = model.OK
		r.Detail = "recent timestamp"
		return r
	}
	r.Detail = "unrecognized status value"
	return r
}

// Marker searches only a bounded log tail and returns counts and timestamps, never log text.
func Marker(marker config.LogMarker, now time.Time) model.Result {
	r := model.Result{Name: "log_marker:" + marker.Name, State: model.Unknown, Detail: "log unreadable"}
	label := marker.DisplayName
	if label == "" {
		label = strings.ReplaceAll(marker.Name, "_", " ")
	}
	r.Fields = map[string]string{"display_name": label}
	f, e := openRegular(marker.Path)
	if e != nil {
		return r
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return r
	}
	start := info.Size() - 256*1024
	if start < 0 {
		start = 0
	}
	if _, e = f.Seek(start, io.SeekStart); e != nil {
		return r
	}
	b, e := io.ReadAll(io.LimitReader(f, 256*1024))
	if e != nil {
		return r
	}
	var newest time.Time
	count := 0
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, marker.Contains) {
			continue
		}
		stamp := strings.Fields(line)
		if len(stamp) == 0 {
			continue
		}
		at, e := time.Parse(time.RFC3339, stamp[0])
		if e != nil {
			continue
		}
		if at.After(now.Add(5*time.Minute)) || now.Sub(at) > time.Duration(marker.MaxAgeSeconds)*time.Second {
			continue
		}
		count++
		if at.After(newest) {
			newest = at
		}
	}
	r.Values = map[string]float64{"count": float64(count)}
	if count == 0 {
		r.State = marker.AbsentState
		if r.State == "" {
			r.State = model.Warn
		}
		r.Detail = "no matching recent log entry"
		return r
	}
	age := now.Sub(newest).Seconds()
	if age < 0 {
		age = 0
	}
	r.Values["age_seconds"] = age
	r.State = marker.PresentState
	if r.State == "" {
		r.State = model.OK
	}
	r.Detail = "matching log entry observed"
	return r
}

func openRegular(path string) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), path), nil
}

func Repositories(ctx context.Context, roots []string, localOnly, excluded []string, now time.Time, runner Runner) []model.Result {
	if runner == nil {
		runner = Exec
	}
	out := []model.Result{}
	for rootIndex, root := range roots {
		entries, e := os.ReadDir(root)
		if e != nil {
			out = append(out, model.Result{Name: fmt.Sprintf("repositories:%d", rootIndex), State: model.Unknown, Detail: "repository root unreadable"})
			continue
		}
		parent := model.Result{Name: fmt.Sprintf("repositories:%d", rootIndex), State: model.OK, Detail: "repository root scanned", Values: map[string]float64{"count": 0}}
		out = append(out, parent)
		parentIndex := len(out) - 1
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			ignored := false
			for _, n := range excluded {
				if n == entry.Name() {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if _, e := os.Lstat(filepath.Join(path, ".git")); e != nil {
				continue
			}
			out[parentIndex].Values["count"]++
			name := fmt.Sprintf("repository:%d:%s", rootIndex, entry.Name())
			r := model.Result{Name: name, State: model.Unknown, Detail: "repository unreadable", Fields: map[string]string{"repository": entry.Name()}}
			// Disable hooks and optional fsmonitor; no fetch is performed.
			run := func(args ...string) (string, error) {
				b, e := runner(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "safe.directory=" + path, "-C", path}, args...)...)
				return strings.TrimSpace(string(b)), e
			}
			status, e := run("status", "--porcelain", "--untracked-files=normal")
			if e != nil {
				out = append(out, r)
				continue
			}
			dirtyCount := 0
			if status != "" {
				dirtyCount = len(strings.Split(status, "\n"))
			}
			dirty := dirtyCount > 0
			branch, e := run("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
			upstream := e == nil && branch != ""
			local := false
			for _, n := range localOnly {
				if n == entry.Name() {
					local = true
					break
				}
			}
			ahead, behind := 0, 0
			if upstream {
				counts, e := run("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
				if e != nil {
					out = append(out, r)
					continue
				}
				fmt.Sscanf(counts, "%d %d", &ahead, &behind)
			}
			stamp, e := run("log", "-1", "--format=%ct")
			if e != nil {
				out = append(out, r)
				continue
			}
			sec, e := strconv.ParseInt(stamp, 10, 64)
			if e != nil {
				out = append(out, r)
				continue
			}
			age := now.Sub(time.Unix(sec, 0)).Seconds()
			if age < 0 {
				age = 0
			}
			r.State = model.OK
			r.Detail = "clean repository"
			if dirty || ahead > 0 || behind > 0 || !upstream && !local {
				r.State = model.Warn
				parts := []string{}
				if dirty {
					word := "files"
					if dirtyCount == 1 {
						word = "file"
					}
					parts = append(parts, fmt.Sprintf("%d uncommitted %s", dirtyCount, word))
				}
				if ahead > 0 {
					word := "commits"
					if ahead == 1 {
						word = "commit"
					}
					parts = append(parts, fmt.Sprintf("%d %s not pushed", ahead, word))
				}
				if !upstream && !local {
					parts = append(parts, "no upstream")
				}
				if behind > 0 {
					parts = append(parts, fmt.Sprintf("%d behind", behind))
				}
				r.Detail = strings.Join(parts, ", ")
			}
			r.Values = map[string]float64{"dirty": 0, "dirty_count": float64(dirtyCount), "ahead": float64(ahead), "behind": float64(behind), "last_commit_age_seconds": age, "has_upstream": 0, "local_only": 0}
			if dirty {
				r.Values["dirty"] = 1
			}
			if upstream {
				r.Values["has_upstream"] = 1
			}
			if local {
				r.Values["local_only"] = 1
			}
			out = append(out, r)
		}
	}
	return out
}

func Internal(ctx context.Context, p config.InternalProbe) (r model.Result) {
	r = model.Result{Name: "internal:" + p.Name, State: model.Warn, Detail: "target unreachable"}
	label := p.DisplayName
	if label == "" {
		label = strings.ReplaceAll(p.Name, "_", " ")
	}
	r.Fields = map[string]string{"display_name": label}
	timeout := time.Duration(p.TimeoutMS) * time.Millisecond
	start := time.Now()
	defer func() {
		if r.Values == nil {
			r.Values = map[string]float64{}
		}
		r.Values["latency_ms"] = float64(time.Since(start).Microseconds()) / 1000
	}()
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if p.Kind == "tcp" {
		conn, e := (&net.Dialer{}).DialContext(c, "tcp", p.Target)
		if e != nil {
			return r
		}
		conn.Close()
		r.State = model.OK
		r.Detail = "TCP connect succeeded"
	} else {
		transport := &http.Transport{Proxy: nil}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		req, e := http.NewRequestWithContext(c, "GET", p.Target, nil)
		if e != nil {
			r.State = model.Unknown
			r.Detail = "invalid HTTP target"
			return r
		}
		resp, e := client.Do(req)
		if e != nil {
			return r
		}
		resp.Body.Close()
		r.Values = map[string]float64{"status_code": float64(resp.StatusCode)}
		r.State = model.OK
		r.Detail = "HTTP response received"
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			r.State = model.Warn
			r.Detail = "HTTP status outside accepted range"
		}
	}
	return r
}
