#!/usr/bin/env python3
"""Reproduce and fix the password-format error in real Navidrome's browser UI.
Uses a disposable local account, no real credentials, and never enables the plugin.
Requires NAVIDROME_BIN, Python Playwright and Chromium; run after scripts/build.sh.
"""
import copy,json,os,pathlib,re,secrets,shutil,socket,subprocess,tempfile,time,urllib.request,zipfile
from playwright.sync_api import sync_playwright

ROOT=pathlib.Path(__file__).resolve().parents[1]
(ROOT/'test-runtime').mkdir(exist_ok=True)
base=pathlib.Path(tempfile.mkdtemp(prefix='config-form-',dir=ROOT/'test-runtime'))
for directory in ['data/plugins','music','cache']:(base/directory).mkdir(parents=True,exist_ok=True)
navidrome=os.environ.get('NAVIDROME_BIN') or shutil.which('navidrome')
if not navidrome:raise SystemExit('Set NAVIDROME_BIN to a Navidrome 0.64.2 executable.')
with socket.socket() as available:available.bind(('127.0.0.1',0));port=available.getsockname()[1]
url=f'http://127.0.0.1:{port}'
secret_fields=['ntfy_access_token','caldav_app_password','google_client_secret','google_refresh_token']
fixed=json.loads((ROOT/'manifest.json').read_text());broken=copy.deepcopy(fixed)
broken['config'].pop('uiSchema',None)
for name in secret_fields:broken['config']['schema']['properties'][name]['format']='password'
package=base/'data/plugins/release-radar.ndp'
with zipfile.ZipFile(ROOT/'dist/release-radar.ndp') as z:wasm=z.read('plugin.wasm')
def write_package(manifest):
 with zipfile.ZipFile(package,'w',zipfile.ZIP_DEFLATED) as z:z.writestr('manifest.json',json.dumps(manifest));z.writestr('plugin.wasm',wasm)
write_package(broken)
config=base/'navidrome.toml'
config.write_text(f"DataFolder = '{base}/data'\nMusicFolder = '{base}/music'\nCacheFolder = '{base}/cache'\nAddress = '127.0.0.1'\nPort = {port}\nLogLevel = 'error'\nAgents = ''\n[Plugins]\nEnabled = true\n")
auth={}
def api(path,method='GET',body=None):
 headers={'Content-Type':'application/json'}
 if auth:headers['x-nd-authorization']='Bearer '+auth['token']
 request=urllib.request.Request(url+path,method=method,headers=headers,data=json.dumps(body).encode() if body is not None else None)
 with urllib.request.urlopen(request,timeout=10) as response:
  raw=response.read();return json.loads(raw) if raw else None
def wait_for(predicate):
 deadline=time.monotonic()+30
 while time.monotonic()<deadline:
  try:
   if predicate():return
  except (OSError,ValueError):pass
  time.sleep(.2)
 raise AssertionError('Local Navidrome did not become ready; inspect its ignored test-runtime logs.')
