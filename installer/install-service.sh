#!/bin/sh
# Reviewed local installer. Never changes panel/Xray inbounds.
set -eu
umask 077
identity=
cert=/root/cert/ip/fullchain.pem
key=/root/cert/ip/privkey.pem
reroll=no
check=no
usage() {
    cat <<'HELP'
Usage: sh install.sh [NODE_IPV4_OR_DOMAIN] [options]
  --identity VALUE  Certificate IP/domain (auto-detect a single IPv4 SAN)
  --cert PATH       Panel certificate; default /root/cert/ip/fullchain.pem
  --key PATH        Panel private key; default /root/cert/ip/privkey.pem
  --check           Validate environment and certificate without installation
  --reroll          Choose another game; otherwise keep the existing selection
  --help            Show this help
Requires Linux/systemd and root. Installs the local HTTPS game service.
Then enable Game fallback in ECAcc after verifying the service.
HELP
}
die() { echo "ERROR: $*" >&2; exit 1; }
while [ "$#" -gt 0 ]; do
    case "$1" in
        --help|-h) usage; exit 0 ;;
        --check) check=yes; shift ;;
        --reroll) reroll=yes; shift ;;
        --identity|--cert|--key)
            option=$1; [ "$#" -ge 2 ] || die "Missing value: $option"
            case "$option" in --identity) identity=$2 ;; --cert) cert=$2 ;; --key) key=$2 ;; esac
            shift 2 ;;
        -*) die "Unknown option: $1" ;;
        *) [ -z "$identity" ] || die 'Specify one identity only'; identity=$1; shift ;;
    esac
done
[ "$(uname -s)" = Linux ] || die 'Linux is required'
[ "$(id -u)" = 0 ] || die 'Run as root, or use sudo sh'
for command in systemctl openssl curl sha256sum ss flock mktemp cmp sed awk install; do
    command -v "$command" >/dev/null 2>&1 || die "Missing dependency: $command (Debian/Ubuntu: apt-get install ca-certificates curl openssl iproute2 util-linux coreutils)"
