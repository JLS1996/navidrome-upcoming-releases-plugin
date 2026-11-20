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
  page.reload();page.get_by_label('Navidrome username',exact=True).wait_for()
  assert page.get_by_text('Unable to render configuration form.',exact=False).count()==0
  titles={name:fixed['config']['schema']['properties'][name]['title'] for name in secret_fields}
  for name,title in titles.items():
   control=page.get_by_label(title,exact=True)
   assert control.get_attribute('type')=='password',name+' is not masked'
   control.fill('fixture-'+name)
  page.get_by_label('Navidrome username',exact=True).fill('config-test')
  with page.expect_response(lambda r:r.request.method=='PUT' and '/api/plugin/release-radar' in r.url) as saved:
   page.get_by_role('button',name=re.compile(r'^save$',re.I)).click()
  assert saved.value.status==200,'configuration could not be saved'
  record=api('/api/plugin/release-radar');values=json.loads(record['config'])
  assert not record['enabled'],'test must not enable outbound integrations'
  for name in secret_fields:assert values[name]=='fixture-'+name
  page.reload();page.get_by_label('Navidrome username',exact=True).wait_for()
  for name,title in titles.items():
   control=page.get_by_label(title,exact=True);assert control.get_attribute('type')=='password';assert control.input_value()=='fixture-'+name
  assert page.get_by_text('Unable to render configuration form.',exact=False).count()==0
  browser.close()
 print('PASS: reproduced unknown password format, rendered all controls, verified four masked credentials, and saved/reloaded settings in real Navidrome.')
finally:
 server.terminate();server.wait(timeout=15);log.close()
