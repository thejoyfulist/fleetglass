"""Capture README screenshots from synthetic, healthy local fixtures only."""
import os
import sys
if __name__ == "__main__" and os.environ.get("FLEETGLASS_HEAVY_LOCK") and not os.environ.get("FLEETGLASS_LOCK_HELD"):
    os.environ["FLEETGLASS_LOCK_HELD"] = "1"
    os.execvp("flock", ["flock", os.environ["FLEETGLASS_HEAVY_LOCK"], sys.executable, *sys.argv])

import re
import json
import threading
from pathlib import Path
from http.server import ThreadingHTTPServer
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright
import visual_check as fixture

OUT = Path(__file__).resolve().parents[2] / "assets" / "screenshots"
fixture.HOST_ID = "demo-01"
fixture.RESULTS = [
    fixture.result("host", {"uptime_seconds": 1043400}) | {"fields": {"kernel_release":"6.12.1-demo", "arch":"amd64"}},
    fixture.result("cpu", {"frequency_current_mhz": 2200, "frequency_max_mhz": 3600, "busy_percent": 18, "core_count": 8, "user_percent": 11, "system_percent": 4, "iowait_percent": 2, "steal_percent": 1, "core_0_percent": 12, "core_1_percent": 24}),
    fixture.result("load", {"1m": 0.82, "5m": 0.76, "15m": 0.69}),
    fixture.result("memory", {"used_bytes": 11 * 1024**3, "total_bytes": 32 * 1024**3, "swap_used_bytes": 0, "swap_total_bytes": 4 * 1024**3, "page_cache_bytes": 4 * 1024**3, "buffers_bytes": 512 * 1024**2, "pressure_some_avg10": 0.4}),
    fixture.result("disk:/", {"used_bytes": 236 * 1024**3, "total_bytes": 1000 * 1024**3, "inodes_used": 82000, "inodes_total": 1000000}),
    fixture.result("cpu_package_power", {"watts": 22}),
    fixture.result("sensor:package", {"celsius": 48}),
    fixture.result("systemd:failed", {"count": 0}),
    fixture.result("systemd:api", detail="active") | {"detail_code": "active"},
    fixture.result("systemd:worker", detail="active") | {"detail_code": "active"},
    fixture.result("dns:demo.example", {"latency_ms": 12}, detail="resolved"),
    fixture.result("network:eth0", {"rx_bytes_per_second": 28672, "tx_bytes_per_second": 12288, "rx_bytes": 8 * 1024**3, "tx_bytes": 4 * 1024**3}),
    fixture.result("port:tcp:ipv4:3100:loopback", {"port": 3100}),
    fixture.result("smart:0", {"wear_percent": 5, "media_errors": 0}, detail="disk health"),
    fixture.result("container:local:api", detail="healthy") | {"detail_code": "healthy"},
    fixture.result("container:local:worker", detail="healthy") | {"detail_code": "healthy"},
    fixture.result("container:local:cache", detail="healthy") | {"detail_code": "healthy"},
    fixture.result("tls:demo.example", {"days_remaining": 21}, state="warn", detail="Expires in 21 days") | {"detail_code": "certificate_expires"},
    fixture.result("recent_errors", {"count": 0, "oom_count": 0}),
    fixture.result("recent_errors:container_restarts", {"count": 0}),
    fixture.result("security:updates", {"total": 0, "security": 0, "reboot_required": 0}),
    fixture.result("security:failed_logins", {"count": 0}),
    fixture.result("security:sensitive_files", {"monitored": 4, "changes": 0}),
    fixture.result("backup:daily", {"count": 30, "age_seconds": 7200, "size_bytes": 840 * 1024**2}),
    fixture.result("status:restore_test", {"age_seconds": 86400}, detail="recent timestamp"),
    fixture.result("status:offsite_copy", {"age_seconds": 3600}, detail="recent timestamp"),
    fixture.result("internal:api", {"latency_ms": 24}, detail="TCP connect succeeded") | {"detail_code": "tcp_connect_succeeded", "fields": {"display_name": "API health"}},
]

def session(number, kind, project, model, cpu, memory):
    return {"id": f"demo-session-{number}", "label": f"{kind} · {project} · demo-{number}", "api_equivalent_usd": (7.92,5.84,3.67)[number-1], "usage_measured": True, "speed_tokens_per_second": (15.2,42.5,13.8)[number-1], "speed_method":"estimated", "kind": kind, "project": project,
            "worktree": project, "user": "developer", "pid": 2400 + number,
            "cpu_percent": cpu, "rss_bytes": memory * 1024**2, "at": fixture.iso(), "started_at": fixture.iso(-5400),
            "state": "working", "reason": "", "source": "otel", "model": model}

fixture.AGENTS = [session(1, "claude", "atlas", "claude-sonnet-4-5", 7.2, 480),
                  session(2, "codex", "orbit", "gpt-6-sol", 5.1, 360),
                  session(3, "claude", "ledger", "claude-sonnet-4-5", 3.4, 410)]
