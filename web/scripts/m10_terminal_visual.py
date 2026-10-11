"""Verify every terminal view with synthetic API data and local Chromium."""
import json
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright

sys.path.insert(0, str(Path(__file__).parent))
import showcase  # Synthetic fixtures only.
import visual_check as fixture

RATE = {"original": 24.05, "currency": "USD", "converted": 20.83,
        "display_currency": "EUR", "rate_source": "ecb", "rate_date": "2026-10-09"}
fixture.AGENTS[:] = fixture.AGENTS[:3]
fixture.AGENTS[0]["api_money"] = RATE
fixture.AGENTS[0]["project"] = "unmatched"
fixture.AGENTS[0]["worktree"] = "coordination"
fixture.AGENTS[0]["label"] = "claude · unmatched · demo-1"
fixture.AGENTS[2].update({"id": "0b82eee0abcd", "kind": "claude", "project": "unmatched",
                          "worktree": "", "label": "claude · session · 0b82eee0", "model": "claude-opus-4-1"})
fixture.AGENTS[0].update({"source":"otlp", "sources":["otlp","process"], "provenance":{"state":"otlp","tokens":"otlp","speed":"estimated","cost":"estimated","resources":"process","model":"otlp","project":"process"}})
fixture.AGENTS[1].update({"source":"log", "sources":["log","process"], "provenance":{"state":"log","tokens":"log","speed":"estimated","cost":"estimated","resources":"process","model":"log","project":"process"}})
fixture.AGENTS[2].update({"source":"otlp", "sources":["otlp","log","process"], "provenance":{"state":"log","tokens":"otlp","speed":"otlp","cost":"estimated","resources":"process","model":"log","project":"log"}})
fixture.AGENTS.append({"id":"process-only-demo", "kind":"codex", "project":"demo", "worktree":"", "at":fixture.iso(), "state":"running", "reason":"process alive", "source":"process", "sources":["process"], "provenance":{"state":"process","resources":"process","project":"process"}, "pid":2999, "cpu_percent":1.0, "rss_bytes":128*1024**2, "model":""})
fixture.QUOTAS[:] = [{"kind": "codex", "window": "primary", "window_minutes": 10080,
                      "used_percent": 42, "resets_at": fixture.iso(4 * 86400),
                      "observed_at": fixture.iso(), "method": "cli_reported"}]
fixture.SUBSCRIPTIONS[0]["subscription_usd"] = 20
fixture.RESULTS.extend(fixture.result(f"network:{name}", {"rx_bytes_per_second": 4096-i*300,
                       "tx_bytes_per_second": 1024+i*100, "rx_bytes": 10**9,
                       "tx_bytes": 10**8}) for i,name in enumerate(
                       ("tailscale0", "docker0", "veth123456789abcdef", "veth987654321fedcba",
                        "br-123456789abcdef", "enp123s456f789verylong", "wg0", "eth1")))
fixture.APPS[0]["name"] = "Claude session · coordination · abcdef12"
fixture.APPS[1]["name"] = "Container synthetic-worker-12345678"
fixture.APPS[-1]["name"] = "Other / not attributed"
fixture.MODEL_COSTS[0]["money"] = RATE
fixture.COSTS[0]["money"] = RATE
fixture.SPEED.extend([{**fixture.SPEED[0], "model": "gpt-6-sol", "live_tokens_per_second": 12.0},
                      {**fixture.SPEED[0], "model": "idle-model", "live_tokens_per_second": 0.0,
                       "median_tokens_per_second": 0.0}])
fixture.RESULTS.extend([
    fixture.result(f"recent_error:{i}", {"count": i + 1}, state="warn", detail="journal error group") | {"fields": {"unit": f"demo-{i}.service"}}
    for i in range(5)
])
fixture.RESULTS.append(fixture.result("repository:0:atlas", {"dirty": 1, "dirty_count": 4}, state="warn", detail="4 uncommitted files") | {"fields": {"repository": "atlas"}})

