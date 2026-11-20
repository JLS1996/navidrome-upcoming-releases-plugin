"""Real Navidrome/WASM integration with a local MusicBrainz-shaped fixture server.
The fixture endpoint and loopback permission exist ONLY in the test package.
Run from project root, with Go/TinyGo on PATH and NAVIDROME_BIN set.
"""
import datetime,hashlib,http.server,json,os,pathlib,secrets,shutil,subprocess,tempfile,threading,time,urllib.request,zipfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
(ROOT/'test-runtime').mkdir(exist_ok=True)
navidrome_bin=os.environ.get('NAVIDROME_BIN') or shutil.which('navidrome')
if not navidrome_bin:raise SystemExit('Set NAVIDROME_BIN to the Navidrome 0.64.2 executable.')
base=pathlib.Path(tempfile.mkdtemp(prefix='integration-',dir=ROOT/'test-runtime'))
for d in ['music','data/plugins','cache']:(base/d).mkdir(parents=True,exist_ok=True)
mbid='a74b1b7f-71a5-4011-9441-d0b5e4122711'
rid='11111111-2222-4333-8444-555555555555'
subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-f','lavfi','-i','sine=frequency=440:duration=1','-metadata','artist=Radiohead','-metadata','album_artist=Radiohead','-metadata','album=Integration fixture','-metadata','title=Generated test tone','-metadata','MUSICBRAINZ_ARTISTID='+mbid,'-metadata','MUSICBRAINZ_ALBUMARTISTID='+mbid,str(base/'music/test.flac')],check=True)
requests=[];version=[1];rate_limit=[429,503]
notifications=[];caldav={};google={};token_requests=[];calendar_writes=[]

class Handler(http.server.BaseHTTPRequestHandler):
 def reply(self,status,body=b'',headers=None):
  self.send_response(status)
  for k,v in (headers or {}).items():self.send_header(k,v)
  self.end_headers();self.wfile.write(body)
 def do_GET(self):
  from urllib.parse import urlsplit,parse_qs
  path=urlsplit(self.path).path
  if path.startswith('/caldav/'):
   if path not in caldav:return self.reply(404)
   entry=caldav[path];return self.reply(200,entry['body'],{'ETag':entry['etag']})
  if path.startswith('/calendar/v3/'):
   key=path.rsplit('/',1)[-1]
   if key not in google:return self.reply(404)
   return self.reply(200,json.dumps(google[key]).encode())
  q=parse_qs(urlsplit(self.path).query);offset=int(q.get('offset',['0'])[0]);requests.append({'at':time.time(),'path':self.path})
  assert 'Authorization' not in self.headers,'public metadata lookups must not authenticate'
  assert 'https://github.com/JLS1996/navidrome-upcoming-releases-plugin' in self.headers.get('User-Agent','')
  assert 'private-unused' not in self.headers.get('User-Agent','')
  if rate_limit:
   return self.reply(rate_limit.pop(0),headers={'Retry-After':'3'})
  date=(datetime.date.today()+datetime.timedelta(days=10+version[0])).isoformat()
  group={'id':rid,'title':'[fixture] Updated release '+str(version[0])+' <script>window.pwned=1</script>','first-release-date':date,'primary-type':'Album'}
  body=json.dumps({'release-group-count':2,'release-group-offset':offset,'release-groups':[group]}).encode()
  self.reply(200,body,{'Content-Type':'application/json'})
 def do_POST(self):
  body=self.rfile.read(int(self.headers.get('Content-Length','0')))
  if self.path=='/ntfy/':
   assert self.headers.get('Authorization')=='Bearer fixture-ntfy-secret'
   notifications.append(json.loads(body));return self.reply(200,b'{}')
  if self.path=='/token':
   token_requests.append(True);return self.reply(200,b'{"access_token":"fixture-access","expires_in":3600}')
  if self.path.startswith('/calendar/v3/'):
   assert self.headers.get('Authorization')=='Bearer fixture-access'
   event=json.loads(body);key=event['id']
   if key in google:return self.reply(409)
   event['etag']='v1';google[key]=event;calendar_writes.append('google-create');return self.reply(200,json.dumps(event).encode())
  self.reply(404)
 def do_PUT(self):
  body=self.rfile.read(int(self.headers.get('Content-Length','0')));path=self.path
  assert path.startswith('/caldav/rr-')
  assert b'METHOD:' not in body,'CalDAV storage must not contain an invitation METHOD'
  current=caldav.get(path)
  if current and self.headers.get('If-Match')!=current['etag']:return self.reply(412)
  if not current and self.headers.get('If-None-Match')!='*':return self.reply(412)
  etag='"v'+str(len(calendar_writes)+1)+'"';caldav[path]={'body':body,'etag':etag};calendar_writes.append('caldav-put');self.reply(201,headers={'ETag':etag})
 def do_PATCH(self):
  body=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0'))));key=self.path.rsplit('/',1)[-1]
  assert key.startswith('rr')
  if self.headers.get('If-Match')!=google[key]['etag']:return self.reply(412)
  google[key].update(body);google[key]['etag']='v'+str(len(calendar_writes)+1);calendar_writes.append('google-patch');self.reply(200,json.dumps(google[key]).encode())
 def do_DELETE(self):
  if self.path.startswith('/caldav/'):
   assert '/rr-' in self.path
   if self.path not in caldav:return self.reply(404)
   assert self.headers.get('If-Match')==caldav[self.path]['etag'];del caldav[self.path]
  else:
   key=self.path.rsplit('/',1)[-1];assert key.startswith('rr')
   if key not in google:return self.reply(404)
   assert self.headers.get('If-Match')==google[key]['etag'];del google[key]
  calendar_writes.append('delete');self.reply(204)
 def log_message(self,*args):pass
