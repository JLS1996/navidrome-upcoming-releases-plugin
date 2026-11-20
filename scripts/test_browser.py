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
 assert page.locator('#release-count').inner_text()=='3'
 assert page.locator('#type option[value="Broadcast"]').count()==0
 assert page.locator('#period option[value="Today"]').count()==0
 assert page.locator('h1').inner_text()=='Your music.\nSomething to look forward to.'
 page.locator('#type').select_option('Live');assert page.locator('.release:visible').count()==1
 assert 'Live' in page.locator('.release:visible .tag').inner_text()
 page.locator('#type').select_option('')
 page.locator('#search').fill('glass orchard');assert page.locator('.release:visible').count()==1
 page.locator('#search').fill('not in this library');assert page.locator('.release:visible').count()==0;assert page.locator('#empty').is_visible();assert page.locator('#release-count').inner_text()=='0'
 page.locator('#search').fill('');page.locator('#type').select_option('EP');assert page.locator('.release:visible').count()==1
 page.locator('#period').select_option('Upcoming');assert page.locator('.release:visible').count()==0
 page.locator('#type').select_option('');page.locator('#period').select_option('Date uncertain');assert page.locator('.release:visible').count()==1
 page.locator('#period').select_option('');page.get_by_text('1 artist needs attention',exact=True).click();assert page.get_by_text('Multiple artists share this name.',exact=False).is_visible()
 assert not page.evaluate('document.documentElement.scrollWidth > innerWidth')
 # Calendar is entirely local and keeps uncertain dates off the day grid.
 page.locator('#calendar-view').click();assert page.locator('#calendar').is_visible()
 assert page.locator('#month-label').inner_text()=='November 2026'
 assert page.locator('.calendar-event').count()==1
 assert page.locator('#day-release-list .day-release').count()==1
 assert 'Slow Light' in page.locator('#day-release-list').inner_text()
 page.locator('.day-select[data-date="2026-11-06"]').click()
 page.keyboard.press('ArrowRight')
 assert page.locator('.day-select[data-date="2026-11-07"]').get_attribute('aria-pressed')=='true'
 assert page.locator('.day-select[tabindex="0"]').count()==1
 assert page.locator('#day-empty').is_visible()
 page.locator('#next-month').click();assert page.locator('#month-label').inner_text()=='December 2026'
 assert page.locator('.calendar-event').count()==1
 assert 'A Place in the Morning' in page.locator('#day-release-list').inner_text()
 page.locator('.day-select[data-date="2026-12-31"]').click();page.keyboard.press('ArrowRight')
 assert page.locator('#month-label').inner_text()=='January 2027'
 assert page.locator('.day-select[data-date="2027-01-01"]').get_attribute('aria-pressed')=='true'
 page.locator('#previous-month').click()
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
 for width in [320,375,390,430,768,1280]:
  page.set_viewport_size({'width':width,'height':844})
  assert not page.evaluate('document.documentElement.scrollWidth > innerWidth')
  assert page.locator('.calendar-scroll').evaluate('(n)=>n.scrollWidth<=n.clientWidth+1'),'month grid requires sideways scrolling at '+str(width)
  assert page.locator('.calendar-grid').evaluate('(n)=>n.scrollWidth<=n.clientWidth+1'),'calendar cells overflow at '+str(width)
  headers=page.locator('.weekday').evaluate_all('(nodes)=>nodes.map(n=>{const r=n.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top}})')
  assert len(headers)==7 and max(h['right'] for h in headers)<=width and min(h['left'] for h in headers)>=0
  assert len(set(h['top'] for h in headers))==1,'weekdays must fit in one row'
  page.locator('.day-select[data-date="2026-12-04"]').click()
  assert 'A Place in the Morning' in page.locator('#day-release-list').inner_text()
  if width<=640:
   assert page.locator('.calendar-event:visible').count()==0
   assert page.locator('.day-count:visible').count()==1
  else:assert page.locator('.calendar-event:visible').count()==1
  if width==375:
   page.screenshot(path=str(out/'calendar-mobile.png'),full_page=True)
   page.locator('#calendar').screenshot(path=str(out/'calendar-mobile-component.png'))
 # Multiple releases and long metadata must remain readable below a compact cell.
 page.set_viewport_size({'width':320,'height':844})
 page.evaluate("const original=document.querySelector('.release[data-date=\"2026-12-04\"]'),copy=original.cloneNode(true);copy.querySelector('h2 a').textContent='Long album title '.repeat(24);copy.querySelector('.artist').textContent='Long artist name '.repeat(24);document.querySelector('.list').append(copy)")
 # Rows are captured once on page load, so reload the same synthetic fixture.
 fixture=page.content()
 stress=browser.new_page(viewport={'width':320,'height':844});stress.context.set_offline(True);stress.on('pageerror',lambda e:errors.append(str(e)));stress.on('request',lambda r:requests.append(r.url));stress.set_content(fixture)
 stress.locator('#calendar-view').click();stress.locator('#next-month').click()
 stress.locator('.day-select[data-date="2026-12-04"]').click()
 assert stress.locator('#day-release-list .day-release').count()==2
 assert stress.locator('.day-count:visible').inner_text()=='2'
 assert not stress.evaluate('document.documentElement.scrollWidth > innerWidth')
 with stress.expect_download() as restored:
  stress.locator('#calendar-download').click()
 assert pathlib.Path(restored.value.path()).read_bytes()==data,'saved HTML could not rebuild its calendar download'
 stress.close()
 # Simulate a device with file sharing, then a preview that rejects sharing.
 mobile=browser.new_page(viewport={'width':390,'height':844})
 mobile.context.set_offline(True);mobile.on('pageerror',lambda e:errors.append(str(e)));mobile.on('request',lambda r:requests.append(r.url))
 # set_content does not run context init scripts, so install the fixture explicitly.
 mobile.evaluate("window.fixtureShared=[];Object.defineProperty(navigator,'canShare',{configurable:true,value:()=>true});Object.defineProperty(navigator,'share',{configurable:true,value:async(d)=>{window.fixtureShared.push({name:d.files[0].name,text:await d.files[0].text()})}})")
 mobile.set_content((root/'dist/preview.html').read_text())
 assert mobile.locator('#calendar-share').is_visible()
 assert not mobile.locator('#calendar-download').is_visible()
 mobile.locator('#calendar-share').click()
 mobile.wait_for_function('window.fixtureShared.length===1')
 shared=mobile.evaluate('window.fixtureShared[0]')
 assert shared['name']=='music-releases.ics' and shared['text']==data.decode()
 mobile.evaluate("Object.defineProperty(navigator,'share',{value:async()=>{throw new Error('fixture unsupported preview')}})")
 mobile.locator('#calendar-share').click()
 assert mobile.locator('#calendar-text').is_visible()
 assert 'Sharing is unavailable' in mobile.locator('#download-status').inner_text()
 saved_mobile=mobile.content()
 reopened=browser.new_page();reopened.context.set_offline(True);reopened.on('pageerror',lambda e:errors.append(str(e)));reopened.on('request',lambda r:requests.append(r.url));reopened.set_content(saved_mobile)
 assert reopened.locator('#calendar-share').is_hidden()
 assert reopened.locator('#calendar-download').is_visible()
 with reopened.expect_download() as restored:
  reopened.locator('#calendar-download').click()
 assert pathlib.Path(restored.value.path()).read_bytes()==data
 reopened.close();mobile.close()
 # Artwork remains embedded, fits both card layouts, and never needs browser HTTP.
 art=browser.new_page(viewport={'width':1280,'height':1000});art.context.set_offline(True)
 art.on('pageerror',lambda e:errors.append(str(e)));art.on('request',lambda r:requests.append(r.url))
 art.set_content((root/'dist/preview-covers.html').read_text())
 art.wait_for_function("[...document.querySelectorAll('img.release-cover')].every(n=>n.complete&&n.naturalWidth>0)")
 assert art.locator('.release.has-cover').count()==3
 assert art.locator('.cover-placeholder').count()==1
 assert art.locator('.cover-placeholder').inner_text()=='?'
 assert art.locator('img.release-cover').count()==2
 assert art.locator('.meta time').first.evaluate('(n)=>n.previousElementSibling.tagName')=='BR'
 for width in [320,375,390,430,768,1280]:
  art.set_viewport_size({'width':width,'height':1000})
  assert not art.evaluate('document.documentElement.scrollWidth>innerWidth'),'cover cards overflow at '+str(width)
  assert art.locator('.release').evaluate_all('(nodes)=>nodes.every(n=>{const a=n.querySelector(".release-cover").getBoundingClientRect(),b=n.querySelector(".release-body").getBoundingClientRect(),r=n.getBoundingClientRect();return a.right<=r.right&&a.left>=b.right+6})'),'cover overlaps release text'
  assert art.locator('.release.has-artwork').first.evaluate('(n)=>getComputedStyle(n,"::before").backgroundImage.includes("data:image/png")')
  if width==390:art.screenshot(path=str(out/'covers-mobile.png'),full_page=True)
 art.screenshot(path=str(out/'covers-desktop.png'),full_page=True)
 art.set_viewport_size({'width':390,'height':1000});art.locator('#calendar-view').click();art.locator('#next-month').click();art.locator('.day-select[data-date="2026-12-04"]').click()
 assert art.locator('#day-release-list .release-cover').count()==1
 assert not art.evaluate('document.documentElement.scrollWidth>innerWidth')
 art.locator('#calendar').screenshot(path=str(out/'covers-calendar-mobile.png'))
 art.close()
 # Capture a real navigation while fulfilling locally: no public request is sent.
 navigation=browser.new_page();navigation.context.set_offline(True)
 navigation.set_content((root/'dist/preview.html').read_text())
 artist=navigation.locator('.release .artist a').first
 expected=navigation.locator('.release h2 a').first.get_attribute('href')
 assert artist.get_attribute('href')==expected and artist.get_attribute('target') is None
 intercepted=[]
 def source_page(route):
  intercepted.append(route.request.url);route.fulfill(status=200,content_type='text/html',body='<title>Local MusicBrainz navigation fixture</title>')
 navigation.route('https://musicbrainz.org/**',source_page)
 artist.click();navigation.wait_for_url(expected)
 assert intercepted==[expected]
 navigation.close()
 assert not errors,errors
 assert not [u for u in requests if u.startswith(('http:','https:'))],requests
 browser.close()
 (out/'results.json').write_text(json.dumps({'file_protocol':'blocked by managed Chromium policy; rendered offline from identical HTML instead','release_count':3,'filters':True,'empty_state':True,'responsive':True,'month_calendar':True,'seven_columns_fit_320_to_1280':True,'selected_day_details':True,'calendar_keyboard_navigation':True,'uncertain_dates_separate':True,'ics_download':True,'calendar_text_fallback':True,'saved_html_download_roundtrip':True,'embedded_covers_and_mobile_layout':True,'artist_release_navigation':True,'mobile_share_and_fallback':True,'page_errors':errors,'network_requests':requests},indent=2))
 print('PASS: offline list and calendar, shared filters, uncertain dates, ICS download, desktop/mobile layout; no external requests or JavaScript errors.')