OUT = Path(tempfile.mkdtemp(prefix="fleetglass-m10-views-"))
VIEWS = ("overview", "agents", "apps", "projects", "machine", "storage", "services", "system", "changes", "costs", "uptime", "settings", "glossary", "fleet", "share")
SECTIONS = ("CPU", "Memory", "Disk", "Network", "Containers")

class Handler(showcase.ShowcaseHandler):
    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/api/v1/version":
            body = json.dumps({"version": "0.0.0-dev", "commit": "93f9731e5c5d", "latest": "", "update_check_enabled": False}).encode()
            self.send_response(200); self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
            return
        if path in ("/api/v1/hosts", "/api/v1/fleet"):
            hosts = [{"host_id": "demo-01", "at": fixture.iso(), "liveness": "live", "normal": 12, "checks": 13, "cpu": 18, "memory": 34, "disk": 24, "agents": 3, "usage": fixture.USAGE, "costs": fixture.COSTS},
                     {"host_id": "demo-02", "at": fixture.iso(), "liveness": "live", "normal": 10, "checks": 10, "cpu": 8, "memory": 20, "disk": 12, "agents": 0, "usage": [], "costs": []}]
            body = json.dumps(hosts).encode()
            self.send_response(200); self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
            return
        if path in ("/glossary", "/fleet"):
            self.path = "/index.html"
        super().do_GET()

def inspect(page, width):
    return page.evaluate(r"""width => {
      const errors=[];
      const visible=e=>{const s=getComputedStyle(e),r=e.getBoundingClientRect();return s.display!=='none'&&s.visibility!=='hidden'&&r.width>0&&r.height>0};
      if(document.documentElement.scrollWidth>innerWidth+1)errors.push('page horizontal scroll');
      if(document.querySelector('select'))errors.push('native select rendered');
      if(document.querySelector('main svg circle'))errors.push('SVG circle gauge in content');
      for(const e of document.querySelectorAll('main *')){
        if(!visible(e))continue;
        const s=getComputedStyle(e);
        if(['borderTopLeftRadius','borderTopRightRadius','borderBottomLeftRadius','borderBottomRightRadius'].some(prop=>parseFloat(s[prop])>0.1))errors.push('rounded content: '+e.tagName+'.'+e.className);
        const forbidden=color=>{const m=color.match(/rgba?\((\d+), (\d+), (\d+)/);if(!m)return false;const c=[+m[1],+m[2],+m[3]],max=Math.max(...c),min=Math.min(...c),delta=max-min;if(delta<35||max<90)return false;let hue=max===c[0]?((c[1]-c[2])/delta+6)%6:max===c[1]?(c[2]-c[0])/delta+2:(c[0]-c[1])/delta+4;hue*=60;return hue<170||(hue>340&&hue<360)};
        for(const prop of ['color','backgroundColor','borderTopColor','fill','stroke'])if(forbidden(s[prop])){errors.push('forbidden colour: '+e.tagName+'.'+e.className);break}
      }
      if(document.querySelector('main table img, main [role="table"] img, main table .av, main [role="table"] .av'))errors.push('image or avatar in table');
      if(width===1440){
        for(const body of document.querySelectorAll('.terminal-box-body, main .g.t'))if(body.scrollWidth>body.clientWidth+1)errors.push('panel overflow: '+(body.parentElement.getAttribute('aria-label')||body.className));
        for(const scroller of document.querySelectorAll('main .terminal-table-scroll'))if(scroller.scrollWidth>scroller.clientWidth+1)errors.push('table overflow: '+scroller.closest('section')?.getAttribute('aria-label'));
        for(const cell of document.querySelectorAll('main .terminal-data-table td.numeric'))if(cell.scrollWidth>cell.clientWidth+1)errors.push('numeric cell clipped: '+cell.innerText);
      }
      if(width===1280 || width===1440){
        const tabs=document.querySelector('.terminal-tabs .tabs');
        const buttons=[...tabs.querySelectorAll('button')];
        if(buttons.length && Math.max(...buttons.map(b=>b.getBoundingClientRect().top))-Math.min(...buttons.map(b=>b.getBoundingClientRect().top))>1)errors.push('tab row wraps');
        if(tabs.scrollWidth>tabs.clientWidth+1)errors.push('desktop tabs clipped');
        const status=[...document.querySelectorAll('.terminal-status > *')].filter(visible);
        if(status.length && Math.max(...status.map(e=>e.getBoundingClientRect().top+e.getBoundingClientRect().height/2))-Math.min(...status.map(e=>e.getBoundingClientRect().top+e.getBoundingClientRect().height/2))>8)errors.push('status line wraps');
        if(document.querySelector('.terminal-status').scrollWidth>document.querySelector('.terminal-status').clientWidth+1)errors.push('desktop status clipped');
      }
      const tag=document.querySelector('.terminal-warn .terminal-tag');
      if(tag&&getComputedStyle(tag).backgroundColor!=='rgb(0, 166, 178)')errors.push('warning tag colour');
      if(location.pathname==='/'&&!document.querySelector('main')?.innerText.includes('what stands out'))errors.push('English terminal labels missing');
      return [...new Set(errors)];
    }""", width)