done
[ -d /run/systemd/system ] || die 'A running systemd host is required; do not run inside a normal Docker container'
for path in "$cert" "$key"; do
    case "$path" in /*) ;; *) die 'Certificate/key paths must be absolute' ;; esac
    case "$path" in *[!a-zA-Z0-9_./-]*) die 'Certificate/key paths must not contain spaces or special characters' ;; esac
    [ -r "$path" ] && [ -f "$path" ] || die "Cannot read: $path"
done
cd "$(dirname "$0")"
[ -f cover-site ] && [ -f cover-site.sha256 ] || die 'Missing release binary/checksum; use the single-file installer'
sha256sum -c cover-site.sha256
if [ -z "$identity" ]; then
    identities=$(openssl x509 -in "$cert" -noout -ext subjectAltName | tr ',' '\n' | sed -n 's/^[[:space:]]*IP Address:\([0-9.]*\)[[:space:]]*$/\1/p')
    count=$(printf '%s\n' "$identities" | awk 'NF {n++} END {print n+0}')
    [ "$count" = 1 ] || die 'Cannot auto-detect one IPv4 SAN; supply NODE_IPV4_OR_DOMAIN explicitly'
    identity=$identities
fi
case "$identity" in *[!a-zA-Z0-9.-]*|'') die 'Use an IPv4 address or DNS name matching the certificate' ;; esac
openssl x509 -in "$cert" -checkend 86400 -noout || die 'Certificate is expired or expires within 24 hours'
scratch=$(mktemp -d)
backup=
started=no
cleanup() { rm -f "$scratch"/*; rmdir "$scratch"; }
trap cleanup EXIT
openssl x509 -in "$cert" -pubkey -noout > "$scratch/cert-public.pem"
openssl pkey -pubin -in "$scratch/cert-public.pem" -outform DER -out "$scratch/cert-public.der"
openssl pkey -in "$key" -passin pass: -pubout -outform DER -out "$scratch/key-public.der"
cmp -s "$scratch/cert-public.der" "$scratch/key-public.der" || die 'Certificate and private key do not match'
# Uses the service's real certificate loader without binding any port.
./cover-site -check -identity "$identity" -cert "$cert" -key "$key"
exec 9>/run/lock/ecacc-cover-install.lock
flock -n 9 || die 'Another game installation is running'
listeners=$(ss -H -ltnp 'sport = :8443')
if [ -n "$listeners" ]; then
    pid=$(systemctl show -p MainPID --value ecacc-cover.service 2>/dev/null || true)
    [ -n "$pid" ] && [ "$pid" != 0 ] || die 'TCP 8443 is occupied by another service; no changes made'
    printf '%s\n' "$listeners" | awk -v token="pid=$pid," 'index($0,token)==0 {bad=1} END {exit bad}' || die 'TCP 8443 is occupied by another service; no changes made'
fi
if [ "$check" = yes ]; then
    echo "Checks passed for $identity. No service, panel, inbound or firewall changes made."
    exit 0
fi
mkdir -p /var/backups/ecacc-cover /opt/ecacc-cover /var/lib/ecacc-cover
backup=$(mktemp -d "/var/backups/ecacc-cover/$(date +%Y%m%d-%H%M%S).XXXXXX")
chmod 700 "$backup" /var/lib/ecacc-cover
was_active=no
was_enabled=no
if systemctl is-active --quiet ecacc-cover.service; then was_active=yes; fi
if systemctl is-enabled --quiet ecacc-cover.service; then was_enabled=yes; fi
for file in /opt/ecacc-cover/cover-site /var/lib/ecacc-cover/state.json /etc/systemd/system/ecacc-cover.service; do
    if [ -f "$file" ]; then cp -p "$file" "$backup/$(basename "$file")"; fi
done
rollback() {
    result=$?
    trap - EXIT
    if [ "$result" -ne 0 ] && [ "$started" = yes ]; then
        systemctl stop ecacc-cover.service || true
        for file in /opt/ecacc-cover/cover-site /var/lib/ecacc-cover/state.json /etc/systemd/system/ecacc-cover.service; do
            saved="$backup/$(basename "$file")"
            if [ -f "$saved" ]; then cp -p "$saved" "$file"; else rm -f "$file"; fi
        done
        systemctl daemon-reload || true
        if [ "$was_enabled" = yes ]; then systemctl enable ecacc-cover.service || true; else systemctl disable ecacc-cover.service 2>/dev/null || true; fi
        if [ "$was_active" = yes ]; then systemctl start ecacc-cover.service || true; fi
        echo "Installation failed; original game files/service restored. Backup: $backup" >&2
    fi
    cleanup
    exit "$result"
}
trap rollback EXIT
started=yes
install -m 755 cover-site /opt/ecacc-cover/cover-site.new
mv /opt/ecacc-cover/cover-site.new /opt/ecacc-cover/cover-site
if [ "$reroll" = yes ]; then /opt/ecacc-cover/cover-site -prepare -reroll; else /opt/ecacc-cover/cover-site -prepare; fi
cat > /etc/systemd/system/ecacc-cover.service <<EOF
[Unit]
Description=Local HTTPS game site
After=network.target
[Service]
ExecStart=/opt/ecacc-cover/cover-site -identity ${identity} -cert ${cert} -key ${key}
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
CapabilityBoundingSet=
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/var/lib/ecacc-cover
PrivateTmp=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable ecacc-cover.service
systemctl restart ecacc-cover.service
attempt=0
until curl --fail --silent --show-error --connect-timeout 2 --max-time 5 --resolve "${identity}:8443:127.0.0.1" "https://${identity}:8443/" -o "$scratch/page.html" 2>"$scratch/curl-error"; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 10 ]; then cat "$scratch/curl-error" >&2; die 'Local HTTPS verification failed'; fi
    sleep 1
done
systemctl is-active --quiet ecacc-cover.service
expected=$(sed -n 's/^[[:space:]]*"sha256":[[:space:]]*"\([a-f0-9]*\)".*/\1/p' /var/lib/ecacc-cover/state.json)
actual=$(sha256sum "$scratch/page.html" | awk '{print $1}')
[ "${#expected}" = 64 ] && [ "$actual" = "$expected" ] || die 'HTTPS response does not match the selected game'
echo "Installed and verified: $identity -> local HTTPS 127.0.0.1:8443"
echo "Backup: $backup"
echo 'Panel certificates are read directly, including after renewal.'
echo 'Next: ECAcc -> Servers -> Game fallback -> select REALITY inbound -> enable.'
echo 'Xray/3x-ui inbounds, clients, subscriptions, TCP/UDP 443 and firewall were not modified.'
cat /var/lib/ecacc-cover/state.json
trap cleanup EXIT
