"""Exercise the committed cockpit build with deterministic local API fixtures."""
import os
import sys
import tempfile
if __name__ == "__main__" and os.environ.get("FLEETGLASS_HEAVY_LOCK") and not os.environ.get("FLEETGLASS_LOCK_HELD"):
    os.environ["FLEETGLASS_LOCK_HELD"] = "1"
    os.execvp("flock", ["flock", os.environ["FLEETGLASS_HEAVY_LOCK"], sys.executable, *sys.argv])
import json
import math
import re
import threading
from datetime import datetime, timedelta, timezone
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse, parse_qs
from playwright.sync_api import sync_playwright

DIST = Path(__file__).resolve().parents[1] / "dist"
SHOTS = Path(os.environ.get('FLEETGLASS_SCREENSHOTS_DIR', tempfile.mkdtemp(prefix='fleetglass-screenshots-')))
HOST_ID = "demo-host"
NOW = datetime.now(timezone.utc)
def iso(delta=0):
    return (NOW + timedelta(seconds=delta)).isoformat().replace("+00:00", "Z")
def result(name, values=None, state="ok", detail="Observed"):
    row = {"name": name, "state": state, "detail": detail, "values": values or {}}
    if detail == "Observed": row["detail_code"] = "observed"
    return row
RESULTS = [
    result("cpu", {"busy_percent": 5.0, "core_count": 4}),
    result("memory", {"used_bytes": 47 * 1024**3, "total_bytes": 64 * 1024**3, "swap_used_bytes": 2 * 1024**3, "swap_total_bytes": 8 * 1024**3}),
    result("disk:/", {"used_bytes": 470 * 1024**3, "total_bytes": 1000 * 1024**3, "inodes_used": 120000, "inodes_total": 1000000}),
    result("cpu_package_power", {"watts": 6.0}),
    result("sensor:package", {"celsius": 42.0}),
    result("systemd:failed", {"count": 0}),
    result("systemd:caddy", detail="active") | {"detail_code":"active"},
    result("systemd:tailscaled", detail="active") | {"detail_code":"active"},
    result("systemd:sample_service", detail="active") | {"detail_code":"active"},
    result("network:eth0", {"rx_bytes_per_second": 2048, "tx_bytes_per_second": 512, "rx_bytes": 5 * 1024**3, "tx_bytes": 2 * 1024**3}),
    result("container:local:web", detail="healthy") | {"detail_code":"healthy"},
    result("tls:example.test", {"days_remaining":21}, state="warn", detail="Expires in 21 days") | {"detail_code":"certificate_expires"},
    result("port:tcp:ipv4:443:any", detail="HTTPS"),
    result("recent_errors", {"count":14,"oom_count":1}, state="warn") | {"detail_code":"observed"},
    {**result("recent_error:0", {"count":11}, state="warn"), "fields":{"unit":"livepatch.service","message":"Refresh failed while checking metadata"}},
    result("recent_errors:container_restarts", {"count":0}),
    result("security:updates", {"total":8,"security":2,"reboot_required":0}, state="warn"),
    {**result("security:ssh_logins", {"count":4}), "fields":{"last_user":"agent","source_class":"tailnet"}},
    result("security:sudo", {"count":2}),
    result("security:failed_logins", {"count":3}, state="warn"),
    result("security:sensitive_files", {"monitored":7,"changes":0}),
    result("backup:example", {"count":8,"age_seconds":7200,"size_bytes":240*1024**2}),
    result("status:guard", {"age_seconds":360}, detail="configured status value") | {"detail_code":"configured_status_value"},
    result("status:offsite_copy", {"age_seconds":3600}, detail="recent timestamp") | {"detail_code":"recent_timestamp"},
    result("log_marker:backup_verified", {"count":4,"age_seconds":7200}, detail="matching log entry observed") | {"detail_code":"log_entry_observed"},
    result("internal:db_relay", {"latency_ms":21}, detail="TCP connect succeeded") | {"detail_code":"tcp_connect_succeeded"},
    result("internal:closed_target", state="warn", detail="Connection refused"),
    result("oom:0", {"count":1}, state="warn", detail="Kernel out-of-memory kill"),
    {**result("repository:0:fleetglass", {"dirty":1,"ahead":2,"behind":0,"last_commit_age_seconds":3600,"has_upstream":1}, state="warn"), "fields":{"repository":"fleetglass"}},
]
AGENTS = [
    {"id":"session-1", "kind":"claude", "project":"dashboard", "worktree":"dashboard", "user":"agent", "pid":2345, "cpu_percent":5.0, "rss_bytes":512*1024**2, "at":iso(), "state":"working", "reason":"", "source":"otel", "model":"Claude Sonnet"},
    {"id":"session-2", "kind":"codex", "project":"fleetglass", "worktree":"fleetglass-m3", "user":"agent", "pid":2456, "cpu_percent":2.5, "rss_bytes":384*1024**2, "at":iso(), "state":"waiting-for-input", "reason":"Awaiting approval", "source":"local log", "model":"Codex"},
]
def tokens(n):
    return {"input":n, "output":0, "cache_write":0, "cache_read":0}