def inspect_sticky_tabs(page):
    page.evaluate("""() => {
      if (document.documentElement.scrollHeight <= innerHeight) {
        document.querySelector('main').style.minHeight = `${innerHeight * 2}px`;
      }
    }""")

    page.evaluate("window.scrollTo(0, document.documentElement.scrollHeight)")
    page.wait_for_timeout(100)
    return page.evaluate("""() => {
      const row = document.querySelector('.terminal-tabs');
      const rect = row.getBoundingClientRect();
      const x = rect.left + rect.width / 2;
      const y = rect.top + rect.height / 2;
      const hit = document.elementFromPoint(x, y);
      const errors = [];
      if (window.scrollY < 1) errors.push('page did not scroll');
      if (rect.top < -1 || rect.bottom > innerHeight + 1) errors.push('tab row outside viewport');
      if (!row.contains(hit)) errors.push('tab row covered by content');
      if (innerWidth < 700 && row.scrollWidth > row.clientWidth + 1) {
        row.scrollLeft = row.scrollWidth;
        if (row.scrollLeft < 1) errors.push('tab row cannot scroll horizontally');
      }
      return errors;
    }""")

def inspect_detail_metrics(page):
    page.locator('.agent-table .terminal-link').first.click()
    page.locator('#session-detail .session-metric').first.wait_for()
    return page.evaluate("""() => {
      const rows=[...document.querySelectorAll('#session-detail .session-metric')];
      const errors=[];
      if(document.documentElement.scrollWidth>innerWidth+1) errors.push('page horizontal scroll with detail open');
      if(rows.length!==8) errors.push(`expected 8 metric rows, got ${rows.length}`);
      const x=[];
      for(const row of rows){
        const cells=[...row.children];
        if(cells.length!==3) {errors.push('metric row missing feed column');continue}
        const [label,value,feed]=cells;
        if(value.textContent.trim()!=='-' && !feed.textContent.trim()) errors.push(`missing feed: ${label.textContent}`);
        if(value.textContent.trim()==='-' && feed.textContent.trim()) errors.push(`feed on missing value: ${label.textContent}`);
        if(row.scrollWidth>row.clientWidth+1 || feed.getBoundingClientRect().right>innerWidth+1) errors.push(`metric clipped: ${label.textContent}`);
        x.push(feed.getBoundingClientRect().left);
      }
      if(x.length && Math.max(...x)-Math.min(...x)>1) errors.push('feed columns misaligned');
      return errors;
    }""")

