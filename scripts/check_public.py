#!/usr/bin/env python3
"""Reject private/runtime artifacts before publishing a clean source directory.
This is a defense-in-depth check, not a replacement for reviewing the file list.
It never prints matched values. Synthetic credentials in tests are intentional.
"""
import argparse,io,pathlib,re,struct,subprocess,zipfile

DENIED_PARTS={'test-runtime','browser-results','__pycache__','.cache','.aws','.codex'}
DENIED_NAMES={'auth.json','state.json','integrations.json','covers.json','client.json','google-oauth.json','navidrome.toml'}
PATTERNS={
 'private key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
 'GitHub token':rb'\b(?:gh[pousr]_[A-Za-z0-9_]{30,}|github_pat_[A-Za-z0-9_]{30,})',
 'cloud access key':rb'\b(?:AKIA|ASIA)[A-Z0-9]{16}\b',
 'Google API key':rb'\bAIza[A-Za-z0-9_-]{30,}',
 'JWT credential':rb'\beyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}',
 'local workspace path':rb'/(?:workspace|home|Users)/[A-Za-z0-9_./-]+',
}
EMAIL=re.compile(rb'[A-Za-z0-9_.+%-]+@([A-Za-z0-9.-]+\.[A-Za-z]{2,})')
ALLOWED_EMAIL_DOMAINS={b'example.com',b'example.org',b'example.net',b'users.noreply.github.com'}

def inspect_bytes(name,data):
 errors=[]
 for label,pattern in PATTERNS.items():
  if re.search(pattern,data):errors.append(f'{name}: possible {label}')
 for match in EMAIL.finditer(data):
  if match.group(1).lower() not in ALLOWED_EMAIL_DOMAINS:
   errors.append(f'{name}: non-example email address needs manual review');break
 return errors

def inspect_file(root,path):
 relative=path.relative_to(root);parts=set(relative.parts)
 if '.git' in parts:return []
 if parts & DENIED_PARTS or path.name in DENIED_NAMES or path.name.startswith('.env') or path.suffix.lower() in {'.log','.db','.sqlite','.sqlite3','.pem','.key','.flac','.mp3','.wav','.zip'}:
  return [f'{relative}: runtime/private file is not publishable']
 data=path.read_bytes()
 if path.suffix=='.ndp':
  problems=[]
  with zipfile.ZipFile(io.BytesIO(data)) as archive:
   if set(archive.namelist())!={'manifest.json','plugin.wasm','LICENSE','THIRD_PARTY_NOTICES.md'}:problems.append(f'{relative}: unexpected package files')
   for item in archive.namelist():problems+=inspect_bytes(f'{relative}!{item}',archive.read(item))
  return problems
 if path.suffix=='.png':
  # Browser screenshots need no textual metadata or EXIF in a public source tree.
  offset=8
  while offset+12<=len(data):
   size=struct.unpack('>I',data[offset:offset+4])[0];kind=data[offset+4:offset+8]
   if kind in {b'eXIf',b'tEXt',b'zTXt',b'iTXt'}:return [f'{relative}: image metadata needs review']
   offset+=size+12
  return []
 return inspect_bytes(str(relative),data)

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('directory',nargs='?',default='.');parser.add_argument('--tracked',action='store_true');args=parser.parse_args();root=pathlib.Path(args.directory).resolve()
 if args.tracked:
  raw=subprocess.check_output(['git','-C',str(root),'ls-files','-z']);files=[root/pathlib.Path(x.decode()) for x in raw.split(b'\0') if x]
 else:files=[p for p in root.rglob('*') if p.is_file() and '.git' not in p.relative_to(root).parts]
 errors=[]
 for path in files:
  if path.is_symlink():errors.append(f'{path.relative_to(root)}: symlink requires manual review');continue
  errors+=inspect_file(root,path)
 if errors:
  print('\n'.join(sorted(set(errors))));raise SystemExit(1)
 print(f'Public-file checks passed for {len(files)} files; review filenames and commit metadata before pushing.')
if __name__=='__main__':main()