fixture.AGENTS[0].update({"source":"otlp", "sources":["otlp","process"], "provenance":{"state":"otlp","tokens":"otlp","speed":"estimated","cost":"estimated","resources":"process","model":"otlp","project":"process"}})
fixture.AGENTS[1].update({"source":"log", "sources":["log","process"], "provenance":{"state":"log","tokens":"log","speed":"estimated","cost":"estimated","resources":"process","model":"log","project":"process"}})
fixture.AGENTS[2].update({"source":"otlp", "sources":["otlp","log","process"], "provenance":{"state":"log","tokens":"otlp","speed":"otlp","cost":"estimated","resources":"process","model":"log","project":"log"}})
fixture.AGENTS.append({"id":"demo-process-only", "label":"codex · demo · process", "kind":"codex", "project":"demo", "worktree":"", "pid":2499, "at":fixture.iso(), "state":"running", "reason":"process alive", "source":"process", "sources":["process"], "provenance":{"state":"process","resources":"process","project":"process"}})
fixture.USAGE = [
    {"name": name, "tokens": fixture.tokens(count), "hourly": [],
     "daily": [{"at": fixture.iso(-86400 * i), "tokens": fixture.tokens(round(count / 7))} for i in range(7)]}
    for name, count in (("atlas", 920000), ("orbit", 680000), ("ledger", 430000))
]
fixture.QUOTAS = [
    {"kind": "codex", "window": "secondary", "used_percent": 38,
     "resets_at": fixture.iso(4 * 86400), "observed_at": fixture.iso(), "method": "cli_reported"},
    {"kind": "codex", "window": "primary", "used_percent": 26,
     "resets_at": fixture.iso(3 * 3600), "observed_at": fixture.iso(), "method": "cli_reported"},
    {"kind": "claude", "window": "primary", "used_percent": 31,
     "resets_at": fixture.iso(2 * 3600), "observed_at": fixture.iso(), "method": "estimated"},
]
fixture.MODEL_COSTS = [{"project":name,"model":name,"method":"api_equivalent","usd":amount,"partial":False,"unpriced_tokens":0} for name, amount in (("claude-sonnet-4-5",11.59),("gpt-6-sol",5.84))]
fixture.COSTS = [{"project": name, "method": "api_equivalent", "usd": amount,
                  "partial": False, "unpriced_tokens": 0}
                 for name, amount in (("atlas", 7.92), ("orbit", 5.84), ("ledger", 3.67))]
fixture.SPEED[0]["api_errors"] = 0
fixture.APPS = [
    {"name": name, "kind": kind, "rates_ready": True, "cpu_percent": cpu,
     "memory_bytes": memory * 1024**2, "page_cache_bytes": 0,
     "disk_read_bytes_per_second": 2048, "disk_write_bytes_per_second": 1024,
     "disk_read_bytes": 6 * 1024**3, "disk_write_bytes": 2 * 1024**3}
    for name, kind, cpu, memory in (("api", "container", 12, 540),
                                    ("worker", "container", 9, 380),
                                    ("cache", "container", 3, 290),
                                    ("atlas", "agent", 7, 480),
                                    ("orbit", "agent", 5, 360),
                                    ("ledger", "agent", 3, 410))
]
fixture.APPS.append({"name":"other", "kind":"other", "rates_ready":True, "cpu_percent":2,
    "memory_bytes":128*1024**2, "page_cache_bytes":0, "disk_read_bytes_per_second":0,
    "disk_write_bytes_per_second":0, "disk_read_bytes":0, "disk_write_bytes":0})
fixture.WORKTREES = {
    "worktrees": [
        {"root": "/tmp/demo-workspaces", "project": name, "name": name,
         "path": f"/tmp/demo-workspaces/{name}", "size_bytes": (120 + i * 30) * 1024**2,
         "size_complete": True, "created_at": fixture.iso(-172800),
         "last_commit_at": fixture.iso(-3600 * (i + 1)), "merged": False,
         "agent_processes": 1, "processes": 2, "stale": False, "reclaimable": False}
        for i, name in enumerate(("atlas", "orbit", "ledger"))
    ],
    "reclaimable_bytes": 0, "at": fixture.iso(),
}
fixture.CHANGES = [
    {"id": "a" * 64, "at": fixture.iso(-3600), "type": "Service",
     "text": "api.service restarted", "change_code": "service_restarted",
     "change_params": {"name": "api.service"}},
]
fixture.OBSERVATIONS = {
    "projects": [
        {"pid": 3200 + i, "name": "node", "project": name, "worktree": name,
         "kind": "dev_server", "cpu_percent": 1.1 + i, "memory_bytes": (180 + i * 40) * 1024**2,
         "ports": [3100 + i], "ports_measured": True, "connections_measured": True,
         "idle_minutes": 0, "connection_idle_minutes": 0, "idle": False,
         "stop_command": f"kill -TERM -- {3200 + i}", "started_at": fixture.iso(-7200)}
        for i, name in enumerate(("atlas", "orbit", "ledger"))
    ],
    "insights": [], "heat": [], "storage": [
        {"root":"/tmp/demo-workspaces", "path":"atlas", "depth":0, "size_bytes":420*1024**2,
         "complete":True, "growth_7d_bytes":12*1024**2, "at":fixture.iso()},
        {"root":"/tmp/demo-workspaces", "path":"orbit", "depth":0, "size_bytes":310*1024**2,
         "complete":True, "growth_7d_bytes":-8*1024**2, "at":fixture.iso()},
    ],
}
fixture.PROBES = [{**fixture.PROBES[0], "name": "demo_site", "display_name": "Demo site",
                   "coverage_24h": 1, "coverage_7d": 1, "coverage_30d": 1,
                   "days": [{"date": (fixture.NOW - fixture.timedelta(days=29-i)).date().isoformat(),
                             "state": "ok", "uptime": 100, "coverage": 1} for i in range(30)],
                   "points": [{"at": fixture.iso(-3600 * (23-i)), "latency_ms": 105 + i, "state": "ok"} for i in range(24)]}]
