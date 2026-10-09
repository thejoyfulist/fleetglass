# Database backup and restore

The server uses SQLite WAL mode. `fleetglass-server -config /etc/fleetglass/config.json backup /safe/place/fleetglass.db` creates a consistent online snapshot with SQLite `VACUUM INTO`; the destination must not exist. The command prints only a success message. Store snapshots away from the monitored disk and protect them like the database, since host, project and session metadata are present.

For daily local snapshots, set `backup_dir` to an absolute directory writable by the server account and `backup_retention_days` to a positive integer. The server takes at most one snapshot per UTC day at startup or hourly maintenance, and removes only old files named `fleetglass-YYYY-MM-DD.db` in that directory. This is a local copy, not an off-site backup. Use `backup_sets` and `status_files` to observe independent off-site copies and restore tests.

To restore, stop Fleetglass services under administrator control, keep the existing database and its `-wal`/`-shm` siblings together as a rollback copy, copy the validated backup to the configured `db_path` with owner `fleetglass` and mode `0600`, then start the services. Verify `PRAGMA integrity_check` with `sqlite3` or a local SQLite tool first. Do not copy over a live WAL database. A restore rolls history back to the snapshot time; remote agents buffer only about ten minutes of batches.
