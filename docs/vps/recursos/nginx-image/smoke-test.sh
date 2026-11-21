#!/usr/bin/env bash
# Functional smoke test of an nginx-clang image. It starts a throw-away nginx on high
# loopback ports (18998 TLS, 18999 plain) with its own minimal configuration, so it can run
# next to the production service without touching it.
#   ./smoke-test.sh nginx-clang:1.31.9-r2 [/path/to/GeoLite2-Country.mmdb]
# Needs: docker, curl, openssl, gzip on the host. Exit code 0 = every check passed.
set -uo pipefail

IMAGE="${1:?usage: $0 IMAGE [GEOIP_DB]}"
GEOIP_DB="${2:-}"
NAME=nginx-smoke
T=$(mktemp -d)
PLAIN=127.0.0.1:18999
TLS=smoke.test:18998
pass=0; fail=0

ok()   { printf '  ok    %s\n' "$1"; pass=$((pass+1)); }
bad()  { printf '  FAIL  %s\n' "$1"; fail=$((fail+1)); }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1; rm -rf "$T"; }
trap cleanup EXIT
docker rm -f "$NAME" >/dev/null 2>&1

mkdir -p "$T/www" "$T/geoip"
printf 'hello from the smoke test, long enough to be compressed by gzip and brotli, hello hello hello\n' > "$T/www/index.txt"
printf 'this file is stored gzipped on disk and must be served inflated\n' | gzip -9 > "$T/www/stored.txt.gz"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj "/CN=smoke.test" -keyout "$T/key.pem" -out "$T/cert.pem" >/dev/null 2>&1
cat > "$T/mime.types" <<'EOF'
types { text/plain txt; }
EOF
cat > "$T/modsec.conf" <<'EOF'
SecRuleEngine On
SecRule ARGS "@contains attackme" "id:9001,phase:2,deny,status:403,msg:'smoke test rule'"
EOF

GEO_MOUNT=""; GEO_BLOCK=""; GEO_HDR=""
if [ -n "$GEOIP_DB" ] && [ -f "$GEOIP_DB" ]; then
  cp "$GEOIP_DB" "$T/geoip/GeoLite2-Country.mmdb"
  GEO_BLOCK='geoip2 /etc/nginx/geoip/GeoLite2-Country.mmdb { $smoke_country source=$remote_addr country iso_code; }'
  GEO_HDR='add_header X-Country "[$smoke_country]";'
fi

cat > "$T/nginx.conf" <<EOF
user www-data;
worker_processes 1;
pid /tmp/nginx-smoke.pid;
error_log stderr notice;
load_module modules/ngx_http_headers_more_filter_module.so;
load_module modules/ngx_http_brotli_filter_module.so;
load_module modules/ngx_http_brotli_static_module.so;
load_module modules/ngx_http_geoip2_module.so;
load_module modules/ngx_http_modsecurity_module.so;
events { worker_connections 64; }
http {
  include /etc/nginx/mime.types;
  access_log off;
  server_tokens off;
  more_set_headers "Server: smoke";
  gzip on; gzip_min_length 10; gzip_types text/plain;
  brotli on; brotli_min_length 10; brotli_types text/plain;
  $GEO_BLOCK
  modsecurity on;
  modsecurity_rules_file /etc/nginx/modsec.conf;
  server {
    listen 127.0.0.1:18999;
    root /srv/www;
    $GEO_HDR
    location / { try_files \$uri =404; }
    location /stored/ { alias /srv/www/; gzip_static always; gunzip on; }
  }
  server {
    listen 127.0.0.1:18998 ssl;
    http2 on;
    server_name smoke.test;
    ssl_certificate /etc/nginx/cert.pem;
    ssl_certificate_key /etc/nginx/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    root /srv/www;
    location / { try_files \$uri =404; }
  }
}
EOF

