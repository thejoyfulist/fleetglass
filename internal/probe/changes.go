package probe

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

var aptPackage = regexp.MustCompile(`([A-Za-z0-9][A-Za-z0-9+.-]*)(?::[A-Za-z0-9-]+)?\s*\(`)
var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

func NewChange(kind, source, text string, at time.Time) model.Change {
	sum := sha256.Sum256([]byte(kind + "\x00" + source + "\x00" + at.UTC().Format(time.RFC3339Nano)))
	if len(text) > 160 {
		text = text[:160]
	}
	c := model.Change{ID: hex.EncodeToString(sum[:]), At: at.UTC(), Type: kind, Text: text}
	model.AnnotateChange(&c)
	return c
}

// APTChanges reads bounded local history, including rotations. It emits package names only.
func APTChanges(root string, now time.Time) []model.Change {
	paths := []string{filepath.Join(root, "history.log"), filepath.Join(root, "history.log.1")}
	gz, _ := filepath.Glob(filepath.Join(root, "history.log.*.gz"))
	if len(gz) > 8 {
		gz = gz[:8]
	}
	paths = append(paths, gz...)
	out := []model.Change{}
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			continue
		}
		var reader io.Reader = f
		if strings.HasSuffix(path, ".gz") {
			z, e := gzip.NewReader(f)
			if e != nil {
				f.Close()
				continue
			}
			defer z.Close()
			reader = z
		}
		b, _ := io.ReadAll(io.LimitReader(reader, 2<<20))
		f.Close()
		for _, block := range strings.Split(string(b), "\n\n") {
			var at time.Time
			var names []string
			for _, line := range strings.Split(block, "\n") {
				if s, ok := strings.CutPrefix(line, "Start-Date:"); ok {
					at, _ = time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(s), time.Local)
				}
				if s, ok := strings.CutPrefix(line, "Upgrade:"); ok {
					for _, m := range aptPackage.FindAllStringSubmatch(s, 64) {
						names = append(names, m[1])
					}
				}
			}
			if at.IsZero() || at.After(now) || now.Sub(at) > 7*24*time.Hour || len(names) == 0 {
				continue
			}
			shown := names
			if len(shown) > 6 {
				shown = shown[:6]
			}
			label := "Upgraded " + strconv.Itoa(len(names)) + " packages: " + strings.Join(shown, ", ")
			if len(names) > 6 {
				label += ", +" + strconv.Itoa(len(names)-6)
			}
			out = append(out, NewChange("APT", strings.Join(names, ","), label, at))
		}
	}
	return out
}

// JournalChanges accepts JSON fixtures or journalctl output and selects only fixed event labels.
func JournalChanges(input []byte, units []string, now time.Time) []model.Change {
	allowed := map[string]bool{}
	for _, u := range units {
		allowed[u] = true
	}
	out := []model.Change{}
	scan := bufio.NewScanner(strings.NewReader(string(input)))
	scan.Buffer(make([]byte, 4096), 1<<20)
	for scan.Scan() {
		var row struct {
			Unit        string `json:"_SYSTEMD_UNIT"`
			ManagerUnit string `json:"UNIT"`
			Message     string `json:"MESSAGE"`
			Timestamp   string `json:"__REALTIME_TIMESTAMP"`
		}
		if json.Unmarshal(scan.Bytes(), &row) != nil {
			continue
		}
		if !allowed[row.Unit] {
			row.Unit = row.ManagerUnit
		}
		if !allowed[row.Unit] {
			continue
		}
		us, e := strconv.ParseInt(row.Timestamp, 10, 64)
		if e != nil {
			continue
		}
		at := time.UnixMicro(us)
		if at.After(now) || now.Sub(at) > 7*24*time.Hour {
			continue
		}
		verb := ""
		if strings.HasPrefix(row.Message, "Started ") {
			verb = "restarted"
		}
		if strings.HasPrefix(row.Message, "Failed ") || strings.Contains(row.Message, "Failed with result") {
			verb = "failed"
		}
		if verb != "" {
			out = append(out, NewChange("Service", row.Unit+verb, row.Unit+" "+verb, at))
		}
	}
	return out
}

