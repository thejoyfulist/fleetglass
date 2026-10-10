"""Check the Terminal cockpit against synthetic local API data."""
import sys
import threading
from http.server import ThreadingHTTPServer
from pathlib import Path
from playwright.sync_api import sync_playwright

sys.path.insert(0, str(Path(__file__).parent))
import showcase  # Synthetic fixtures only.
import visual_check as fixture

OUT = Path(__file__).resolve().parents[2] / "assets" / "screenshots"

def inspect(page, width, theme):
    return page.evaluate("""({width,theme}) => {
      const errors=[];
      const rgb=s=>{const m=s.match(/rgba?\\((\\d+), (\\d+), (\\d+)/);return m?[+m[1],+m[2],+m[3]]:null};
      const forbidden=s=>{const c=rgb(s);if(!c)return false;const max=Math.max(...c),min=Math.min(...c),delta=max-min;if(delta<35)return false;
        let hue=max===c[0]?((c[1]-c[2])/delta+6)%6:max===c[1]?(c[2]-c[0])/delta+2:(c[0]-c[1])/delta+4;hue*=60;
        return (hue<170 || (hue>340 && hue<360)) && max>90;
      };
      if(document.documentElement.scrollWidth>innerWidth+1)errors.push('page horizontal scroll');
      if(width===1440){const shell=document.querySelector('.shell').getBoundingClientRect();for(const e of document.querySelectorAll('.terminal-status > *, .terminal-tabs > *')){const r=e.getBoundingClientRect();if(r.right>shell.right-8||r.left<shell.left+8)errors.push('header clipping: '+e.className);}}
      for(const body of document.querySelectorAll('.terminal-box-body')){
        if(width===1440 && body.scrollWidth>body.clientWidth+1)errors.push('panel overflow: '+body.parentElement.getAttribute('aria-label'));
      }
      const tag=document.querySelector('.terminal-warn .terminal-tag');
      if(!tag)errors.push('warning tag missing');
      else if(getComputedStyle(tag).backgroundColor!=='rgb(0, 166, 178)')errors.push('warning tag colour: '+getComputedStyle(tag).backgroundColor);
      for(const e of document.querySelectorAll('.shell main *')){
        const s=getComputedStyle(e);if(s.display==='none'||s.visibility==='hidden')continue;
        for(const prop of ['color','backgroundColor','borderTopColor','fill','stroke']){
          const v=s[prop];if(forbidden(v)){errors.push('forbidden '+prop+' '+v+' in '+e.tagName+'.'+e.className);break;}
        }
      }
      const text=document.querySelector('main')?.innerText||'';
      if(document.querySelector('.terminal-grid') && !location.pathname.includes('glossary') && (!text.includes('what stands out')||!text.includes('not collected')||!text.includes('top apps')))errors.push('English terminal labels missing');
      const tab=document.querySelector('.tabs button[aria-pressed="true"]');
      if(location.pathname==='/' && (!tab||!tab.textContent.includes('overview')))errors.push('overview tab missing');
      return errors;
    }""", {"width": width, "theme": theme})

def main():
    OUT.mkdir(parents=True, exist_ok=True)
    server=ThreadingHTTPServer(("127.0.0.1",0),fixture.Handler)
    thread=threading.Thread(target=server.serve_forever,daemon=True)
    thread.start()
    errors=[]
    try:
        with sync_playwright() as playwright:
            browser=playwright.chromium.launch(headless=True)
            for width,height in ((1440,900),(390,844)):
                for theme in ("light","dark"):
                    page=browser.new_page(viewport={"width":width,"height":height},device_scale_factor=1)
                    page.add_init_script("window.localStorage.setItem('fg-theme', '%s'); window.EventSource=class {constructor(){setTimeout(()=>this.onopen?.(),20)} addEventListener(){} close(){}};" % theme)
                    page.goto(f"http://127.0.0.1:{server.server_port}/",wait_until="networkidle")
                    page.locator('.terminal-box').first.wait_for()
                    errors.extend(f"{theme}-{width}: {e}" for e in inspect(page,width,theme))
                    if width==1440:
                        page.keyboard.press('2')
                        if not page.url.endswith('/agents'): errors.append(f'{theme}: key 2 did not open agents')
                        page.keyboard.press('1')
                        page.keyboard.press('r')
                        if '7d' not in page.locator('.terminal-footer').inner_text(): errors.append(f'{theme}: r did not cycle range')
                        page.keyboard.press('?')
                        if not page.url.endswith('/glossary'): errors.append(f'{theme}: ? did not open glossary')
                        page.keyboard.press('1')
                        before=page.locator('body').get_attribute('data-fg')
                        page.keyboard.press('t')
                        if page.locator('body').get_attribute('data-fg')==before: errors.append(f'{theme}: t did not cycle theme')
                        page.goto(f"http://127.0.0.1:{server.server_port}/settings",wait_until="networkidle")
                        page.locator('.settings-grid select').first.select_option('fr')
                        page.locator('.tabs button').first.click()
                        if 'faits marquants' not in page.locator('main').inner_text() or 'non collecté' not in page.locator('main').inner_text(): errors.append(f'{theme}: French selection did not translate terminal labels')
                        page.goto(f"http://127.0.0.1:{server.server_port}/",wait_until="networkidle")
                        page.locator('.tabs button').first.wait_for()
                        page.evaluate("theme => {localStorage.removeItem('fleetglass.preferences.v1');localStorage.setItem('fg-theme',theme)}",theme)
                        page.reload(wait_until="networkidle")
                    page.screenshot(path=str(OUT/f"overview-{theme}-{width}.png"),full_page=True)
                    for index in range(1,10):
                        page.locator('.tabs button').nth(index).click()
                        page.wait_for_timeout(50)
                        view=page.url.rsplit('/',1)[-1]
                        for issue in inspect(page,width,theme):
                            if 'warning tag missing' in issue or 'English terminal labels missing' in issue: continue
                            errors.append(f'{view}-{theme}-{width}: {issue}')
                    page.close()
            browser.close()
    finally:
        server.shutdown();server.server_close()
    if errors: raise AssertionError('\n'.join(errors))
    print('Terminal visual checks passed at 1440x900 and 390x844 in both themes')

if __name__=='__main__': main()
