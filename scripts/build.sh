#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
tinygo build -no-debug -panic=trap -o dist/plugin.wasm -target wasip1 -buildmode=c-shared ./cmd/plugin
python3 - "$@" <<'PY'
import argparse,ipaddress,json,re,zipfile
parser=argparse.ArgumentParser(description='Package plugin with optional explicit self-hosted service permissions.')
parser.add_argument('--allow-host',action='append',default=[],help='Additional hostname, IP or CIDR; no URL, port, credentials or wildcard.')
args=parser.parse_args()
m=json.load(open('manifest.json'))
for host in args.allow_host:
    try:ipaddress.ip_network(host,strict=False)
    except ValueError:
        if len(host)>253 or not re.fullmatch(r'[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?',host):
            raise SystemExit('Invalid extra host: use only a hostname, IP or CIDR, without credentials or wildcards.')
    if host not in m['permissions']['http']['requiredHosts']:m['permissions']['http']['requiredHosts'].append(host)
with zipfile.ZipFile('dist/navidrome-upcoming-releases-plugin.ndp','w',zipfile.ZIP_DEFLATED) as z:
    z.writestr('manifest.json',json.dumps(m,indent=2)+'\n')
    z.write('dist/plugin.wasm','plugin.wasm')
    z.write('LICENSE','LICENSE')
    z.write('THIRD_PARTY_NOTICES.md','THIRD_PARTY_NOTICES.md')
PY
