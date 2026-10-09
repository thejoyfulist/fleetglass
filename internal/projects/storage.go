package projects

import (
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

// ScanIONice runs the bounded scan in a disposable idle-I/O-priority process.
// du emits directories only at the configured depth, never child file names.
func ScanIONice(ctx context.Context, root string) []model.StorageEntry {
	root = filepath.Clean(root)
	cmd := exec.CommandContext(ctx, "ionice", "-c3", "nice", "-n", "19", "du", "-x", "-B1", "--max-depth=2", "--null", "--", root)
	raw, err := cmd.Output()
	complete := err == nil && ctx.Err() == nil
	out := []model.StorageEntry{}
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		pair := strings.SplitN(record, "\t", 2)
		if len(pair) != 2 {
			continue
		}
		size, e := strconv.ParseInt(pair[0], 10, 64)
		if e != nil || size < 0 {
			continue
		}
		path := filepath.Clean(pair[1])
		if !within(path, root) {
			continue
		}
		depth := 0
		if path != root {
			rel, e := filepath.Rel(root, path)
			if e != nil {
				continue
			}
			depth = len(strings.Split(rel, string(filepath.Separator)))
		}
		if depth > 2 {
			continue
		}
		out = append(out, model.StorageEntry{Root: root, Path: path, Depth: depth, SizeBytes: size, Complete: complete})
	}
	if len(out) == 0 {
		out = append(out, model.StorageEntry{Root: root, Path: root, Complete: false})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SizeBytes == out[j].SizeBytes {
			return out[i].Path < out[j].Path
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})
	return out
}