fixture=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=fixture.serve_forever,daemon=True).start()
fixture_url=f'http://127.0.0.1:{fixture.server_port}/ws/2/'
wasm=base/'fixture.wasm'
fixture_source=ROOT/'internal/radar/provider_fixture.go'
fixture_source.write_text('//go:build radar_fixture\n\npackage radar\nfunc init(){mbBase='+json.dumps(fixture_url)+';integrationTransportBase='+json.dumps(fixture_url.removesuffix('/ws/2/'))+'}\n')
try:
 subprocess.run(['tinygo','build','-o',str(wasm),'-target','wasip1','-buildmode=c-shared','-tags','radar_fixture','./cmd/plugin'],check=True,cwd=ROOT)
finally:fixture_source.unlink()
m=json.loads((ROOT/'manifest.json').read_text());m['permissions']['http']['requiredHosts']=['127.0.0.1']
with zipfile.ZipFile(base/'data/plugins/release-radar.ndp','w',zipfile.ZIP_DEFLATED) as z:z.writestr('manifest.json',json.dumps(m));z.write(wasm,'plugin.wasm')
config=base/'navidrome.toml';config.write_text(f"DataFolder = '{base}/data'\nMusicFolder = '{base}/music'\nCacheFolder = '{base}/cache'\nAddress = '127.0.0.1'\nPort = 4534\nLogLevel = 'debug'\nAgents = ''\n[Plugins]\nEnabled = true\n")
url='http://127.0.0.1:4534';token='';server=None;log=None
storage=base/'data/plugins/release-radar/storage';statefile=storage/'state.json'
def call(path,method='GET',payload=None):
 headers={'Content-Type':'application/json'}
 if token:headers['x-nd-authorization']='Bearer '+token
 req=urllib.request.Request(url+path,method=method,headers=headers,data=json.dumps(payload).encode() if payload is not None else None)
 with urllib.request.urlopen(req,timeout=20) as r:
  b=r.read();return json.loads(b) if b else None
def wait_for(predicate,timeout=60):
 end=time.time()+timeout
 while time.time()<end:
  try:
   result=predicate()
   if result:return result
  except (OSError,ValueError,KeyError,IndexError,urllib.error.URLError):pass
  time.sleep(.25)
 raise AssertionError('Timed out; inspect '+str(base/'server.log'))
def start():
 global server,log
 log=open(base/'server.log','a');server=subprocess.Popen([navidrome_bin,'--configfile',str(config)],stdout=log,stderr=log,umask=0o022)
 wait_for(lambda:urllib.request.urlopen(url+'/app/',timeout=2).status==200)
