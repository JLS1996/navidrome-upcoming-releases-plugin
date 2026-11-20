#!/usr/bin/env python3
"""Bundle reviewed source paths only. Never package runtime logs or account data."""
import hashlib,pathlib,zipfile
root=pathlib.Path(__file__).resolve().parents[1];dist=root/'dist';dist.mkdir(exist_ok=True)
# Only Git-reviewed paths may enter a release; never recurse through local files.
import subprocess
tracked=subprocess.check_output(['git','ls-files','-z'],cwd=root).decode().split('\0')
root_files={'.gitignore','LICENSE','THIRD_PARTY_NOTICES.md','README.md','TESTING.md','SECURITY.md','CHANGELOG.md','manifest.json','go.mod','go.sum'}
files=[root/p for p in tracked if p and (p in root_files or p.startswith(('.github/','cmd/','internal/','scripts/','docs/')))]
if any(p.is_symlink() or not p.is_file() for p in files):raise SystemExit('Source paths must be regular reviewed files.')
with zipfile.ZipFile(dist/'navidrome-upcoming-releases-plugin-source.zip','w',zipfile.ZIP_DEFLATED) as z:
 for p in sorted(files):z.write(p,pathlib.Path('navidrome-upcoming-releases-plugin')/p.relative_to(root))
names=[name for name in ['navidrome-upcoming-releases-plugin.ndp','navidrome-upcoming-releases-plugin-source.zip'] if (dist/name).exists()]
(dist/'SHA256SUMS').write_text(''.join(hashlib.sha256((dist/name).read_bytes()).hexdigest()+'  '+name+'\n' for name in names))
print('Source bundle contains project files only: no runtime reports, state, credentials or logs.')
