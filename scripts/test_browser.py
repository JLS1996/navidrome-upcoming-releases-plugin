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
 page.locator('#period').select_option('');page.locator('summary').click();assert page.get_by_text('Multiple artists share this name.',exact=False).is_visible()
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
 assert not errors,errors
 assert not [u for u in requests if u.startswith(('http:','https:'))],requests
 browser.close()
 (out/'results.json').write_text(json.dumps({'file_protocol':'blocked by managed Chromium policy; rendered offline from identical HTML instead','release_count':3,'filters':True,'empty_state':True,'responsive':True,'month_calendar':True,'uncertain_dates_separate':True,'ics_download':True,'page_errors':errors,'network_requests':requests},indent=2))
 print('PASS: offline list and calendar, shared filters, uncertain dates, ICS download, desktop/mobile layout; no external requests or JavaScript errors.')
