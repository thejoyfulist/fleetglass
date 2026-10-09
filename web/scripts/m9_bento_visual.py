"""Check that every desktop Overview row fills its grid, including sparse fixtures."""
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

OUT = Path(os.environ.get('FLEETGLASS_SCREENSHOTS_DIR', tempfile.mkdtemp(prefix='fleetglass-screenshots-')))
OUT.mkdir(parents=True, exist_ok=True)
original_results = fixture.RESULTS
original_changes = fixture.CHANGES
original_speed = fixture.SPEED
variants = {
    'full': (),
    'no-services': ('systemd:', 'tls:'),
    'no-network': ('network:',),
    'no-containers': ('container:',),
    'services-only': ('network:', 'container:'),
    'sparse': ('systemd:', 'tls:', 'container:', 'backup:', 'security:', 'recent_errors'),
}
server = ThreadingHTTPServer(('127.0.0.1', 0), fixture.Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
origin = f'http://127.0.0.1:{server.server_port}'
errors = []
def row_gaps(page, selector):
    return page.evaluate('''selector => {
      return [...document.querySelectorAll(selector)].flatMap(grid => {
        const box = grid.getBoundingClientRect();
        if (!box.width || !box.height) return [];
        const gutter = parseFloat(getComputedStyle(grid).columnGap) || 0;
        const rows = new Map();
        for (const child of grid.children) {
          const rect = child.getBoundingClientRect();
          if (!rect.width || !rect.height) continue;
          const key = Math.round(rect.top);
          rows.set(key, [...(rows.get(key) || []), rect]);
        }
        return [...rows].flatMap(([top, rects]) => {
          rects.sort((a,b) => a.left-b.left);
          const gaps = [rects[0].left-box.left, ...rects.slice(1).map((r,i) => r.left-rects[i].right), box.right-rects.at(-1).right];
          return gaps.map((gap,i) => ({grid:grid.className,top,index:i,gap:Math.round(gap)})).filter(x => x.gap > gutter+2);
        });
      });
    }''', selector)

try:
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        for language in ('en', 'fr'):
            for width in (1440, 1280, 1024):
                for variant, prefixes in variants.items():
                    fixture.RESULTS = [r for r in original_results if not r['name'].startswith(prefixes)]
                    fixture.CHANGES = [] if variant == 'sparse' else original_changes
                    page = browser.new_page(viewport={'width': width, 'height': 900})
                    page.add_init_script('''{
                        localStorage.setItem('fg-theme', 'light');
                        localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:%s,temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                        window.EventSource = class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                    }''' % json.dumps(language))
                    page.goto(origin, wait_until='networkidle')
                    page.locator('.hero').wait_for()
                    gaps = row_gaps(page, 'main .bento, .overview-system-row, .overview-bottom-row')
                    if gaps:
                        errors.append(f'{language}-{width}-{variant}: empty grid space {gaps}')
                    if variant == 'full' and width == 1440:
                        page.screenshot(path=str(OUT / f'overview-light-1440-{language}.png'), full_page=True)
                    page.close()
        for language in ('en', 'fr'):
            for width in (1440, 1280, 1024):
                for count in range(1, 5):
                    fixture.SPEED = [{**original_speed[0], 'model': f'demo-model-{i + 1}'} for i in range(count)]
                    for view in ('overview', 'agents'):
                        page = browser.new_page(viewport={'width': width, 'height': 900})
                        page.add_init_script('''{
                            localStorage.setItem('fg-theme', 'light');
                            localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:%s,temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                            window.EventSource = class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                        }''' % json.dumps(language))
                        page.goto(origin + ('/' if view == 'overview' else '/agents'), wait_until='networkidle')
                        page.locator('.speed-item').first.wait_for()
                        found = page.locator('.speed-grid .speed-item').count()
                        if found != count:
                            errors.append(f'{view}-{language}-{width}-{count}: expected {count} speed cards, found {found}')
                        gaps = row_gaps(page, '.speed-grid')
                        if gaps:
                            errors.append(f'{view}-{language}-{width}-{count}: empty speed row space {gaps}')
                        page.close()
        fixture.SPEED = original_speed
        for language in ('en', 'fr'):
            fixture.RESULTS = original_results
            fixture.CHANGES = original_changes
            for theme in ('light', 'dark'):
                for width in (1440, 390):
                    page = browser.new_page(viewport={'width': width, 'height': 844 if width == 390 else 900})
                    page.add_init_script('''{
                        localStorage.setItem('fg-theme', %s);
                        localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:%s,temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                        window.EventSource = class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                    }''' % (json.dumps(theme), json.dumps(language)))
                    page.goto(origin, wait_until='networkidle')
                    page.locator('.hero').wait_for()
                    page.screenshot(path=str(OUT / f'overview-{theme}-{width}-{language}.png'), full_page=True)
                    page.close()
        browser.close()
finally:
    fixture.RESULTS = original_results
    fixture.CHANGES = original_changes
    fixture.SPEED = original_speed
    server.shutdown()
    server.server_close()
if errors:
    raise AssertionError('\n'.join(errors))
print(f'Overview grid passed: {len(variants) * 3 * 2} sparse cases and 48 speed layouts; 8 screenshots in {OUT}')