log=(base/'server.log').open('w')
server=subprocess.Popen([navidrome,'--configfile',str(config)],stdout=log,stderr=log)
try:
 wait_for(lambda:urllib.request.urlopen(url+'/app/',timeout=2).status==200)
 auth=api('/auth/createAdmin','POST',{'username':'config-test','password':secrets.token_urlsafe(32)})
 wait_for(lambda:api('/api/plugin/release-radar')['id']=='release-radar')
 storage={'token':auth['token'],'userId':auth['id'],'username':auth['username'],'name':auth['name'],'role':'admin','subsonic-salt':auth['subsonicSalt'],'subsonic-token':auth['subsonicToken'],'is-authenticated':'true'}
 with sync_playwright() as p:
  browser=p.chromium.launch(headless=True,executable_path=shutil.which('chromium'),args=['--no-sandbox'])
  context=browser.new_context()
  context.add_init_script('const fixtureAuth='+json.dumps(storage)+';for(const [key,value] of Object.entries(fixtureAuth)){localStorage.setItem(key,value)}')
  page=context.new_page();page.goto(url+'/app/#/plugin/release-radar/show')
  page.get_by_text('Unable to render configuration form.',exact=False).wait_for()
  assert page.get_by_text('unknown format "password"',exact=False).count()>0,'did not reproduce the reported AJV failure'
  write_package(fixed);api('/api/plugin/rescan','POST')
  page.reload();page.get_by_role('checkbox',name='Download cover art',exact=True).wait_for()
  assert page.get_by_text('Unable to render configuration form.',exact=False).count()==0
  titles={name:fixed['config']['schema']['properties'][name]['title'] for name in secret_fields}
  categories=['General','Releases','Integrations','Matching']
  def category(name):
   try:page.get_by_role('tab',name=name,exact=True).click(timeout=5000)
   except Exception:
    page.screenshot(path=str(base/'category-failure.png'),full_page=True,animations='disabled')
    (base/'category-failure.txt').write_text(page.locator('body').inner_text())
    print('Category failure artifacts:',base)
    raise
  def provider(value):
   try:page.get_by_role('button',name=re.compile('^'+re.escape(fixed['config']['schema']['properties']['calendar_provider']['title'])+r'(?:\s|$)')).click(timeout=5000)
   except Exception:
    page.screenshot(path=str(base/'provider-failure.png'),full_page=True,animations='disabled')
    (base/'provider-failure.txt').write_text(page.locator('body').inner_text())
    (base/'provider-failure.html').write_text(page.content())
    print('Provider failure artifacts:',base)
    raise
   page.get_by_role('option',name=value,exact=True).click()
   page.get_by_role('listbox').wait_for(state='hidden')
  def assert_spacing():
   card=page.locator('.MuiCard-root').filter(has=page.get_by_role('heading',name='Configuration',exact=True))
   boxes=card.locator('.MuiFormControl-root:visible').evaluate_all('(nodes)=>nodes.map(n=>{const r=n.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})')
   for i,left in enumerate(boxes):
    for right in boxes[i+1:]:
     overlap_x=min(left['x']+left['w'],right['x']+right['w'])-max(left['x'],right['x'])
     if overlap_x<=1:continue
     upper,lower=sorted([left,right],key=lambda r:r['y'])
     gap=lower['y']-(upper['y']+upper['h'])
     assert gap>=8, 'configuration controls overlap or have insufficient spacing: '+str(round(gap,2))
   assert card.evaluate('(n)=>n.scrollWidth<=n.clientWidth+1'),'configuration card overflows horizontally'
  for name in categories:
   category(name);assert_spacing()
   files=page.get_by_text('Info',exact=True).bounding_box()
   tabs=page.get_by_role('tablist').bounding_box()
   access=page.get_by_text('Library access',exact=True).bounding_box()
   permission=page.get_by_role('heading',name='Users Permission',exact=True).bounding_box()
   assert files and tabs and access and permission
   assert files['y']<tabs['y']<access['y']<permission['y'],'information was not placed outside tabs above permission cards'
  assert page.get_by_role('tab',name='Local files',exact=True).count()==0
  toggle=page.get_by_role('checkbox',name='Refresh releases',exact=True)
  assert toggle.count()==1 and toggle.locator('xpath=ancestor::*[contains(@class,"MuiSwitch-root")]').count()>0
  toggle.check()
  assert toggle.bounding_box()['y']<page.get_by_role('tablist').bounding_box()['y']
  assert page.get_by_role('tab',name='Matching',exact=True).get_attribute('aria-selected')=='true'
  category('Matching');assert page.get_by_label('Artist ID overrides',exact=True).is_visible()
  category('Integrations')
  assert page.get_by_label(titles['caldav_app_password'],exact=True).count()==0
  assert page.get_by_label(titles['google_client_secret'],exact=True).count()==0
  category('Integrations')
  ntfy=page.get_by_label(titles['ntfy_access_token'],exact=True);assert ntfy.get_attribute('type')=='password';ntfy.fill('fixture-ntfy_access_token')
  category('Integrations')
  intro=page.get_by_role('heading',name=re.compile('^Local calendar:')).bounding_box()
  select=page.get_by_role('button',name=re.compile('^Calendar service')).bounding_box()
  assert select['y']-(intro['y']+intro['height'])>=20,'calendar instructions touch the floating label'
  provider('caldav')
  caldav=page.get_by_label(titles['caldav_app_password'],exact=True);caldav.wait_for();assert caldav.get_attribute('type')=='password';caldav.fill('fixture-caldav_app_password');assert_spacing()
  provider('google')
  assert page.get_by_label(titles['caldav_app_password'],exact=True).count()==0
  for name in ['google_client_secret','google_refresh_token']:
   control=page.get_by_label(titles[name],exact=True);control.wait_for();assert control.get_attribute('type')=='password';control.fill('fixture-'+name)
  assert_spacing()
  category('General')
  page.get_by_role('checkbox',name='Download cover art',exact=True).check()
  assert page.get_by_label('Navidrome username',exact=True).count()==0
  assert page.get_by_label('MusicBrainz contact',exact=True).count()==0
  assert page.get_by_label('Export library ID',exact=True).count()==0
  category('General')
  page.get_by_role('checkbox',name='Exclude releases already in library',exact=True).check()
  direct=page.get_by_role('checkbox',name='Exclude artists without directly credited music',exact=True)
  assert not direct.is_checked();direct.check()
  assert page.get_by_role('checkbox',name='Match artists without IDs',exact=True).is_visible()
  assert page.get_by_label('Days ahead',exact=True).is_visible()
  assert page.get_by_label('Days back',exact=True).is_visible()
  category('Releases')
  for title in ['Albums','EPs','Singles','Live','Compilations','Demos','Field recordings']:
   assert page.get_by_role('checkbox',name=title,exact=True).count()==1
  page.get_by_role('checkbox',name='Live',exact=True).check()
  page.get_by_role('checkbox',name='Singles',exact=True).check()
  with page.expect_response(lambda r:r.request.method=='PUT' and '/api/plugin/release-radar' in r.url) as saved:
   page.get_by_role('button',name=re.compile(r'^save$',re.I)).click()
  assert saved.value.status==200,'configuration could not be saved without a username'
  record=api('/api/plugin/release-radar');values=json.loads(record['config'])
  assert not record['enabled'],'test must not enable outbound integrations'
  for removed in ['username','contact','export_library_id','include_secondary']:assert removed not in values
  assert values['manual_refresh'] is True and values['exclude_owned'] is True and values['exclude_uncredited'] is True
  assert values['include_live'] is True and values['include_singles'] is True and values['cover_art'] is True
  for name in secret_fields:assert values[name]=='fixture-'+name
  page.reload();page.get_by_role('checkbox',name='Download cover art',exact=True).wait_for()
  assert page.get_by_role('checkbox',name='Refresh releases',exact=True).is_checked()
  page.get_by_role('checkbox',name='Refresh releases',exact=True).uncheck()
  with page.expect_response(lambda r:r.request.method=='PUT' and '/api/plugin/release-radar' in r.url) as saved_off:
   page.get_by_role('button',name=re.compile(r'^save$',re.I)).click()
  assert saved_off.value.status==200 and json.loads(api('/api/plugin/release-radar')['config'])['manual_refresh'] is False
  category('Releases');assert page.get_by_role('checkbox',name='Live',exact=True).is_checked()
  category('Integrations');assert page.get_by_label(titles['ntfy_access_token'],exact=True).input_value()=='fixture-ntfy_access_token'
  category('Integrations')
  for name in ['google_client_secret','google_refresh_token']:
   control=page.get_by_label(titles[name],exact=True);assert control.get_attribute('type')=='password';assert control.input_value()=='fixture-'+name
  provider('caldav');assert page.get_by_label(titles['caldav_app_password'],exact=True).input_value()=='fixture-caldav_app_password'
  for width in [1280,390]:
   page.set_viewport_size({'width':width,'height':1000})
   if width<600:
    # Navidrome changes its open desktop sidebar into a modal on resize.
    drawer=page.locator('.MuiDrawer-modal')
    drawer.wait_for(state='visible')
    page.keyboard.press('Escape')
    drawer.wait_for(state='hidden')
   for name in categories:category(name);assert_spacing()
   category('Integrations');provider('google');assert_spacing()
   page.screenshot(path=str(base/('settings-'+str(width)+'.png')),full_page=True,animations='disabled')
   category('Integrations');provider('caldav')
  assert page.get_by_text('Unable to render configuration form.',exact=False).count()==0
  browser.close()
 print('PASS: categorized settings render, desktop/mobile spacing, conditional calendar fields, masked credentials, and saving/reloading with automatic username selection.')
finally:
 server.terminate();server.wait(timeout=15);log.close()
