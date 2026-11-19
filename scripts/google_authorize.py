#!/usr/bin/env python3
"""Obtain an OAuth refresh token locally; never run this on a public web server.
Create a Google OAuth Desktop app, enable Calendar API, then run:
  python3 scripts/google_authorize.py --client-json client.json --output google-oauth.json
No token or client secret is printed. The user completes consent in their browser.
"""
import argparse,base64,hashlib,http.server,json,os,pathlib,secrets,time,urllib.parse,urllib.request,webbrowser

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--client-json',required=True);p.add_argument('--output',default='google-oauth.json');p.add_argument('--no-browser',action='store_true');args=p.parse_args()
 output=pathlib.Path(args.output)
 if output.exists():raise SystemExit('Output file exists; choose a new output path to avoid overwriting credentials.')
 raw=json.loads(pathlib.Path(args.client_json).read_text());client=raw.get('installed')
 if not client or not client.get('client_id') or not client.get('client_secret'):raise SystemExit('Use credentials for a Google OAuth Desktop app (installed client).')
 state=secrets.token_urlsafe(32);verifier=secrets.token_urlsafe(64);challenge=base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b'=').decode();result={}
 class Callback(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
   q=urllib.parse.parse_qs(urllib.parse.urlsplit(self.path).query)
   if urllib.parse.urlsplit(self.path).path!='/oauth2callback' or not secrets.compare_digest(q.get('state',[''])[0],state):self.send_error(403);return
   if 'error'in q:result['error']=True
   else:result['code']=q.get('code',[''])[0]
   self.send_response(200);self.send_header('Content-Type','text/plain; charset=utf-8');self.send_header('Cache-Control','no-store');self.end_headers();self.wfile.write(b'Authorization received. Return to your terminal; you can close this tab.')
  def log_message(self,*args):pass
 server=http.server.HTTPServer(('127.0.0.1',0),Callback);server.timeout=1
 redirect=f'http://127.0.0.1:{server.server_port}/oauth2callback'
 query=urllib.parse.urlencode({'client_id':client['client_id'],'redirect_uri':redirect,'response_type':'code','scope':'https://www.googleapis.com/auth/calendar.events','access_type':'offline','prompt':'consent','state':state,'code_challenge':challenge,'code_challenge_method':'S256'})
 authorization='https://accounts.google.com/o/oauth2/v2/auth?'+query
 print('Open this authorization URL on this computer:\n'+authorization)
 if not args.no_browser:webbrowser.open(authorization)
 deadline=time.monotonic()+300
 try:
  while not result and time.monotonic()<deadline:server.handle_request()
 finally:server.server_close()
 if result.get('error') or not result.get('code'):raise SystemExit('Authorization was declined or timed out; no credentials were saved.')
 body=urllib.parse.urlencode({'client_id':client['client_id'],'client_secret':client['client_secret'],'code':result['code'],'redirect_uri':redirect,'grant_type':'authorization_code','code_verifier':verifier}).encode()
 req=urllib.request.Request('https://oauth2.googleapis.com/token',method='POST',data=body,headers={'Content-Type':'application/x-www-form-urlencoded'})
 try:
  with urllib.request.urlopen(req,timeout=20) as response:token=json.load(response)
 except Exception:raise SystemExit('Token exchange failed. Check Google client setup; no credentials were printed.') from None
 if not token.get('refresh_token'):raise SystemExit('Google returned no refresh token; revoke the old app consent and authorize again.')
 config={'google_client_id':client['client_id'],'google_client_secret':client['client_secret'],'google_refresh_token':token['refresh_token']}
 fd=os.open(output,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'w') as f:json.dump(config,f,indent=2)
 print('Credentials saved privately to '+str(output)+'. Copy the three fields into Navidrome plugin settings, select google, and set your dedicated calendar ID.')
if __name__=='__main__':main()