USAGE = [
    {"name":"dashboard", "tokens":tokens(970000), "hourly":[], "daily":[{"at":iso(-86400*i),"tokens":tokens(100000+i*2000)} for i in range(7)]},
    {"name":"fleetglass", "tokens":tokens(740000), "hourly":[], "daily":[{"at":iso(-86400*i),"tokens":tokens(80000+i*1000)} for i in range(7)]},
]
QUOTAS = [
    {"kind":"codex","window":"secondary","used_percent":41.6,"resets_at":iso(4*86400),"observed_at":iso(),"method":"cli_reported"},
    {"kind":"codex","window":"primary","used_percent":18.1,"resets_at":iso(3*3600),"observed_at":iso(),"method":"cli_reported"},
]
COSTS = [
    {"project":"dashboard", "method":"api_equivalent", "usd":8.47,"partial":True,"unpriced_tokens":970000},
    {"project":"fleetglass", "method":"api_equivalent", "usd":4.21,"partial":False,"unpriced_tokens":0},
]
SPEED = [{"kind":"codex","model":"gpt-6-sol","method":"estimated","window":"1h","live_tokens_per_second":42.5,"median_tokens_per_second":38.2,"p95_tokens_per_second":52.1,"median_duration_ms":None,"median_first_token_ms":None,"tokens_per_minute":2400,"requests_per_minute":1.2,"api_errors":1,"retries":0}]
SESSION_SPEED = [{"kind":"claude","model":"Claude Sonnet","method":"estimated","window":"1h","live_tokens_per_second":12.5,"median_tokens_per_second":11.8,"p95_tokens_per_second":15.1,"median_duration_ms":None,"median_first_token_ms":None,"tokens_per_minute":700,"requests_per_minute":0.6,"api_errors":0,"retries":0}]
SUBSCRIPTIONS = [{"kind":"claude","month":"2026-10","api_equivalent_usd":8.47,"subscription_usd":None,"coverage":"partial","unpriced_tokens":970000},{"kind":"codex","month":"2026-10","api_equivalent_usd":4.21,"subscription_usd":None,"coverage":"complete","unpriced_tokens":0}]
WORKTREES = {"worktrees":[{"root":"/tmp/workspaces","project":"fleetglass","name":"fleetglass-m9d","path":"/tmp/workspaces/fleetglass/fleetglass-m9d","size_bytes":125829120,"size_complete":True,"created_at":iso(-172800),"last_commit_at":iso(-3600),"merged":False,"agent_processes":1,"processes":1,"stale":False,"reclaimable":False}],"reclaimable_bytes":0,"at":iso()}
APPS = [{"name":"demo-worker","kind":"unit","rates_ready":True,"cpu_percent":120,"memory_bytes":268435456,"page_cache_bytes":0,"disk_read_bytes_per_second":0,"disk_write_bytes_per_second":0,"disk_read_bytes":0,"disk_write_bytes":0}]
CHANGES = [
    {"id":"a"*64,"at":iso(-3600),"type":"APT","text":"Upgraded 3 packages: curl, openssl, git","change_code":"apt_upgraded","change_params":{"count":"3","packages":"curl, openssl, git"}},
    {"id":"b"*64,"at":iso(-7200),"type":"Service","text":"caddy.service restarted","change_code":"service_restarted","change_params":{"name":"caddy.service"}},
    {"id":"c"*64,"at":iso(-14400),"type":"Marker","text":"demo_guard_alert","change_code":"marker","change_params":{"name":"Demo guard alert"}},
]
ENERGY = {"today_kwh":0.134,"month_kwh":3.847,"today_cost":0.0337,"month_cost":0.968,"today_seconds":24000,"month_seconds":420000,"price_per_kwh":0.2516,"currency":"EUR","label":"CPU package only"}
OBSERVATIONS = {"projects":[],"insights":[],"heat":[],"storage":[{"root":"/home","path":"/home/projects","depth":1,"size_bytes":1024**3,"complete":True,"growth_7d_bytes":1024**2,"at":iso()}]}
def probe_days(degraded=False):
    return [{"date":(NOW-timedelta(days=29-i)).date().isoformat(),
             "state":"unknown" if i < 28 else "warn" if degraded and i == 28 else "ok",
             "uptime":None if i < 28 else 95 if degraded and i == 28 else 100,
             "coverage":0 if i < 28 else 0.4 if i == 29 else 1} for i in range(30)]
