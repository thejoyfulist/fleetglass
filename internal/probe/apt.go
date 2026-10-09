package probe

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func paragraphs(path string, fields []string, visit func(map[string]string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	want := map[string]bool{}
	for _, k := range fields {
		want[k] = true
	}
	row := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(row) > 0 {
				visit(row)
				row = map[string]string{}
			}
			continue
		}
		if line[0] == ' ' {
			continue
		}
		key, val, ok := strings.Cut(line, ": ")
		if ok && want[key] {
			row[key] = val
		}
	}
	if len(row) > 0 {
		visit(row)
	}
	return scanner.Err()
}
func versionPart(v string) (int, string, string) {
	epoch := 0
	if head, tail, ok := strings.Cut(v, ":"); ok {
		fmt.Sscanf(head, "%d", &epoch)
		v = tail
	}
	upstream, revision, ok := strings.Cut(v, "-")
	if ok {
		i := strings.LastIndex(v, "-")
		upstream, revision = v[:i], v[i+1:]
	} else {
		upstream = v
		revision = "0"
	}
	return epoch, upstream, revision
}
func order(c byte) int {
	if c == '~' {
		return -1
	}
	if c == 0 {
		return 0
	}
	if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
		return int(c)
	}
	return int(c) + 256
}
func comparePart(a, b string) int {
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		for (i < len(a) && !(a[i] >= '0' && a[i] <= '9')) || (j < len(b) && !(b[j] >= '0' && b[j] <= '9')) {
			var x, y byte
			if i < len(a) && !(a[i] >= '0' && a[i] <= '9') {
				x = a[i]
				i++
			}
			if j < len(b) && !(b[j] >= '0' && b[j] <= '9') {
				y = b[j]
				j++
			}
			if order(x) != order(y) {
				if order(x) > order(y) {
					return 1
				}
				return -1
			}
		}
		for i < len(a) && a[i] == '0' {
			i++
		}
		for j < len(b) && b[j] == '0' {
			j++
		}
		ai, bj := i, j
		for i < len(a) && a[i] >= '0' && a[i] <= '9' {
			i++
		}
		for j < len(b) && b[j] >= '0' && b[j] <= '9' {
			j++
		}
		if i-ai != j-bj {
			if i-ai > j-bj {
				return 1
			}
			return -1
		}
		if c := strings.Compare(a[ai:i], b[bj:j]); c != 0 {
			return c
		}
	}
	return 0
}
func newer(candidate, installed string) bool {
	ae, au, ar := versionPart(candidate)
	be, bu, br := versionPart(installed)
	if ae != be {
		return ae > be
	}
	if c := comparePart(au, bu); c != 0 {
		return c > 0
	}
	return comparePart(ar, br) > 0
}

// Updates reads the local APT package indexes and dpkg status, without refreshing or changing them.
func Updates(ctx context.Context, listsRoot, statusPath, rebootPath string) model.Result {
	unknown := model.Result{Name: "security:updates", State: model.Unknown, Detail: "APT state unreadable"}
	installed := map[string]string{}
	if err := paragraphs(statusPath, []string{"Package", "Version", "Status"}, func(row map[string]string) {
		if row["Status"] == "install ok installed" && row["Package"] != "" {
			installed[row["Package"]] = row["Version"]
		}
	}); err != nil {
		return unknown
	}
	paths, err := filepath.Glob(filepath.Join(listsRoot, "*_Packages"))
	if err != nil || len(paths) == 0 {
		return unknown
	}
	type candidate struct {
		version  string
		security bool
	}
	candidates := map[string]candidate{}
	for _, path := range paths {
		security := strings.Contains(filepath.Base(path), "-security_")
		err = paragraphs(path, []string{"Package", "Version"}, func(row map[string]string) {
			pkg := row["Package"]
			old, ok := installed[pkg]
			if !ok || row["Version"] == "" || ctx.Err() != nil {
				return
			}
			v := row["Version"]
			if !newer(v, old) {
				return
			}
			prev, exists := candidates[pkg]
			if !exists || newer(v, prev.version) {
				candidates[pkg] = candidate{v, security || prev.security}
			} else if v == prev.version && security {
				prev.security = true
				candidates[pkg] = prev
			}
		})
		if err != nil || ctx.Err() != nil {
			return unknown
		}
	}
	security := 0
	for _, c := range candidates {
		if c.security {
			security++
		}
	}
	state := model.OK
	if security > 0 {
		state = model.Warn
	}
	reboot := 0.0
	if _, err := os.Stat(rebootPath); err == nil {
		reboot = 1
		state = model.Warn
	} else if !os.IsNotExist(err) {
		return unknown
	}
	return model.Result{Name: "security:updates", State: state, Detail: fmt.Sprintf("%d pending, %d security; local APT cache", len(candidates), security), Values: map[string]float64{"total": float64(len(candidates)), "security": float64(security), "reboot_required": reboot}}
}
