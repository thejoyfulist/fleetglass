package projects

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func gitRead(ctx context.Context, root string, args ...string) (string, bool) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := command.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func gitAncestor(ctx context.Context, root, branch string) *bool {
	command := exec.CommandContext(ctx, "git", "-C", root, "merge-base", "--is-ancestor", "HEAD", branch)
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	err := command.Run()
	if err == nil {
		value := true
		return &value
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 && ctx.Err() == nil {
		value := false
		return &value
	}
	return nil
}

// ScanWorktrees inspects only configured two-level roots, with bounded size walks.
func ScanWorktrees(ctx context.Context, roots []string, processes []model.ProjectProcess, staleSessions []model.AgentSession) []model.WorktreeEntry {
	out := []model.WorktreeEntry{}
	stale := map[string]bool{}
	for _, session := range staleSessions {
		if session.State == "stale-worktree" {
			stale[session.Project+"/"+session.Worktree] = true
		}
	}
	for _, root := range roots {
		projects, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, project := range projects {
			if ctx.Err() != nil || len(out) >= 100 {
				return out
			}
			if !project.IsDir() || project.Type()&os.ModeSymlink != 0 {
				continue
			}
			trees, err := os.ReadDir(filepath.Join(root, project.Name()))
			if err != nil {
				continue
			}
			for _, tree := range trees {
				if ctx.Err() != nil || len(out) >= 100 {
					return out
				}
				if !tree.IsDir() || tree.Type()&os.ModeSymlink != 0 {
					continue
				}
				path := filepath.Join(root, project.Name(), tree.Name())
				gitInfo, err := os.Lstat(filepath.Join(path, ".git"))
				if err != nil {
					continue
				}
				entry := model.WorktreeEntry{Root: root, Project: project.Name(), Name: tree.Name(), Path: path, Stale: stale[project.Name()+"/"+tree.Name()]}
				if gitInfo.Mode().IsRegular() {
					entry.CreatedAt = gitInfo.ModTime().UTC()
				}
				for _, process := range processes {
					if process.Project == entry.Project && process.Worktree == entry.Name {
						entry.Processes++
						if process.Kind == "agent" || process.AgentSessionID != "" {
							entry.AgentProcesses++
						}
					}
				}
				treeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				if raw, ok := gitRead(treeCtx, path, "log", "-1", "--format=%cI"); ok {
					entry.LastCommitAt, _ = time.Parse(time.RFC3339, raw)
				}
				branch, ok := gitRead(treeCtx, path, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
				if !ok {
					branch = "refs/remotes/origin/main"
				}
				if _, ok := gitRead(treeCtx, path, "rev-parse", "--verify", branch); ok {
					entry.Merged = gitAncestor(treeCtx, path, branch)
				}
				cancel()
				sizeCtx, sizeCancel := context.WithTimeout(ctx, 2*time.Second)
				for _, size := range ScanIONice(sizeCtx, path) {
					if size.Path == path {
						entry.SizeBytes = size.SizeBytes
						entry.SizeComplete = size.Complete
						break
					}
				}
				sizeCancel()
				entry.Reclaimable = entry.Merged != nil && *entry.Merged && entry.Processes == 0 && entry.SizeComplete
				out = append(out, entry)
			}
		}
	}
	return out
}
