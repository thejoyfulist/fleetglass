"""Exercise milestone 9 settings, explanations and share card with local fixtures."""
import os
import sys
import tempfile
if __name__ == "__main__" and os.environ.get("FLEETGLASS_HEAVY_LOCK") and not os.environ.get("FLEETGLASS_LOCK_HELD"):
    os.environ["FLEETGLASS_LOCK_HELD"] = "1"
    os.execvp("flock", ["flock", os.environ["FLEETGLASS_HEAVY_LOCK"], sys.executable, *sys.argv])
import re
import json
import threading
from pathlib import Path
from http.server import ThreadingHTTPServer
from playwright.sync_api import sync_playwright
from visual_check import Handler

OUT = Path(os.environ.get('FLEETGLASS_SCREENSHOTS_DIR', tempfile.mkdtemp(prefix='fleetglass-screenshots-')))
OUT.mkdir(parents=True, exist_ok=True)
server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
base = f'http://127.0.0.1:{server.server_port}'
errors = []
try:
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        for language in ('en', 'fr'):
            for theme in ('light', 'dark'):
                for width, height in ((1440, 900), (390, 844)):
                    page = browser.new_page(viewport={'width':width,'height':height}, device_scale_factor=1)
                    page.on('pageerror', lambda e: errors.append(f'{language}-{theme}-{width}: {e}'))
                    page.add_init_script('''{
                        const args = %s;
                        localStorage.setItem('fg-theme', args.theme);
                        localStorage.setItem('fleetglass.preferences.v1', JSON.stringify({language:args.language,temperature:'C',network:'bytes',cpu:'core',hiddenTabs:[],hiddenSections:[]}));
                        window.EventSource=class { constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){} };
                    }''' % json.dumps({'language':language,'theme':theme}))
                    page.goto(base, wait_until='networkidle')
                    page.locator('.hero').wait_for()
                    page.wait_for_timeout(250)
                    for view in ('overview','settings','explanation','share'):
                        if view == 'settings':
                            page.locator('.top-actions button').last.click()
                            page.locator('main h1').wait_for(); assert page.locator('main h1').inner_text() == ('Réglages' if language=='fr' else 'Settings')
                        elif view == 'explanation':
                            page.goto(base, wait_until='networkidle')
                            page.locator('.hero').wait_for()
                            page.evaluate('window.scrollTo(0,0)')
                            page.locator('.kpis .explain button').first.click()
                            page.locator('.explain-popover').first.wait_for()
                            bounds=page.locator('.explain-popover').first.bounding_box()
                            if bounds['x']<0 or bounds['x']+bounds['width']>width or bounds['y']<0: errors.append(f'{language}-{theme}-{width}: explanation outside viewport {bounds}')
                        elif view == 'share':
                            page.locator('.top-actions button').first.click()
                            page.locator('.share-preview img').wait_for()
                            img=page.locator('.share-preview img').get_attribute('src')
                            if not img.startswith('data:image/png;base64,'): errors.append(f'{language}-{theme}-{width}: card is not PNG')
                            dimensions=page.evaluate('''() => {const i=document.querySelector('.share-preview img');return [i.naturalWidth,i.naturalHeight]}''')
                            if dimensions != [1200,630]: errors.append(f'{language}-{theme}-{width}: card size {dimensions}')
                        page.wait_for_timeout(150)
                        if language=='fr': page.screenshot(path=str(OUT/f'{view}-{theme}-{width}.png'),full_page=True)
                        if re.search(r'\b\d{1,2}:\d{2}\s*[AP]M\b',page.locator('body').inner_text(),re.I): errors.append(f'{language}-{theme}-{width}-{view}: 12-hour time')
                        metrics=page.evaluate('''() => {const h=document.querySelector('.top').getBoundingClientRect();return {width:document.documentElement.scrollWidth,header:[h.left,h.right],outside:[...document.querySelectorAll('.top *')].filter(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return s.display!=='none'&&r.width&&r.height&&!e.closest('svg')&&(r.left<h.left+6||r.right>h.right-6)}).map(e=>e.tagName+'.'+e.className),clipped:[...document.querySelectorAll('main .t')].filter(e=>e.scrollHeight>e.clientHeight+2&&!['auto','scroll'].includes(getComputedStyle(e).overflowY)).map(e=>e.getAttribute('aria-label')||e.className)}}''')
                        if metrics['width']>width+1 or metrics['outside'] or metrics['clipped']: errors.append(f'{language}-{theme}-{width}-{view}: {metrics}')
                    page.close()
        page=browser.new_page(viewport={'width':1440,'height':900})
        page.goto(base+'/settings',wait_until='networkidle')
        page.get_by_role('heading',name='Settings').wait_for()
        page.get_by_label('Language').select_option('fr')
        page.get_by_role('heading',name='Réglages').wait_for()
        page.get_by_label('Langue').select_option('en')
        page.get_by_role('heading',name='Settings').wait_for()
        page.get_by_role('combobox',name='Temperature').select_option('F')
        page.get_by_role('combobox',name='Network rate').select_option('bits')
        page.get_by_role('combobox',name='CPU usage').select_option('machine')
        page.goto(base,wait_until='networkidle')
        page.locator('.hero').wait_for()
        if '°F' not in page.locator('.kpis').inner_text(): errors.append('Temperature preference did not convert the figure')
        if 'bit/s' not in page.get_by_label('Network',exact=True).inner_text(): errors.append('Network preference did not convert the rate')
        page.get_by_role('link',name='Apps').click()
        page.locator('.app-row').first.wait_for()
        if '30.0%' not in page.locator('.app-row').first.inner_text(): errors.append('Whole-machine CPU preference did not divide by core count')
        page.locator('.top-actions button').last.click()
        page.get_by_role('checkbox',name='Apps').uncheck()
        if page.get_by_role('link',name='Apps').count(): errors.append('Hidden tab is still visible')
        page.get_by_role('checkbox',name='Memory').uncheck()
        page.goto(base,wait_until='networkidle')
        page.locator('.hero').wait_for()
        if page.get_by_label('Memory',exact=True).count(): errors.append('Hidden section is still visible')
        page.locator('.top-actions button').last.click()
        page.get_by_role('button',name='Reset settings').click()
        if page.get_by_role('link',name='Apps').count()!=1 or page.get_by_role('combobox',name='Temperature').input_value()!='C': errors.append('Reset did not restore defaults')
        page.close()
        browser.close()
finally:
    server.shutdown(); server.server_close()
if errors: raise AssertionError('\n'.join(errors))
print('Milestone 9 visual check passed: EN/FR, light/dark, 1440/390, 1200x630 PNG, no clipping or 12-hour time')
