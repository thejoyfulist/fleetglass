"""Capture milestone 9d views against isolated, deterministic API fixtures."""
import os
import sys
import tempfile
if __name__ == "__main__" and os.environ.get("FLEETGLASS_HEAVY_LOCK") and not os.environ.get("FLEETGLASS_LOCK_HELD"):
    os.environ["FLEETGLASS_LOCK_HELD"] = "1"
    os.execvp("flock", ["flock", os.environ["FLEETGLASS_HEAVY_LOCK"], sys.executable, *sys.argv])
import json
import threading
from http.server import ThreadingHTTPServer
from pathlib import Path

from playwright.sync_api import sync_playwright
import visual_check as fixture

fixture.OBSERVATIONS = {**fixture.OBSERVATIONS, 'projects': [
    {'pid': 321, 'name': 'node', 'project': 'fleetglass', 'worktree': 'fleetglass-m9d', 'kind': 'dev_server',
     'cpu_percent': 1.2, 'memory_bytes': 256 * 1024**2, 'ports': [3000], 'ports_measured': True,
     'connections_measured': True, 'idle_minutes': 90, 'connection_idle_minutes': 90, 'idle': True,
     'stop_command': 'kill -TERM -- 321', 'agent_session_id': 'session-1', 'agent_kind': 'claude',
     'left_behind': True, 'started_at': fixture.iso(-7200)}]}
Handler = fixture.Handler

out = Path(os.environ.get('FLEETGLASS_SCREENSHOTS_DIR', tempfile.mkdtemp(prefix='fleetglass-screenshots-')))
out.mkdir(parents=True, exist_ok=True)
server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
origin = f'http://127.0.0.1:{server.server_port}'
errors = []
try:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        for language in ('en', 'fr'):
            for theme in ('light', 'dark'):
                for width, height in ((1440, 900), (390, 844)):
                    page = browser.new_page(viewport={'width': width, 'height': height})
                    page.on('pageerror', lambda error: errors.append(str(error)))
                    page.add_init_script('''{
                        const args = %s;
                        localStorage.setItem('fg-theme', args.theme);
                        localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:args.language,temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                        window.EventSource = class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                    }''' % json.dumps({'language':language, 'theme':theme}))
                    for view, path, selector in (
                        ('overview-speed', '/', '.speed-grid'),
                        ('agents-session', '/agents?session=session-1', '.session-detail'),
                        ('projects-worktrees', '/projects', '.worktree-list'),
                        ('costs-subscriptions', '/costs', '.subscription-grid'),
                    ):
                        page.goto(origin + path, wait_until='networkidle')
                        page.locator(selector).wait_for(timeout=10000)
                        page.screenshot(path=str(out / f'{view}-{language}-{theme}-{width}.png'), full_page=True)
                        extent = page.evaluate('document.documentElement.scrollWidth')
                        if extent > width + 1:
                            errors.append(f'{view}-{language}-{theme}-{width}: horizontal overflow {extent}')
                    page.goto(origin, wait_until='networkidle')
                    page.locator('.hero').wait_for()
                    page.evaluate('navigator.serviceWorker.ready')
                    page.reload(wait_until='networkidle')
                    page.wait_for_function('navigator.serviceWorker.controller !== null')
                    page.context.set_offline(True)
                    page.reload(wait_until='domcontentloaded')
                    page.get_by_role('heading', name='Hors connexion' if language == 'fr' else 'Offline').wait_for()
                    if ('Dernières données le' if language == 'fr' else 'Last data at') not in page.locator('main').inner_text():
                        errors.append(f'offline shell has no last data timestamp: {language}-{theme}-{width}')
                    page.screenshot(path=str(out / f'offline-{language}-{theme}-{width}.png'), full_page=True)
                    page.context.set_offline(False)
                    page.close()
        browser.close()
finally:
    server.shutdown()
    server.server_close()
if errors:
    raise SystemExit('\n'.join(errors))
print(f'M9d views passed: 40 screenshots in {out}')
