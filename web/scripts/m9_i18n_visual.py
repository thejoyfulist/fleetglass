"""Capture French milestone 9 views against isolated local fixtures."""
import os
import sys
import tempfile
if __name__ == "__main__" and os.environ.get("FLEETGLASS_HEAVY_LOCK") and not os.environ.get("FLEETGLASS_LOCK_HELD"):
    os.environ["FLEETGLASS_LOCK_HELD"] = "1"
    os.execvp("flock", ["flock", os.environ["FLEETGLASS_HEAVY_LOCK"], sys.executable, *sys.argv])
import json
import re
import threading
from pathlib import Path
from http.server import ThreadingHTTPServer
from playwright.sync_api import sync_playwright
import visual_check as fixture

OUT = Path(os.environ.get('FLEETGLASS_SCREENSHOTS_DIR', tempfile.mkdtemp(prefix='fleetglass-screenshots-')))
OUT.mkdir(parents=True, exist_ok=True)
fixture.OBSERVATIONS = {
    'projects': [
        {'pid': 321, 'name': 'node', 'project': 'atelier-demo', 'worktree': 'feature-ui',
         'kind': 'dev_server', 'cpu_percent': 0.5, 'memory_bytes': 256 * 1024**2,
         'ports': [3000], 'ports_measured': True, 'connections_measured': True,
         'idle_minutes': 185, 'connection_idle_minutes': 185, 'idle': True,
         'stop_command': 'kill -TERM -- 321'},
    ],
    'insights': [
        {'kind': 'idle_dev_server', 'detail_code': 'idle_dev_server',
         'detail': 'node idle for at least 3 hours', 'app': 'node',
         'app_kind': 'project', 'since': fixture.iso(-10800)},
    ],
    'heat': [],
    'storage': fixture.OBSERVATIONS['storage'],
}
views = ('overview', 'agents', 'apps', 'projects', 'machine', 'system', 'settings')
english_interface = re.compile(
    r'\b(?:Overview|Settings|What stands out|Idle dev servers|Every running process|'
    r'All types|Copy stop command|Root filesystem|Network rate|No observations|'
    r'View all changes|Process names and PIDs|Resource history|Expires in|'
    r'Observed|Upgraded \d+ packages|\.service restarted|recent_errors|'
    r'demo_guard_alert|healthy|matching log entry observed|configured status value|'
    r'recent timestamp|TCP connect succeeded)\b', re.I)
server = ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
base = f'http://127.0.0.1:{server.server_port}'
errors = []
try:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        for theme in ('light', 'dark'):
            for width, height in ((1440, 900), (390, 844)):
                page = browser.new_page(viewport={'width': width, 'height': height}, device_scale_factor=1)
                page.on('pageerror', lambda error: errors.append(f'{theme}-{width}: {error}'))
                page.add_init_script('''{
                    const theme = %s;
                    localStorage.setItem('fg-theme', theme);
                    localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:'fr',temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                    window.EventSource = class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                }''' % json.dumps(theme))
                page.goto(base, wait_until='networkidle')
                page.locator('.hero').wait_for()
                for view in views:
                    if view == 'overview':
                        page.locator('nav.tabs a[href="/"]').click()
                        page.locator('.hero').wait_for()
                    elif view == 'settings':
                        page.locator('.top-actions button').last.click()
                        page.locator('main h1').wait_for()
                    else:
                        page.locator(f'nav.tabs a[href="/{view}"]').click()
                        page.locator('main h1').wait_for()
                    page.wait_for_timeout(150)
                    slug = f'{view}-{theme}-{width}'
                    page.screenshot(path=str(OUT / f'{slug}.png'), full_page=True)
                    body = page.locator('body').inner_text()
                    (OUT / f'{slug}.txt').write_text(body)
                    match = english_interface.search(body)
                    if match:
                        errors.append(f'{slug}: untranslated interface text {match.group()!r}')
                    # Fixture process, project and journal names are observed data; UI labels are not.
                    observed_names = {'sample_service'}
                    technical = [word for word in re.findall(r'\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b', body) if word not in observed_names]
                    if technical:
                        errors.append(f'{slug}: technical identifiers visible {sorted(set(technical))}')
                    if re.search(r'^\s*(?:active|healthy)\s*$', body, re.I | re.M):
                        errors.append(f'{slug}: untranslated state')
                    if re.search(r'\b\d{1,2}:\d{2}\s*[AP]M\b', body, re.I):
                        errors.append(f'{slug}: 12-hour clock')
                    metrics = page.evaluate('''() => ({
                        width: document.documentElement.scrollWidth,
                        clipped: [...document.querySelectorAll('main .t')].filter(e =>
                            e.scrollHeight > e.clientHeight + 2 && !['auto','scroll'].includes(getComputedStyle(e).overflowY)
                        ).map(e => e.getAttribute('aria-label') || e.className)
                    })''')
                    if metrics['width'] > width + 1 or metrics['clipped']:
                        errors.append(f'{slug}: {metrics}')
                page.close()
        browser.close()
finally:
    server.shutdown()
    server.server_close()
if errors:
    raise AssertionError('\n'.join(errors))
print(f'French visual check passed: {len(views) * 4} screenshots in {OUT}')
