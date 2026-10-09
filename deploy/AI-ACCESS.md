# AI agent access

The Fleetglass agent runs as the unprivileged `fleetglass` user. The optional log fallback needs read-only access to the configured Claude Code and Codex JSONL roots and execute (traversal) access to their parent directories. Apply ACLs only to roots explicitly listed in `ai_log_roots`. For example, after creating the `fleetglass` user and confirming the selected paths, an administrator can run:

```sh
setfacl -m u:fleetglass:x /home/example
setfacl -m u:fleetglass:x /home/example/.claude /home/example/.codex
setfacl -R -m u:fleetglass:rX /home/example/.claude/projects /home/example/.codex/sessions
find /home/example/.claude/projects /home/example/.codex/sessions -type d -exec setfacl -m d:u:fleetglass:rX {} +
```

Review existing ACLs before applying this example on a multi-user host. No writable ACL is needed. The helper's protected Unix socket uses the `fleetglass` group and exposes a fixed `/agents` read endpoint; only processes with allowlisted `claude` and `codex` names are returned. The agent does not need root privileges or access to CLI configuration files. Do not add account credentials, environment files, or command-line access to these grants.
