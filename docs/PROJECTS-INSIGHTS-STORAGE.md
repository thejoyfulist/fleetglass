# Projects, fixed insights, heat, and storage (M9b)

## Configuration and access

Set `project_roots` and `worktree_roots` to absolute, clean directories. Processes are attributed by `/proc/<pid>/cwd`: the first directory below a project root becomes its project; the first two below a worktree root become project and worktree. The longest matching configured root wins. Agents and processes with a listening TCP socket appear immediately; other processes appear after five minutes of observation. Paths outside these roots are not collected. Process names come from `/proc/<pid>/stat`; command lines and environment are never read. Memory is process RSS and the freeable sum is an estimate, since shared pages can be counted more than once.

On a multi-user server, the agent asks the existing privileged helper for the project process inventory over its Unix socket. The helper accepts only `GET /projects`, uses the roots in its own configuration, and returns bounded process metadata. If the helper is unavailable, the agent uses only processes it can read. Install the updated helper binary and config together with the updated agent; no new listener or permission is needed. Fleetglass never executes the displayed `kill -TERM -- PID` command. A copied PID can be reused, so verify the process before using the command.

Set `storage_roots` to absolute, clean directories; the default is `/`, `/home`, and `/var`. The running agent starts a directory-only `du` scan once an hour in a disposable `ionice -c3 nice -n 19` process, limited to 20 seconds per root; the scan runs separately so live samples continue. One-shot agent mode skips this slow scan. The scan stays on one filesystem, never follows symlinks, and reports directories at depth zero through two. If permissions make the result partial, the agent asks the helper's fixed `GET /storage/{configured-index}` endpoint to scan that root. Each root is fixed by config; no URL supplies a path. Install `ionice`, `nice`, and GNU `du` for scans. Partial or timed-out totals are marked as such. The server keeps hourly storage observations for 31 days; 7-day growth is unknown until a complete observation at least seven days old exists.

For a host with project and worktree directories, example roots are:

```json
{
  "project_roots": ["/srv/projects"],
  "worktree_roots": ["/srv/workspaces"],
  "storage_roots": ["/", "/home", "/var"]
}
```

## Fixed rules and coverage

Project idle time starts when Fleetglass first sees the process and resets on CPU above 1% of one core or a new established connection. The UI calls a dev server idle after 15 minutes. The current process RSS and listening ports determine the displayed freeable estimate. Connection detection uses process-owned socket inodes; inaccessible descriptors show ports as unmeasured and prevent idle or runaway claims.

The read API `GET /api/v1/hosts/{id}/observations` returns current project processes, the latest storage scan, insights and recent heat markers. It follows the normal read-token access rules. No LLM is involved. The insight rules are:

| Rule | Threshold and duration |
| --- | --- |
| Sustained CPU | App CPU at least 80% of one core for 10 minutes |
| Memory growth | At least 100 MiB rise over an hour, without a drop of more than 5% between observations |
| Heavy writes | App writes at least 10 MiB/s for 15 minutes |
| Heavy network | Measured receive plus transmit at least 10 MiB/s for 15 minutes; unavailable network does not pass |
| Idle dev server | No qualifying CPU or new connection for 3 hours |
| Runaway dev server | At least 80% CPU and no new connection for 30 minutes |

Insights are evaluated from observed app samples, so gaps do not imply normal operation. They appear in the Overview and on the matching app or project; they do not send alerts. Heat markers are recorded when a sensor reports at least 80 °C or CPU package power reaches 45 W. Each 15-minute marker names up to three current CPU-leading apps. Temperature and package power are separate triggers; neither measures wall power.