PROBES = [{"name":"site","display_name":"Example site","kind":"http","state":"ok","uptime_24h":100,"uptime_7d":100,"uptime_30d":100,"coverage_24h":0.12,"coverage_7d":0.02,"coverage_30d":0.004,"days":probe_days(),"p50_ms":120,"p95_ms":170,"percentiles":{"24h":{"p50_ms":120,"p95_ms":170},"7d":{"p50_ms":120,"p95_ms":170},"30d":{"p50_ms":120,"p95_ms":170}},"status_code":200,"latency_ms":121,"dns_ms":2,"connect_ms":8,"tls_ms":20,"first_byte_ms":90,"certificate_days":54,"points":[{"at":iso(-3600*(23-i)),"latency_ms":110+i*2,"state":"crit" if i==18 else "ok"} for i in range(24)]}]
SERIES_BASES = {"busy_percent":5,"watts":6,"used_bytes":470*1024**3,"celsius":42,"rx_bytes_per_second":2048,"tx_bytes_per_second":512,"cost":1.5}
class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(DIST), **kwargs)
    def log_message(self, *args):
        pass
    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        q = parse_qs(parsed.query)
        if path.startswith("/api/v1/"):
            if path == "/api/v1/status":
                data = [{"display_name":"Example site","state":"ok","uptime_30d":100,"coverage_30d":0.004,"days":probe_days(),"latency_ms":120},
                        {"display_name":"Example API","state":"warn","uptime_30d":98.4,"coverage_30d":0.3,"days":probe_days(True),"latency_ms":180}]
                body = json.dumps(data).encode()
                self.send_response(200); self.send_header("Content-Type", "application/json"); self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body); return

            if path == "/api/v1/auth": data = {"required":False}
            elif path == "/api/v1/version": data = {"version":"v0.3.0-demo","latest":"","update_check_enabled":False}
            elif path == "/api/v1/hosts": data = [{"host_id":HOST_ID,"at":iso()}]
            elif path.endswith("/overview"): data = {"host_id":HOST_ID,"at":iso(),"results":RESULTS,"agents":AGENTS}
            elif path.endswith("/agents"): data = AGENTS
            elif path.endswith("/apps"): data = {"apps":APPS,"history":[],"top_hours":[],"range":"24h"}
            elif path.endswith("/ai/quotas"): data = QUOTAS
            elif path.endswith("/ai/costs"): data = COSTS
            elif path.endswith("/ai/speed"): data = SPEED
            elif path.endswith("/ai/subscriptions"): data = SUBSCRIPTIONS
            elif path.endswith("/worktrees"): data = WORKTREES
            elif "/agents/" in path: data = {"session":AGENTS[0],"tokens":tokens(970000),"usage_measured":True,"tokens_per_minute":700,"api_equivalent_usd":8.47,"cli_reported_usd":0,"unpriced_tokens":0,"processes":[],"speed":SESSION_SPEED}
            elif path.endswith("/changes"): data = CHANGES
            elif path.endswith("/energy"): data = ENERGY
            elif path.endswith("/observations"): data = OBSERVATIONS
            elif path.endswith("/probes"): data = PROBES
            elif path.endswith("/ai/usage"):
                by = q.get("by", ["project"])[0]
                if by == "session": data = [{**usage,"name":agent["id"]} for usage,agent in zip(USAGE,AGENTS)]
                elif by == "model": data = [{**USAGE[0],"name":"Claude Sonnet"},{**USAGE[1],"name":"Codex"}]
                else: data = USAGE
            elif path.endswith("/series") or path.endswith("/ai/cost-series"):
                metric = q.get("metric", ["cost"])[0]
                base = SERIES_BASES.get(metric, 5)
                data = [{"at":iso(-3600*(23-i)),"value":base*(0.8+0.15*math.sin(i/3)+0.01*i)} for i in range(24)]
            else: data = []
            body = json.dumps(data).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if path in ("/", "/agents", "/apps", "/projects", "/machine", "/storage", "/services", "/system", "/changes", "/costs", "/uptime", "/status", "/settings", "/share"):
            self.path = "/index.html"
        return super().do_GET()