fixture.SERIES_BASES.update({"busy_percent": 16, "watts": 20, "used_bytes": 236 * 1024**3,
                             "celsius": 45, "cost": 2.3})

class ShowcaseHandler(fixture.Handler):
    def do_GET(self):
        path = urlparse(self.path).path
        if "/agents/demo-session-" in path:
            index = int(path.rsplit("-",1)[-1])-1
            session = fixture.AGENTS[index]
            cost = (7.92,5.84,3.67)[index]
            data = {"session":session,"tokens":fixture.USAGE[index]["tokens"],
                    "usage_measured":True,"tokens_per_minute":720,
                    "api_equivalent_usd":cost,"cli_reported_usd":0,
                    "unpriced_tokens":0,"processes":[],"speed":[{
                        **fixture.SPEED[0], "kind":session["kind"], "model":session["model"],
                        "live_tokens_per_second":(15.2,42.5,13.8)[index],
                    }]}
            body = json.dumps(data).encode()
            self.send_response(200); self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
            return
        if path == "/api/v1/fleet":
            data = [{"host_id":"demo-01", "at":fixture.iso(), "liveness":"live",
                     "normal":24, "checks":25, "cpu":18, "memory":34, "disk":24,
                     "agents":3, "usage":fixture.USAGE, "costs":fixture.COSTS}]
            body = json.dumps(data).encode()
            self.send_response(200); self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
            return
        super().do_GET()

def main():
    OUT.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", 0), ShowcaseHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base = f"http://127.0.0.1:{server.server_port}"
    shots = tuple((view, theme, 1440) for view in ("overview", "agents", "apps", "projects") for theme in ("light", "dark")) + (("overview", "light", 390),)
    try:
        with sync_playwright() as playwright:
            browser = playwright.chromium.launch(headless=True)
            for view, theme, width in shots:
                page = browser.new_page(viewport={"width": width, "height": 900 if width == 1440 else 844}, device_scale_factor=1)
                page.add_init_script("window.localStorage.setItem('fg-theme', '%s'); window.EventSource=class {constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){}};" % theme)
                page.goto(base + ("/" if view == "overview" else f"/{view}"), wait_until="networkidle")
                page.locator("main").wait_for()
                page.wait_for_timeout(250)
                # Full-page captures stitch the viewport, so pinned bars would land mid-image.
                page.add_style_tag(content=".terminal-footer,.terminal-header,.terminal-tabs{position:static!important}")
                body = page.locator("body").inner_text()
                if re.search(r"\b(?:internal|oom):[a-z0-9_]+\b", body):
                    raise AssertionError(f"Technical identifier visible in {view}-{theme}-{width}")
                if page.evaluate("document.documentElement.scrollWidth > innerWidth + 1"):
                    raise AssertionError(f"Page horizontal scroll in {view}-{theme}-{width}")
                if width == 1440:
                    overflowing = page.locator(".terminal-box-body").evaluate_all("nodes => nodes.filter(n => n.scrollWidth > n.clientWidth + 1).map(n => n.parentElement.getAttribute('aria-label'))")
                    if overflowing:
                        raise AssertionError(f"Panel text clipped in {view}-{theme}-{width}: {overflowing}")
                suffix = "mobile" if width == 390 else theme
                page.screenshot(path=str(OUT / f"{view}-{suffix}.png"), full_page=True)
                if view == "agents" and theme == "light":
                    page.locator(".agent-table .terminal-link").first.click()
                    page.locator("#session-detail").wait_for()
                    page.screenshot(path=str(OUT / "agents-detail-light.png"), full_page=True)
                page.close()
            browser.close()
    finally:
        server.shutdown()
        server.server_close()
    print(f"Wrote {len(shots)} synthetic showcase screenshots to {OUT}")

if __name__ == "__main__":
    main()
