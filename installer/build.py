"""Build a standalone offline installer: Python 3 + Go are needed only here.
The target needs no Go, Git, Python or GitHub access. No secrets are bundled.
"""
import base64,hashlib,io,os,pathlib,re,shutil,subprocess,tarfile
root=pathlib.Path(__file__).resolve().parents[1]
out=root/'installer/.build';out.mkdir(parents=True,exist_ok=True)
source=out/'source';(source/'assets').mkdir(parents=True,exist_ok=True)
shutil.copyfile(root/'installer/cover-site/go.mod',source/'go.mod')
main=(root/'installer/cover-site/main.go').read_text(encoding='utf-8')
for name,path in {'melon':'melon-lab/瓜体实验室.html','mosswing':'mosswing/mosswing.html'}.items():
 data=(root/path).read_bytes();(source/'assets'/f'{name}.html').write_bytes(data)
 main=re.sub(r'('+re.escape(chr(34)+name+chr(34))+r':\s*'+chr(34)+r')[a-f0-9]{64}',lambda m:m[1]+hashlib.sha256(data).hexdigest(),main)
(source/'main.go').write_text(main,encoding='utf-8')
shutil.copyfile(root/'installer/cover-site/main_test.go',source/'main_test.go')
subprocess.run(['go','test','./...'],cwd=source,check=True)
installer=(root/'installer/install-service.sh').read_bytes().replace(b'\r\n',b'\n')
payload=io.BytesIO()
with tarfile.open(fileobj=payload,mode='w:gz') as archive:
 for arch in ['amd64','arm64']:
  binary=out/('cover-site-'+arch)
  env=os.environ.copy();env.update(GOOS='linux',GOARCH=arch,CGO_ENABLED='0')
  subprocess.run(['go','build','-trimpath','-o',str(binary),'.'],cwd=source,env=env,check=True)
  data=binary.read_bytes();digest=hashlib.sha256(data).hexdigest()
  for name,content,mode in [('cover-site',data,0o755),('cover-site.sha256',(digest+'  cover-site\n').encode(),0o644),('install.sh',installer,0o755)]:
   info=tarfile.TarInfo(arch+'/'+name);info.size=len(content);info.mode=mode;info.mtime=0;archive.addfile(info,io.BytesIO(content))
  print(arch,digest,flush=True)
blob=payload.getvalue();digest=hashlib.sha256(blob).hexdigest()
header='''#!/bin/sh
# ECAcc HTTPS game service, standalone offline installer (amd64/arm64).
# Copy this file to a Linux/systemd host and run: sudo sh install.sh NODE_IP
set -eu
umask 077
case "${1:-}" in --help|-h)
    echo 'Usage: sudo sh install.sh [NODE_IPV4_OR_DOMAIN] [--check] [--reroll]'
    echo 'Optional: --cert /absolute/fullchain.pem --key /absolute/privkey.pem'
    echo 'Installs only the local HTTPS game service; enable the REALITY target in ECAcc afterwards.'
    exit 0 ;;
esac
for tool in sed base64 tar gzip sha256sum mktemp; do command -v "$tool" >/dev/null 2>&1 || { echo "Missing dependency: $tool" >&2; exit 1; }; done
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Supported architectures: amd64, arm64' >&2; exit 1 ;; esac
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
sed '1,/^__ECACC_COVER_PAYLOAD__$/d' "$0" | base64 -d > "$temporary/payload.tar.gz"
echo '@HASH@  '"$temporary/payload.tar.gz" | sha256sum -c -
tar -xzf "$temporary/payload.tar.gz" -C "$temporary"
sh "$temporary/$arch/install.sh" "$@"
exit 0
__ECACC_COVER_PAYLOAD__
'''.replace('@HASH@',digest)
target=root/'install.sh';target.write_bytes(header.encode()+base64.encodebytes(blob))
checksum=hashlib.sha256(target.read_bytes()).hexdigest()
(root/'install.sh.sha256').write_text(checksum+'  install.sh\n',encoding='ascii')
print('Installer:',target,flush=True);print('SHA256:',checksum,flush=True)
