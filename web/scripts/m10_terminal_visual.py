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

OUT = Path(tempfile.mkdtemp(prefix="fleetglass-m10-views-"))
VIEWS = ("overview", "agents", "apps", "projects", "machine", "storage", "services", "system", "changes", "costs", "uptime", "settings", "glossary", "fleet", "share")
SECTIONS = ("CPU", "Memory", "Disk", "Network", "Containers")

class Handler(showcase.ShowcaseHandler):
    def do_GET(self):
        path = urlparse(self.path).path
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
      }
      const tag=document.querySelector('.terminal-warn .terminal-tag');
      if(tag&&getComputedStyle(tag).backgroundColor!=='rgb(0, 166, 178)')errors.push('warning tag colour');
      if(location.pathname==='/'&&!document.querySelector('main')?.innerText.includes('what stands out'))errors.push('English terminal labels missing');
      return [...new Set(errors)];
    }""", width)

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
                    errors.extend(f"{view}-{theme}: {e}" for e in inspect(page,1440))
                    page.screenshot(path=str(OUT/f"{view}-{theme}.png"),full_page=True)
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
                mobile.goto(f"http://127.0.0.1:{server.server_port}/",wait_until="networkidle")
                errors.extend(f"overview-{theme}-mobile: {e}" for e in inspect(mobile,390))
                mobile.screenshot(path=str(OUT/f"overview-{theme}-mobile.png"),full_page=True)
                mobile.close()
            browser.close()
    finally:
        server.shutdown();server.server_close()
    if errors: raise AssertionError('\n'.join(errors)+f"\nScreenshots: {OUT}")
    print(f"Terminal visual checks passed for {len(VIEWS)+len(SECTIONS)} views in both themes at 1440x900, plus mobile overview; screenshots: {OUT}")

if __name__=='__main__': main()
