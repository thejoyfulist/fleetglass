package ai

import "strings"

// File-changing tool calls reported by Claude Code and Codex. Shell commands
// are deliberately excluded: their arguments are never inspected.
func IsFileTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "edit", "multiedit", "write", "notebookedit", "apply_patch", "functions.apply_patch", "mcp__codex__apply_patch", "edit_file", "write_file", "functions.edit_file", "functions.write_file":
		return true
	}
	return false
}
