"""Cockpit layout and origin checks against a temporary local server.

Usage: python3 web/tests/browser.py http://127.0.0.1:7878 /tmp/fleetglass-shots
"""
import sys
import re
from pathlib import Path
from urllib.parse import urlsplit
from playwright.sync_api import sync_playwright

base = sys.argv[1].rstrip('/')
out = Path(sys.argv[2])
out.mkdir(parents=True, exist_ok=True)
origin = (urlsplit(base).scheme, urlsplit(base).netloc)
views = ['overview', 'agents', 'apps', 'projects', 'machine', 'storage', 'services', 'system', 'changes', 'costs', 'uptime']
failures = []

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    for theme in ('light', 'dark'):
        for width, height in ((1440, 900), (1280, 900), (1024, 900), (390, 844)):
            page = browser.new_page(viewport={'width': width, 'height': height}, device_scale_factor=1)
            page.on('pageerror', lambda error: failures.append(f'{theme} {width}: browser error {error}'))
            page.add_init_script(f"localStorage.setItem('fg-theme', '{theme}')")
            external = []
            page.on('request', lambda request: external.append(request.url) if (urlsplit(request.url).scheme, urlsplit(request.url).netloc) != origin else None)
            for view in (views if width in (1440, 390) else ['overview']):
                path = '' if view == 'overview' else view
                page.goto(f'{base}/{path}', wait_until='domcontentloaded')
                try:
                    page.locator('.app-list' if view == 'apps' else '.observation-storage' if view == 'storage' else '.bento').wait_for(timeout=15000)
                    if view == 'apps':
                        page.locator('.app-row').first.wait_for(timeout=15000)
                except Exception:
                    print(f'Failed view {view} {theme} {width}: {page.locator("body").inner_text()[:500]}')
                    raise
                page.wait_for_timeout(250)
                page.screenshot(path=str(out / f'{view}-{theme}-{width}.png'), full_page=True)
                if view == 'overview' and width in (1440, 1280, 1024):
                    header = page.evaluate('''() => {
                      const bounds = document.querySelector('.top').getBoundingClientRect();
                      return {height: bounds.height, outside: [...document.querySelectorAll('.top *')]
                        .filter(e => !e.closest('svg'))
                        .filter(e => { const s = getComputedStyle(e), r = e.getBoundingClientRect();
                          return s.display !== 'none' && s.visibility !== 'hidden' && r.width && r.height &&
                            (r.left < bounds.left + 7 || r.right > bounds.right - 7); })
                        .map(e => `${e.tagName}.${e.className}`)};
                    }''')
                    if header['outside']:
                        failures.append(f'{theme} {width}: header exceeds its bounds {header}')
                if view == 'overview' and width in (1440, 390):
                    page.screenshot(path=str(out / f'real-{theme}-{width}.png'), full_page=True)
                    overview = page.request.get(f'{base}/api/v1/hosts/test-host/overview').json()
                    state = next((r['state'] for r in overview['results'] if r['name'] == 'internal:closed_target'), None)
                    if state != 'crit' or 'check down' not in page.get_by_label('Status', exact=True).inner_text().lower():
                        failures.append(f'{theme} {width}: failed internal probe absent from API or cockpit: {state}')
                    text = page.locator('.quota').inner_text()
                    if '7-day window' not in text or '5-hour window' not in text or '64%' not in text:
                        failures.append(f'{theme} {width}: quota windows or percentage missing: {text}')
                    agent_text = page.locator('.agents').inner_text()
                    if '6 sessions' not in agent_text or 'View all 14' not in agent_text:
                        failures.append(f'{theme} {width}: grouped agents or full-view link missing: {agent_text}')
                    network_text = page.locator('.third[aria-label="Network"]').inner_text()
                    if any(name in network_text for name in ('docker0', 'wlan0', 'lo\n')):
                        failures.append(f'{theme} {width}: idle network interface in cockpit: {network_text}')
                    services_text = page.locator('.third[aria-label="Services and certificates"]').inner_text()
                    if 'expiring.test' not in services_text or services_text.index('expiring.test') > services_text.index('caddy.service'):
                        failures.append(f'{theme} {width}: certificate hidden or not first: {services_text}')
                    if 'Upgraded 2 packages' not in page.get_by_label('What changed', exact=True).inner_text():
                        failures.append(f'{theme} {width}: change tile missing fixture event')
                if view == 'changes' and width in (1440, 390):
                    if 'Upgraded 2 packages' not in page.get_by_label('Change journal', exact=True).inner_text():
                        failures.append(f'{theme} {width}: Changes view missing fixture event')
                if view == 'costs' and width in (1440, 390):
                    if 'CPU package only' not in page.get_by_label('Energy', exact=True).inner_text():
                        failures.append(f'{theme} {width}: Costs view missing measured energy')
                if view == 'uptime' and width in (1440, 390):
                    page.get_by_role('button', name='1h', exact=True).click()
                    if 'over 1h' not in page.locator('.chart svg').first.get_attribute('aria-label'):
                        failures.append(f'{theme} {width}: 1 h response history missing')
                    page.get_by_role('button', name='12h', exact=True).click()
                    if 'over 12h' not in page.locator('.chart svg').first.get_attribute('aria-label'):
                        failures.append(f'{theme} {width}: 12 h response history missing')
                if view == 'apps' and width in (1440, 390):
                    content = page.locator('.app-list').inner_text()
                    if 'fleetglass-agent.service' not in content or 'Other / not attributed' not in content or 'Not measured' not in content:
                        failures.append(f'{theme} {width}: app attribution or coverage missing: {content[:400]}')
                    if page.locator('.app-hour').count() == 0:
                        failures.append(f'{theme} {width}: app hourly history missing')
                if view == 'projects' and width in (1440, 390):
                    content = page.locator('main').inner_text()
                    if 'atlas / dashboard' not in content or ':5173' not in content or 'Copy stop command' not in content or 'runaway-build.service' not in content:
                        failures.append(f'{theme} {width}: project ownership or insight missing: {content[:500]}')
                if view == 'storage' and width in (1440, 390):
                    content = page.locator('main').inner_text()
                    if '/srv/projects/atlas' not in content or '+210 MiB' not in content:
                        failures.append(f'{theme} {width}: storage map incomplete: {content[:500]}')
                if view == 'storage':
                    if page.get_by_role('group', name='Subsystem').get_by_role('button', name='Storage', pressed=True).count() != 1:
                        failures.append(f'{theme} {width}: direct storage URL did not select Machine / Storage')
                    if page.get_by_role('link', name='Machine').get_attribute('aria-current') != 'page':
                        failures.append(f'{theme} {width}: Machine tab not active on storage URL')
                if view == 'machine' and width in (1440, 390):
                    if 'Heat attribution' not in page.locator('main').inner_text():
                        failures.append(f'{theme} {width}: heat attribution missing')
                if view == 'apps' and width in (1440, 390):
                    if page.locator('.app-row').count() < 60:
                        failures.append(f'{theme} {width}: dense app fixture missing')
                    if page.locator('.app-row').last.locator('.app-kind').inner_text() != 'Other':
                        failures.append(f'{theme} {width}: Other is not last')
                    page.get_by_role('button', name='Energy', exact=True).click()
                    page.get_by_role('searchbox', name='Search apps').fill('worker-42')
                    if page.locator('.app-row').count() != 2:
                        failures.append(f'{theme} {width}: app search did not filter')
                    page.locator('.app-row').first.click()
                    if 'PID 4042' not in page.locator('.app-detail').inner_text():
                        failures.append(f'{theme} {width}: app process detail missing')
                    page.get_by_role('searchbox', name='Search apps').fill('')
                    page.locator('.app-filters select').select_option('Container')
                    if page.locator('.app-row').count() != 2:
                        failures.append(f'{theme} {width}: app type filter failed')
                    page.locator('.app-filters select').select_option('All types')
                    if page.locator('.app-hour').count() != 12:
                        failures.append(f'{theme} {width}: initial hourly list exceeds 12 rows')
                    hour_label = page.locator('.app-hour time').first.inner_text()
                    if not re.fullmatch(r'[A-Z][a-z]{2} \d{1,2} [A-Z][a-z]{2}, \d{2}:\d{2}', hour_label):
                        failures.append(f'{theme} {width}: wrong 24-hour label {hour_label}')
                    page.get_by_role('button', name='Show 48 h').click()
                    if page.locator('.app-hour').count() <= 12:
                        failures.append(f'{theme} {width}: Show 48 h did not expand')
                    page.screenshot(path=str(out / f'apps-detail-{theme}-{width}.png'), full_page=True)
                if view == 'system' and width in (1440, 390):
                    if 'Closed target' not in page.locator('.system-view').inner_text() or 'Down' not in page.locator('.system-view').inner_text():
                        failures.append(f'{theme} {width}: failed internal check not Down')
                clipped = page.evaluate("""() => [...document.querySelectorAll('.t,.app-list,.app-detail,.app-hourly')].filter(e => {
                    const s=getComputedStyle(e);
                    return e.scrollHeight>e.clientHeight+2 && !['auto','scroll'].includes(s.overflowY);
                }).map(e=>e.getAttribute('aria-label')||e.className)""")
                if clipped:
                    failures.append(f'{view} {theme} {width}: panel content clipped without scrolling {clipped}')
                if re.search(r'\b\d{1,2}:\d{2}\s*[AP]M\b', page.locator('body').inner_text(), re.I):
                    failures.append(f'{view} {theme} {width}: 12-hour time displayed')
                if re.search(r'\b\d{1,2}/\d{1,2}/\d{4}\b', page.locator('body').inner_text()):
                    failures.append(f'{view} {theme} {width}: numeric date displayed')
                metrics = page.evaluate('''() => {
                  const tiles = [...document.querySelectorAll('.bento > .t, .bento > .kpis')];
                  const rows = new Map();
                  for (const t of tiles) { const r=t.getBoundingClientRect(), k=Math.round(r.top); (rows.get(k)||rows.set(k,[]).get(k)).push({name:t.getAttribute('aria-label')||t.className,top:r.top,bottom:r.bottom,left:r.left,right:r.right}); }
                  const overlaps=[];
                  for(const cells of rows.values()) for(let i=0;i<cells.length;i++) for(let j=i+1;j<cells.length;j++) if(cells[i].right>cells[j].left+1 && cells[j].right>cells[i].left+1) overlaps.push([cells[i].name,cells[j].name]);
                  const overflowing=[...document.querySelectorAll('.t,.app-list,.app-detail,.app-hourly')].filter(t=>getComputedStyle(t).overflowX!=='hidden' && (t.scrollWidth>t.clientWidth+1 || t.scrollHeight>t.clientHeight+1)).map(t=>t.getAttribute('aria-label')||t.querySelector('h3')?.textContent);
                  return {scroll:document.documentElement.scrollWidth, viewport:innerWidth, rows:[...rows.values()], overlaps, overflowing};
                }''')
                if metrics['scroll'] > width + 1:
                    offenders = page.evaluate('''() => [...document.querySelectorAll('*')].map(e=>({name:e.tagName.toLowerCase()+'.'+e.className?.baseVal||e.className,left:e.getBoundingClientRect().left,right:e.getBoundingClientRect().right})).filter(x=>x.right>innerWidth+1).slice(0,8)''')
                    failures.append(f'{view} {theme} {width}: horizontal page scroll {metrics["scroll"]}, {offenders}')
                if metrics['overlaps']:
                    failures.append(f'{view} {theme} {width}: overlaps {metrics["overlaps"]}')
                if metrics['overflowing']:
                    failures.append(f'{view} {theme} {width}: overflowing tiles {metrics["overflowing"]}')
                for row in metrics['rows']:
                    if len(row) > 1 and max(x['bottom'] for x in row) - min(x['bottom'] for x in row) > 1:
                        failures.append(f'{view} {theme} {width}: row bottoms differ {row}')
                if view == 'machine' and width in (1440, 390):
                    if page.locator('.subsystem-nav').get_by_role('button', name='GPU', exact=True).count():
                        failures.append(f'{theme} {width}: GPU tab shown without a GPU')
                    for section in ('CPU', 'Memory', 'Disk', 'Network', 'Containers'):
                        page.locator('.subsystem-nav').get_by_role('button', name=section, exact=True).click()
                        page.screenshot(path=str(out / f'machine-{section.lower()}-{theme}-{width}.png'), full_page=True)
                        if section == 'CPU' and page.locator('.core-grid > div').count() != 8:
                            failures.append(f'{theme} {width}: CPU core grid does not show all eight cores')
                        if page.evaluate('document.documentElement.scrollWidth > innerWidth + 1'):
                            failures.append(f'{section} {theme} {width}: horizontal scroll')
            if width == 1440:
                page.keyboard.press('Control+k')
                page.get_by_role('dialog', name='Command palette').wait_for()
                page.get_by_role('textbox', name='Search commands').fill('Overview')
                page.keyboard.press('Enter')
                if urlsplit(page.url).path != '/':
                    failures.append(f'{theme} {width}: command palette did not navigate')
                page.get_by_role('button', name='7d').click()
                if 'over 7d' not in page.locator('.chart svg').first.get_attribute('aria-label'):
                    failures.append(f'{theme} {width}: period did not update the chart')
                other_theme = 'dark' if theme == 'light' else 'light'
                page.get_by_role('button', name=other_theme.title(), exact=True).click()
                if page.locator('body').get_attribute('data-fg') != other_theme:
                    failures.append(f'{theme} {width}: theme switch did not apply')
            if external:
                failures.append(f'{theme} {width}: external requests {external}')
            page.close()
    browser.close()

if failures:
    print('\n'.join(failures))
    sys.exit(1)
print(f'{len(list(out.glob("*.png")))} screenshots and layout/origin checks passed: {out}')