def inspect(page):
    return page.evaluate("""() => {
      const failures=[];
      const visible=e => { const s=getComputedStyle(e), r=e.getBoundingClientRect(); return s.display!=='none' && s.visibility!=='hidden' && r.width>0 && r.height>0; };
      for (const e of document.querySelectorAll('body *')) {
        if (!visible(e) || e.closest('svg')) continue;
        const hasText=[...e.childNodes].some(n=>n.nodeType===3 && n.textContent.trim());
        if (!hasText) continue;
        const r=e.getBoundingClientRect();
        const ownStyle=getComputedStyle(e);
        if (ownStyle.display!=='inline' && e.clientWidth>0 && e.scrollWidth>e.clientWidth+1)
          failures.push(`overflow ${e.tagName}.${e.className}: ${e.textContent.trim().slice(0,65)}`);
        if (['hidden','clip'].includes(ownStyle.overflowY) && e.clientHeight>0 && e.scrollHeight>e.clientHeight+1)
          failures.push(`vertical overflow ${e.tagName}.${e.className}: ${e.textContent.trim().slice(0,65)}`);
        for (let p=e.parentElement; p && p!==document.body; p=p.parentElement) {
          const s=getComputedStyle(p);
          if (!['hidden','clip','scroll','auto'].includes(s.overflowX) && !['hidden','clip','scroll','auto'].includes(s.overflowY)) continue;
          const b=p.getBoundingClientRect();
          if (r.left < b.left-1 || r.right > b.right+1 || r.top < b.top-1 || r.bottom > b.bottom+1)
            failures.push(`clipped ${e.tagName}: ${e.textContent.trim().slice(0,65)}`);
        }
      }
      for (const tile of document.querySelectorAll('.t')) {
        const t=tile.getBoundingClientRect();
        for (const child of tile.querySelectorAll('*')) {
          if (!visible(child) || child.closest('svg')) continue;
          const r=child.getBoundingClientRect();
          if (r.left<t.left-1 || r.right>t.right+1) failures.push(`tile bounds ${child.tagName}.${child.className}`);
        }
      }
      if (document.documentElement.scrollWidth>innerWidth+1) failures.push('page horizontal scroll');
      return [...new Set(failures)];
    }""")