def main():
    server=ThreadingHTTPServer(("127.0.0.1",0),Handler)
    threading.Thread(target=server.serve_forever,daemon=True).start()
    errors=[]
    try:
        with sync_playwright() as playwright:
            browser=playwright.chromium.launch(headless=True)
            for theme in ("light","dark"):
                page=browser.new_page(viewport={"width":1440,"height":900},device_scale_factor=1)
                detail_requests=[]
                page.on("request",lambda request: detail_requests.append(request.url) if "/agents/" in urlparse(request.url).path else None)
                page.add_init_script("window.localStorage.setItem('fg-theme', '%s');window.EventSource=class {constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){}};"%theme)
                for view in VIEWS:
                    page.goto(f"http://127.0.0.1:{server.server_port}/{'' if view=='overview' else view}",wait_until="networkidle")
                    page.locator('main').wait_for()
                    page.wait_for_timeout(100)
                    if view=="overview":
                        status=page.locator(".terminal-status").inner_text()
                        content=page.locator("main").inner_text()
                        if "12d 01:50" not in status or "Linux 6.12.1-demo amd64" not in status: errors.append(f"{theme}: host facts missing")
                        if "2200 / 3600 MHz" not in content or "demo.example" not in content: errors.append(f"{theme}: frequency or DNS missing")
                        if detail_requests: errors.append(f"{theme}: overview requested session details: {len(detail_requests)}")
                        memory=page.get_by_role('region',name='Memory')
                        if memory.locator('td.numeric').first.is_visible() is False: errors.append(f"{theme}: RSS clipped")
                        first_name=memory.locator('td.terminal-name-cell').first
                        if not first_name.get_attribute('title') or '…' not in first_name.inner_text(): errors.append(f"{theme}: app name not truncated with full title")
                        if 'journal errors: demo-0.service · 1 in 24 h' not in content or 'more' not in page.get_by_role('region',name='attention').inner_text(): errors.append(f"{theme}: attention labels or overflow missing")
                        if 'idle-model' in content or content.count('gpt-6-sol:')>1: errors.append(f"{theme}: speed includes idle or duplicate models")
                        if 'ECB rate' in page.locator('table').all_inner_texts().__str__(): errors.append(f"{theme}: rate repeated in table cells")
                        if 'unmatched' in page.get_by_role('region',name='AI agents').inner_text(): errors.append(f"{theme}: unmatched session label")
                        agents_text=page.get_by_role('region',name='AI agents').inner_text()
                        agents_panel=page.get_by_role('region',name='AI agents')
                        if not agents_panel.get_by_role('columnheader',name='src').is_visible(): errors.append(f"{theme}: source column missing")
                        if not all(tag in agents_text for tag in ('otel+log','otel','log','proc')): errors.append(f"{theme}: source cases missing")
                        if 'otel = OpenTelemetry · log = local logs · proc = process · est. = estimated' not in agents_text: errors.append(f"{theme}: source legend missing")
                        if 'session · 0b82' in agents_text or '0b82eee0' not in agents_text: errors.append(f"{theme}: unknown-location session ID clipped")
                        network_text=page.get_by_role('region',name='Network').inner_text()
                        if 'containers (' not in network_text or 'more' not in network_text: errors.append(f"{theme}: network grouping or overflow link missing")
                        if page.get_by_role('region',name='Network').evaluate("e => {const b=e.querySelector('.terminal-box-body');return b.scrollWidth>b.clientWidth+1 || [...e.querySelectorAll('td.numeric')].some(c=>c.scrollWidth>c.clientWidth+1)}"):
                            errors.append(f"{theme}: Network clipping at 1440")
                        if 'Claude · not reported' not in page.get_by_role('region',name='Quotas').inner_text(): errors.append(f"{theme}: missing Claude quota invented or hidden")
                        if '7 days' not in page.get_by_role('region',name='Quotas').inner_text(): errors.append(f"{theme}: weekly primary quota mislabelled")
                        if '0.0.0-dev' in status or '93f9731' not in status: errors.append(f"{theme}: dev commit missing")
                    if view=="costs":
                        if 'Claude · not reported' not in page.get_by_role('region',name='Quotas').inner_text(): errors.append(f"{theme}: Costs quota status missing")
                        if 'ECB rate' in str(page.locator('table').all_inner_texts()): errors.append(f"{theme}: Costs table repeats conversion source")
                        if page.locator('main').inner_text().count('EUR at ECB rate Oct 9') != 1: errors.append(f"{theme}: Costs conversion source is not shown once")
                    errors.extend(f"{view}-{theme}: {e}" for e in inspect(page,1440))
                    if view in ("overview", "agents"):
                        page.set_viewport_size({"width":1280,"height":900})
                        errors.extend(f"{view}-{theme}-1280: {e}" for e in inspect(page,1280))
                        page.set_viewport_size({"width":1440,"height":900})
                    if view=="agents" and 'Claude · not reported' not in page.get_by_role('region',name='Quotas').inner_text(): errors.append(f"{theme}: Agents quota status missing")
                    if view=="agents": errors.extend(f"agents-{theme}-detail: {e}" for e in inspect_detail_metrics(page))
                    page.screenshot(path=str(OUT/f"{view}-{theme}.png"),full_page=True)
                    if view in ("overview", "agents"):
                        errors.extend(f"{view}-{theme}-desktop: {e}" for e in inspect_sticky_tabs(page))
                    if view=="overview":
                        page.get_by_role('region',name='Network').locator('button.terminal-link').click()
                        if not page.url.endswith('/machine?section=Network') or page.get_by_role('button',name='Network',exact=True).first.get_attribute('aria-pressed')!='true':
                            errors.append(f"{theme}: Network more link did not open Network view")
                    if view=="machine":
                        for section in SECTIONS:
                            page.get_by_role('button',name=section,exact=True).first.click()
                            errors.extend(f"{section}-{theme}: {e}" for e in inspect(page,1440))
                            page.screenshot(path=str(OUT/f"machine-{section.lower()}-{theme}.png"),full_page=True)
                page.goto(f"http://127.0.0.1:{server.server_port}/",wait_until="networkidle")
                page.keyboard.press("2")
                if not page.url.endswith("/agents"): errors.append(f"{theme}: key 2 failed")
                page.keyboard.press("1")
                page.keyboard.press("r")
                if "7d" not in page.locator('.terminal-footer').inner_text(): errors.append(f"{theme}: range key failed")
                page.keyboard.press("?")
                if not page.url.endswith("/glossary"): errors.append(f"{theme}: glossary key failed")
                page.keyboard.press("1")
                before=page.locator('body').get_attribute('data-fg')
                page.keyboard.press("t")
                if page.locator('body').get_attribute('data-fg')==before: errors.append(f"{theme}: theme key failed")
                page.goto(f"http://127.0.0.1:{server.server_port}/apps",wait_until="networkidle")
                page.keyboard.press("/")
                if page.evaluate("document.activeElement?.type")!="search": errors.append(f"{theme}: search key failed")
                page.goto(f"http://127.0.0.1:{server.server_port}/settings",wait_until="networkidle")
                page.get_by_role('group',name='Language').get_by_role('button',name='fr').click()
                page.goto(f"http://127.0.0.1:{server.server_port}/",wait_until="networkidle")
                if 'faits marquants' not in page.locator('main').inner_text(): errors.append(f"{theme}: French selection failed")
                page.close()
                mobile=browser.new_page(viewport={"width":390,"height":844},device_scale_factor=1)
                mobile.add_init_script("window.localStorage.setItem('fg-theme', '%s');window.EventSource=class {constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){}};"%theme)
                for view in ("overview", "agents"):
                    mobile.goto(f"http://127.0.0.1:{server.server_port}/{'' if view=='overview' else view}",wait_until="networkidle")
                    errors.extend(f"{view}-{theme}-mobile: {e}" for e in inspect(mobile,390))
                    if view=="agents": errors.extend(f"agents-{theme}-mobile-detail: {e}" for e in inspect_detail_metrics(mobile))
                    if view == "overview":
                        mobile.screenshot(path=str(OUT/f"overview-{theme}-mobile.png"),full_page=True)
                    errors.extend(f"{view}-{theme}-mobile: {e}" for e in inspect_sticky_tabs(mobile))
                mobile.close()
            browser.close()
    finally:
        server.shutdown();server.server_close()
    if errors: raise AssertionError('\n'.join(errors)+f"\nScreenshots: {OUT}")
    print(f"Terminal visual checks passed for {len(VIEWS)+len(SECTIONS)} views in both themes at 1440x900, plus mobile overview and sticky tabs on overview/agents at both sizes; screenshots: {OUT}")

if __name__=='__main__': main()