echo "== starting $IMAGE"
docker run -d --rm --name "$NAME" --network host --pull never \
  -v "$T/nginx.conf:/etc/nginx/nginx.conf:ro" -v "$T/mime.types:/etc/nginx/mime.types:ro" \
  -v "$T/modsec.conf:/etc/nginx/modsec.conf:ro" -v "$T/cert.pem:/etc/nginx/cert.pem:ro" \
  -v "$T/key.pem:/etc/nginx/key.pem:ro" -v "$T/geoip:/etc/nginx/geoip:ro" -v "$T/www:/srv/www:ro" \
  "$IMAGE" >/dev/null || { echo "could not start the container"; exit 2; }
for _ in $(seq 1 20); do curl -s -o /dev/null "http://$PLAIN/index.txt" && break; sleep 0.5; done
docker logs "$NAME" 2>&1 | grep -E "emerg|crit" | head -3

echo "== checks"
check "config loads, nginx serves plain HTTP"          '[ "$(curl -s -o /dev/null -w %{http_code} http://$PLAIN/index.txt)" = 200 ]'
check "headers-more: Server header rewritten"          'curl -sI http://$PLAIN/index.txt | grep -qi "^server: smoke"'
check "gzip response (zlib deflate)"                   'curl -sI -H "Accept-Encoding: gzip" http://$PLAIN/index.txt | grep -qi "^content-encoding: gzip"'
check "gzip body round-trips"                          '[ "$(curl -s --compressed -H "Accept-Encoding: gzip" http://$PLAIN/index.txt)" = "$(cat $T/www/index.txt)" ]'
check "brotli response"                                'curl -sI -H "Accept-Encoding: br" http://$PLAIN/index.txt | grep -qi "^content-encoding: br"'
check "gunzip module inflates a stored .gz (zlib inflate)" '[ "$(curl -s http://$PLAIN/stored/stored.txt)" = "this file is stored gzipped on disk and must be served inflated" ]'
check "ModSecurity blocks a rule match (403)"          '[ "$(curl -s -o /dev/null -w %{http_code} "http://$PLAIN/index.txt?q=attackme")" = 403 ]'
check "ModSecurity lets a normal request through"      '[ "$(curl -s -o /dev/null -w %{http_code} "http://$PLAIN/index.txt?q=normal")" = 200 ]'
check "TLS 1.3 + HTTP/2 (OpenSSL)"                     'o=$(curl -sk -v --http2 --tlsv1.3 --resolve smoke.test:18998:127.0.0.1 -o /dev/null -w "HTTPV=%{http_version}
" https://$TLS/index.txt 2>&1); echo "$o" | grep -q "SSL connection using TLSv1.3" && echo "$o" | grep -q "HTTPV=2"'
check "TLS 1.2 still negotiable"                       'curl -sk --tlsv1.2 --tls-max 1.2 --resolve smoke.test:18998:127.0.0.1 -o /dev/null -w "%{http_code}" https://$TLS/index.txt | grep -q 200'
check "SSLv3 / TLS 1.0 refused"                        '! curl -sk --tlsv1.0 --tls-max 1.0 --resolve smoke.test:18998:127.0.0.1 -o /dev/null https://$TLS/index.txt 2>/dev/null'
check "workers run as www-data (unprivileged)"         'ps -eo user,args | grep "nginx: worker" | grep -v grep | grep -q "^www-data"'
if [ -n "$GEOIP_DB" ]; then
  check "GeoIP2 database loaded (header present)"      'curl -sI http://$PLAIN/index.txt | grep -qi "^x-country:"'
fi
check "no alerts or crashes in the log"                '! docker logs "$NAME" 2>&1 | grep -qE "\[(emerg|alert|crit)\]"'
check "image has no shell"                             '! docker exec "$NAME" /bin/sh -c true >/dev/null 2>&1'

docker exec "$NAME" /usr/sbin/nginx -s quit >/dev/null 2>&1
echo "== $pass passed, $fail failed"
[ "$fail" -eq 0 ]