def stop():
 global server
 if server:server.terminate();server.wait(timeout=15);server=None;log.close()
def update(**kwargs):return call('/api/plugin/release-radar','PUT',kwargs)
def state():return json.loads(statefile.read_text())
def completed():
 s=state();return s if len(s['entries'])==1 and s['entries'][0]['phase']=='done' else None
# Legacy keys must be ignored: identity comes from grants and output stays in storage.
common={'username':'obsolete-user','contact':'private-unused@example.com','auto_match':False,'export_library_id':1,'ntfy_server':'https://ntfy.sh/ntfy','ntfy_topic':'fixture-topic','ntfy_access_token':'fixture-ntfy-secret','calendar_provider':'caldav','caldav_calendar_url':'https://caldav.icloud.com/caldav/','caldav_username':'fixture-user','caldav_app_password':'fixture-app-password'}
def settings(**changes):common.update(changes);return json.dumps(common)
def deliveries():return json.loads((storage/'integrations.json').read_text())
def owned_caldav():return {k:v for k,v in caldav.items() if '/rr-' in k}

try:
 start();auth=call('/auth/createAdmin','POST',{'username':'fixture-admin','password':secrets.token_urlsafe(32)});token=auth['token']
 # Wait for actual audio library discovery, rather than injecting artists into the DB.
 def scanned():
  p={'u':['fixture-admin'],'t':[auth['subsonicToken']],'s':[auth['subsonicSalt']],'v':['1.16.1'],'c':['radar-test'],'f':['json']}
  from urllib.parse import urlencode
  r=call('/rest/getArtists?'+urlencode(p,doseq=True));return r['subsonic-response']['artists'].get('index')
 wait_for(scanned)
 update(config=settings(),allUsers=True,allLibraries=True,allowWriteAccess=False,enabled=True)
 wait_for(lambda:state().get('musicBrainzFailures')==2,90)
 paused=state();assert paused['entries'][0]['phase']=='releases' and not paused['entries'][0].get('problem')
 assert 'automatic retry after' in (storage/'upcoming-releases.html').read_text()
 before=len(requests);stop();start();time.sleep(3);assert len(requests)==before,'service cooldown lost after restart'
 first=wait_for(completed,120);assert len(first['entries'][0]['releases'])==1
 wait_for(lambda:len(owned_caldav())==1 and deliveries()['ntfyReady'])
 assert not notifications,'initial notification backlog was not suppressed'
 assert (storage/'music-releases.ics').exists()
 assert not (base/'music/upcoming-releases.html').exists() and not (base/'music/music-releases.ics').exists(),'legacy setting wrote outside plugin storage'
 storage_mode=(storage/'upcoming-releases.html').stat().st_mode & 0o777
 assert storage_mode==0o600,'reports must be private'

 html=(storage/'upcoming-releases.html').read_text();assert '[fixture] Updated release 1' in html and '&lt;script&gt;window.pwned=1&lt;/script&gt;' in html
 assert '<script>window.pwned=1</script>' not in html
 assert len(requests)==4,requests
 assert requests[1]['at']-requests[0]['at']>=29,'minimum exponential backoff ignored'
 assert requests[2]['at']-requests[1]['at']>=59,'queue-wide backoff did not grow after HTTP 503'
 before=len(requests);stop()
 # Simulate an upgrade with a recent report and a fully completed lookup cache.
 legacy=state();legacy['viewConfig']='previous-plugin-version';legacy['renderedAt']=int(time.time());statefile.write_text(json.dumps(legacy))
 start();wait_for(lambda:state().get('viewConfig','').startswith(m['version']+'/'))
 assert len(requests)==before,'upgrade re-fetched a completed artist';assert completed()
 assert 'day-release-list' in (storage/'upcoming-releases.html').read_text(),'upgrade left the old report layout in place'
 # Force a daily cycle in the test state while the plugin is disabled.
 version[0]=2;update(config=settings(refresh_token='cycle-two'))
 def updated():
  s=completed()
  return s if s and s['entries'][0]['releases'][0]['title'].startswith('[fixture] Updated release 2') else None
 second=wait_for(updated)
 assert len(second['entries'][0]['releases'])==1
 html=(storage/'upcoming-releases.html').read_text();assert '[fixture] Updated release 1' not in html and '[fixture] Updated release 2' in html
 wait_for(lambda:len(notifications)==1 and any(b'[fixture] Updated release 2' in item['body'] for item in owned_caldav().values()))
 assert len(owned_caldav())==1,'CalDAV duplicated a date-changed event'
 # Google OAuth refresh and managed event create/update/delete, against the same fixture.
 google['unrelated-user-event']={'id':'unrelated-user-event','summary':'Do not modify'}
 update(config=settings(calendar_provider='google',google_calendar_id='fixture@example.com',google_client_id='fixture-client',google_client_secret='fixture-secret',google_refresh_token='fixture-refresh'))
 wait_for(lambda:len(google)==2)
 assert len(token_requests)==1
 version[0]=3;update(config=settings(refresh_token='cycle-three'))
 wait_for(lambda:any(event.get('summary','').startswith('Radiohead — [fixture] Updated release 3') for event in google.values()))
 assert len(google)==2,'Google duplicated a date-changed event'
 wait_for(lambda:len(notifications)==2)
 # A display setting must regenerate immediately even when no artists need requests.
 update(config=settings(ahead_days=1))
 wait_for(lambda:'class="release" data-type' not in (storage/'upcoming-releases.html').read_text())
 wait_for(lambda:len(google)==1)
 assert 'unrelated-user-event' in google
 # A user can lose a library while the plugin keeps its own library grant.
 (base/'empty-library').mkdir()
 empty=call('/api/library','POST',{'name':'Empty fixture library','path':str(base/'empty-library')})
 reader=call('/api/user','POST',{'userName':'fixture-reader','name':'Fixture reader','password':secrets.token_urlsafe(32),'isAdmin':False})
 call('/api/user/'+reader['id']+'/library','PUT',{'libraryIds':[1]})
 wait_for(lambda:'multiple users' in state().get('problem',''))
 assert not state().get('entries'),'ambiguous grants kept cached artist data'
 update(config=settings(username='',ahead_days=90,export_library_id=0,calendar_provider='none',ntfy_topic=''),allUsers=False,users=json.dumps([reader['id']]))
 wait_for(completed)
 before_user_revoke=len(requests)
 call('/api/user/'+reader['id']+'/library','PUT',{'libraryIds':[int(empty['id'])]})
 wait_for(lambda:state().get('entries')==[])
 assert call('/api/plugin/release-radar')['enabled'] is True,'test must revoke user access without disabling plugin'
 assert 'Radiohead' not in (storage/'upcoming-releases.html').read_text()
 time.sleep(3);assert len(requests)==before_user_revoke,'metadata requests continued for a revoked library'
 update(allLibraries=False,libraries='[]')
 wait_for(lambda:call('/api/plugin/release-radar').get('enabled') is False)
 before_revoked=len(requests);time.sleep(3);assert len(requests)==before_revoked,'requests continued after revocation'
 # Navidrome disables the runtime before it can clear old disk snapshots.
 results={'navidrome':'0.64.2','wasm_runtime':True,'storage_and_atomic_rename':True,'real_library_discovery':True,'http_429_backoff':True,'http_503_global_backoff_and_restart':True,'pagination_and_deduplication':True,'metadata_escaping_in_wasm':True,'restart_persistence':True,'changed_release_replaces_snapshot':True,'display_config_refresh':True,'idle_upgrade_refresh_preserves_lookup_cache':True,'library_revocation_disables_plugin':True,'old_disk_snapshot_requires_manual_removal':True,'storage_only_reports':True,'storage_file_mode':oct(storage_mode),'legacy_identity_and_contact_ignored':True,'ntfy_baseline_and_update_alerts':True,'ntfy_notifications':len(notifications),'caldav_create_and_date_update':True,'google_token_refresh_create_patch_delete':True,'unrelated_calendar_event_preserved':True,'ics_file_export':True,'fixture_requests':len(requests),'user_library_revocation_clears_cache':True,'automatic_granted_user_selection':True,'ambiguous_grants_stop_processing':True}
 (ROOT/'dist/integration-results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,indent=2))
finally:
 stop();fixture.shutdown()