func BootChange(procRoot string, now time.Time) []model.Change {
	f, e := os.Open(filepath.Join(procRoot, "stat"))
	if e != nil {
		return nil
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		if s, ok := strings.CutPrefix(scan.Text(), "btime "); ok {
			n, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if e == nil {
				at := time.Unix(n, 0)
				if !at.After(now) && now.Sub(at) <= 7*24*time.Hour {
					return []model.Change{NewChange("Boot", s, "Machine restarted", at)}
				}
			}
			break
		}
	}
	return nil
}

// MergeChanges uses only first-parent merge commits on a locally resolved default branch.
func MergeChanges(ctx context.Context, roots, excluded []string, now time.Time, runner Runner) []model.Change {
	if runner == nil {
		runner = Exec
	}
	out := []model.Change{}
	for _, root := range roots {
		entries, e := os.ReadDir(root)
		if e != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !safeName.MatchString(entry.Name()) {
				continue
			}
			skip := false
			for _, x := range excluded {
				if x == entry.Name() {
					skip = true
				}
			}
			if skip {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if _, e := os.Lstat(filepath.Join(path, ".git")); e != nil {
				continue
			}
			run := func(args ...string) (string, error) {
				b, e := runner(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "safe.directory=" + path, "-C", path}, args...)...)
				return strings.TrimSpace(string(b)), e
			}
			branch := ""
			for _, candidate := range []string{"refs/remotes/origin/HEAD", "refs/heads/main", "refs/heads/master"} {
				v, e := run("symbolic-ref", "--quiet", "--short", candidate)
				if e == nil && candidate == "refs/remotes/origin/HEAD" {
					branch = v
					break
				}
				if candidate != "refs/remotes/origin/HEAD" {
					_, e = run("rev-parse", "--verify", candidate)
					if e == nil {
						branch = candidate
						break
					}
				}
			}
			if branch == "" {
				continue
			}
			raw, e := run("log", "--first-parent", "--merges", "--since=7.days", "--format=%H%x09%ct", "-n", "20", branch)
			if e != nil {
				continue
			}
			for _, line := range strings.Split(raw, "\n") {
				parts := strings.Split(line, "\t")
				if len(parts) != 2 || len(parts[0]) != 40 && len(parts[0]) != 64 {
					continue
				}
				sec, e := strconv.ParseInt(parts[1], 10, 64)
				if e != nil {
					continue
				}
				at := time.Unix(sec, 0)
				if at.After(now) || now.Sub(at) > 7*24*time.Hour {
					continue
				}
				out = append(out, NewChange("Git", entry.Name()+parts[0], "Merged into "+entry.Name(), at))
			}
		}
	}
	return out
}

// MarkerChanges never includes a raw log line, even when the line contains secrets.
func MarkerChanges(marker config.LogMarker, now time.Time) []model.Change {
	if !marker.ChangeEvent {
		return nil
	}
	f, e := openRegular(marker.Path)
	if e != nil {
		return nil
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil
	}
	start := info.Size() - 256*1024
	if start < 0 {
		start = 0
	}
	if _, e = f.Seek(start, io.SeekStart); e != nil {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(f, 256*1024))
	out := []model.Change{}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, marker.Contains) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		at, e := time.Parse(time.RFC3339, fields[0])
		if e != nil || at.After(now.Add(5*time.Minute)) || now.Sub(at) > time.Duration(marker.MaxAgeSeconds)*time.Second {
			continue
		}
		label := marker.DisplayName
		if label == "" {
			label = strings.ReplaceAll(marker.Name, "_", " ")
		}
		out = append(out, NewChange("Marker", marker.Name, label, at))
	}
	return out
}
