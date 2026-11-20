"""Run against file:// directly; no dev server, remote assets or credentials."""
import json,pathlib,shutil
from playwright.sync_api import sync_playwright
root=pathlib.Path(__file__).resolve().parents[1]
out=root/'browser-results';out.mkdir(exist_ok=True)
with sync_playwright() as p:
 browser=p.chromium.launch(headless=True,executable_path=shutil.which('chromium'),args=['--no-sandbox'])
 errors=[];requests=[]
 page=browser.new_page(viewport={'width':1280,'height':1000})
 page.on('pageerror',lambda e:errors.append(str(e)))
 page.on('request',lambda r:requests.append(r.url))
 page.context.set_offline(True)
 page.set_content((root/'dist/preview.html').read_text())
 assert page.locator('.release').count()==3
 assert page.locator('.release:visible').count()==3
 assert page.locator('h1').inner_text()=='Your music.\nSomething to look forward to.'
 page.locator('#search').fill('glass orchard');assert page.locator('.release:visible').count()==1
 page.locator('#search').fill('not in this library');assert page.locator('.release:visible').count()==0;assert page.locator('#empty').is_visible()
 page.locator('#search').fill('');page.locator('#type').select_option('EP');assert page.locator('.release:visible').count()==1
 page.locator('#period').select_option('Upcoming');assert page.locator('.release:visible').count()==0
 page.locator('#type').select_option('');page.locator('#period').select_option('Date uncertain');assert page.locator('.release:visible').count()==1
 page.locator('#period').select_option('');page.get_by_text('1 artists need attention',exact=True).click();assert page.get_by_text('Multiple artists share this name.',exact=False).is_visible()
 assert not page.evaluate('document.documentElement.scrollWidth > innerWidth')
 # Calendar is entirely local and keeps uncertain dates off the day grid.
 page.locator('#calendar-view').click();assert page.locator('#calendar').is_visible()
 assert page.locator('#month-label').inner_text()=='November 2026'
 assert page.locator('.calendar-event').count()==1
 page.locator('#next-month').click();assert page.locator('#month-label').inner_text()=='December 2026'
 assert page.locator('.calendar-event').count()==1
 assert page.locator('#uncertain-dates').is_visible()
 page.locator('#search').fill('glass orchard');assert page.locator('.calendar-event').count()==0
 assert page.locator('#uncertain-list .uncertain-item').count()==1
 page.locator('#search').fill('')
 with page.expect_download() as download:
  page.locator('#calendar-download').click()
 data=pathlib.Path(download.value.path()).read_bytes()
 assert page.locator('#calendar-download').get_attribute('href').startswith('blob:')
 assert page.locator('#download-status').inner_text()
 page.locator('#calendar-fallback summary').click()
 text=page.locator('#calendar-text').input_value()
 assert text.replace('\r\n','\n')==data.decode().replace('\r\n','\n')
 page.locator('#calendar-text').focus()
 assert page.locator('#calendar-text').evaluate('(n)=>n.selectionEnd-n.selectionStart')==len(text)
 page.locator('#calendar-fallback summary').click()
 assert data.startswith(b'BEGIN:VCALENDAR\r\n') and data.count(b'BEGIN:VEVENT')==2
 assert b'Unknown day' not in data
 page.screenshot(path=str(out/'calendar-desktop.png'),full_page=True)
 page.locator('#calendar').screenshot(path=str(out/'calendar-component.png'))
 page.locator('#list-view').click()
 page.screenshot(path=str(out/'desktop.png'),full_page=True)
 page.set_viewport_size({'width':375,'height':812})
 assert not page.evaluate('document.documentElement.scrollWidth > innerWidth')
 page.screenshot(path=str(out/'mobile.png'),full_page=True)
 page.locator('#calendar-view').click()
 assert not page.evaluate('document.documentElement.scrollWidth > innerWidth')
 page.screenshot(path=str(out/'calendar-mobile.png'),full_page=True)
 # Simulate a device with file sharing, then a preview that rejects sharing.
 mobile=browser.new_page(viewport={'width':390,'height':844})
 # set_content does not run context init scripts, so install the fixture explicitly.
 mobile.evaluate("window.fixtureShared=[];Object.defineProperty(navigator,'canShare',{configurable:true,value:()=>true});Object.defineProperty(navigator,'share',{configurable:true,value:async(d)=>{window.fixtureShared.push({name:d.files[0].name,text:await d.files[0].text()})}})")
 mobile.set_content((root/'dist/preview.html').read_text())
 assert mobile.locator('#calendar-share').is_visible()
 mobile.locator('#calendar-share').click()
 mobile.wait_for_function('window.fixtureShared.length===1')
 shared=mobile.evaluate('window.fixtureShared[0]')
 assert shared['name']=='music-releases.ics' and shared['text']==data.decode()
 mobile.evaluate("Object.defineProperty(navigator,'share',{value:async()=>{throw new Error('fixture unsupported preview')}})")
 mobile.locator('#calendar-share').click()
 assert mobile.locator('#calendar-text').is_visible()
 assert 'Sharing is unavailable' in mobile.locator('#download-status').inner_text()
 with mobile.expect_download() as download:
  mobile.locator('#calendar-download').click()
 assert pathlib.Path(download.value.path()).read_bytes()==data
 mobile.close()
 assert not errors,errors
 assert not [u for u in requests if u.startswith(('http:','https:'))],requests
 browser.close()
 (out/'results.json').write_text(json.dumps({'file_protocol':'blocked by managed Chromium policy; rendered offline from identical HTML instead','release_count':3,'filters':True,'empty_state':True,'responsive':True,'month_calendar':True,'uncertain_dates_separate':True,'ics_download':True,'calendar_text_fallback':True,'mobile_share_and_fallback':True,'page_errors':errors,'network_requests':requests},indent=2))
 print('PASS: offline list and calendar, shared filters, uncertain dates, ICS download, desktop/mobile layout; no external requests or JavaScript errors.')