def inspect_header(page):
    return page.evaluate("""() => {
      const header = document.querySelector('.top').getBoundingClientRect();
      const margin = 8;
      return [...document.querySelectorAll('.top *')].filter(e => !e.closest('svg')).flatMap(e => {
        const style = getComputedStyle(e), rect = e.getBoundingClientRect();
        if (style.display === 'none' || style.visibility === 'hidden' || !rect.width || !rect.height) return [];
        return rect.left < header.left + margin - 1 || rect.right > header.right - margin + 1
          ? [`${e.tagName}.${e.className} outside header (${rect.left.toFixed(1)}–${rect.right.toFixed(1)} vs ${header.left.toFixed(1)}–${header.right.toFixed(1)})`]
          : [];
      });
    }""")

def main():
    SHOTS.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    origin = f"http://127.0.0.1:{server.server_port}"
    errors=[]
    try:
        with sync_playwright() as p:
            browser=p.chromium.launch()
            for width in (1440,1280,1024,390):
                for theme in ("light","dark"):
                    views=("overview","agents","machine","storage","services","system","changes","costs","uptime")
                    page=browser.new_page(viewport={"width":width,"height":900 if width!=390 else 844}, device_scale_factor=1)
                    page.add_init_script("""window.localStorage.setItem('fg-theme', '%s'); window.EventSource=class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };""" % theme)
                    external=[]
                    page.on("request", lambda req: external.append(req.url) if not req.url.startswith(origin) else None)
                    page.goto(origin, wait_until="networkidle")
                    page.locator(".hero").wait_for()
                    page.wait_for_timeout(350)
                    if width in (1440,1280,1024):
                        failures=inspect_header(page)
                        if failures: errors.append(f"header-{theme}-{width}: " + "; ".join(failures[:12]))
                    for view in views:
                        if view == "storage":
                            page.get_by_role("group", name="Subsystem").get_by_role("button", name="Storage").click()
                            if not page.url.endswith("/storage"): errors.append(f"storage-{theme}-{width}: URL not preserved")
                            page.goto(origin+"/storage", wait_until="networkidle")
                            page.get_by_role("group", name="Subsystem").get_by_role("button", name="Storage", pressed=True).wait_for()
                        elif view != "overview": page.get_by_role("link", name=view.capitalize()).click()
                        page.wait_for_timeout(100)
                        failures=inspect(page)
                        if failures: errors.append(f"{view}-{theme}-{width}: " + "; ".join(failures[:12]))
                        if re.search(r"\b\d{1,2}/\d{1,2}/\d{4}\b", page.locator("body").inner_text()):
                            errors.append(f"{view}-{theme}-{width}: numeric date visible")
                        body = page.locator("body").inner_text()
                        observed_names = {"sample_service"}
                        technical = [word for word in re.findall(r"\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b|\b[a-z][a-z0-9_]*:[a-z0-9_./-]+\b", body) if word not in observed_names]
                        if technical: errors.append(f"{view}-{theme}-{width}: technical identifiers visible {sorted(set(technical))}")
                        if view == "storage" and not re.search(r"\b(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun) \d{1,2} [A-Z][a-z]{2}, \d{2}:\d{2}\b", page.locator(".observation-storage .src").inner_text()):
                            errors.append(f"storage-{theme}-{width}: scan date format")
                        if view=="system" and width in (1440,390):
                            for title in ("Backups","Status files and internal probes","Repositories"):
                                if page.get_by_label(title, exact=True).count()!=1: errors.append(f"{view}-{theme}-{width}: missing {title}")
                        if view=="overview" and width in (1440,390) and page.get_by_label("Availability", exact=True).count()!=1:
                            errors.append(f"{view}-{theme}-{width}: missing Availability tile")
                        if view=="overview" and width in (1440,390) and page.get_by_label("Backups", exact=True).count()!=1:
                            errors.append(f"{view}-{theme}-{width}: missing Backups tile")
                        if view=="overview" and width in (1440,390) and page.get_by_label("What changed", exact=True).count()!=1:
                            errors.append(f"{view}-{theme}-{width}: missing What changed tile")
                        if view=="changes" and width in (1440,390) and page.get_by_label("Change journal", exact=True).count()!=1:
                            errors.append(f"{view}-{theme}-{width}: missing Change journal")
                        if view=="costs" and width in (1440,390) and page.get_by_label("Energy", exact=True).count()!=1:
                            errors.append(f"{view}-{theme}-{width}: missing Energy tile")
                        if view=="changes":
                            journal=page.get_by_label("Change journal",exact=True)
                            if journal.locator("select").evaluate("e=>getComputedStyle(e).appearance")!="none": errors.append(f"{view}-{theme}-{width}: native filter")
                            if journal.bounding_box()["height"]>500: errors.append(f"{view}-{theme}-{width}: sparse panel too tall")
                            if re.search(r"\d{1,2}/\d{1,2}/\d{2,4}|\b[AP]M\b|\d{2}:\d{2}:\d{2}",journal.inner_text()): errors.append(f"{view}-{theme}-{width}: inconsistent date")
                        if view=="agents" and any(x!="none" for x in page.locator(".filters select").evaluate_all("es=>es.map(e=>getComputedStyle(e).appearance)")): errors.append(f"{view}-{theme}-{width}: native filter")
                        if view=="costs" and page.get_by_label("Tokens by model",exact=True).bounding_box()["height"]>250: errors.append(f"{view}-{theme}-{width}: sparse model panel too tall")
                        if view=="uptime" and page.locator(".uptime-day").count()!=30: errors.append(f"{view}-{theme}-{width}: missing daily bars")
                        if view=="uptime" and "Measured" not in page.locator(".uptime-stats").inner_text(): errors.append(f"{view}-{theme}-{width}: missing coverage")
                        if view=="uptime" and page.locator(".uptime-day.unknown").count()!=28: errors.append(f"{view}-{theme}-{width}: unmeasured days are not unknown")
                        if view=="uptime" and page.locator(".response-chart svg .axis").count()<10: errors.append(f"{view}-{theme}-{width}: missing response chart axes")
                        if width in (1440,390) or view == "overview":
                            page.screenshot(path=str(SHOTS/f"{view}-{theme}-{width}.png"), full_page=True)
                    if external: errors.append(f"external requests: {external}")
                    if width in (1440,390):
                        status=browser.new_page(viewport={"width":width,"height":900 if width!=390 else 844},device_scale_factor=1)
                        status.add_init_script("window.localStorage.setItem('fg-theme', '%s')" % theme)
                        status_external=[]
                        status.on("request",lambda req: status_external.append(req.url) if not req.url.startswith(origin) else None)
                        status.goto(origin+"/status",wait_until="networkidle")
                        status.get_by_role("heading",name="Service status").wait_for()
                        status.get_by_text("1 service degraded").wait_for()
                        if "Measured" not in status.locator(".uptime-stats").first.inner_text(): errors.append(f"status-{theme}-{width}: missing coverage")
                        if status.get_by_text("ok",exact=True).count(): errors.append(f"status-{theme}-{width}: raw state label")
                        failures=inspect(status)
                        if failures: errors.append(f"status-{theme}-{width}: " + "; ".join(failures[:12]))
                        if status_external: errors.append(f"status-{theme}-{width}: external requests {status_external}")
                        status.screenshot(path=str(SHOTS/f"status-{theme}-{width}.png"),full_page=True)
                        status.close()
                    page.close()
            browser.close()
    finally:
        server.shutdown()
        server.server_close()
    if errors:
        raise AssertionError("\n".join(errors))
    print("Visual check passed at 1440, 1280, 1024 and 390 px in both themes; header height, response chart, status states, overflow and external requests checked")
if __name__ == "__main__":
    main()
