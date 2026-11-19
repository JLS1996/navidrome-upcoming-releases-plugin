#!/usr/bin/env python3
"""Bundle reviewed source paths only. Never package runtime logs or account data."""
import hashlib,pathlib,zipfile
root=pathlib.Path(__file__).resolve().parents[1];dist=root/'dist';dist.mkdir(exist_ok=True)
files=[root/p for p in ['.gitignore','LICENSE','THIRD_PARTY_NOTICES.md','README.md','TESTING.md','SECURITY.md','CHANGELOG.md','manifest.json','go.mod','go.sum']]
for directory in ['.github','cmd','internal','scripts','docs']:
 for p in (root/directory).rglob('*'):
  if p.is_file() and '__pycache__' not in p.parts and p.name!='provider_fixture.go':files.append(p)
with zipfile.ZipFile(dist/'release-radar-source.zip','w',zipfile.ZIP_DEFLATED) as z:
 for p in sorted(files):z.write(p,pathlib.Path('release-radar')/p.relative_to(root))
names=[name for name in ['release-radar.ndp','release-radar-source.zip','preview.html'] if (dist/name).exists()]
(dist/'SHA256SUMS').write_text(''.join(hashlib.sha256((dist/name).read_bytes()).hexdigest()+'  '+name+'\n' for name in names))
print('Source bundle contains project files only: no runtime reports, state, credentials or logs.')
