#!/usr/bin/env bash
# Moves nginx from "docker compose in /opt/nginx" to the host-default layout:
#   systemd unit + /usr/local/sbin/nginx wrapper + config in /etc/nginx and /var/www.
# Records a baseline of every site first, compares after the switch and rolls back
# automatically if anything differs. Run as root on the VPS.
set -uo pipefail

SRC="$(cd "$(dirname "$0")/.." && pwd)"          # the recursos/ directory
IMAGE="${NGINX_IMAGE:-nginx-clang:1.31.9-r2}"
APP=app.example.com
API=api.example.com
UA="Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537.36"
STAMP=$(date +%Y%m%d-%H%M%S)
BK=/root/nginx-pre-systemd-$STAMP
BASE=$BK/baseline.txt
AFTER=$BK/after.txt
log() { printf '[%s] %s\n' "$(date +%T)" "$*"; }

code() { curl -sk -o /dev/null -m 10 -w '%{http_code}' "$@" 2>/dev/null || true; }

probe() {
  printf 'http->https      %s\n' "$(code --resolve $APP:80:127.0.0.1 http://$APP/)"
  printf 'frontend         %s\n' "$(code -A "$UA" --resolve $APP:443:127.0.0.1 https://$APP/)"
  printf 'frontend robots  %s\n' "$(code -A "$UA" --resolve $APP:443:127.0.0.1 https://$APP/robots.txt)"
  printf 'api no token     %s\n' "$(code -A "$UA" --resolve $API:443:127.0.0.1 https://$API/api/v1/tickets)"
  printf 'api waf sqli     %s\n' "$(code -A "$UA" --resolve $API:443:127.0.0.1 "https://$API/?id=1%27%20OR%201=1--")"
  printf 'api waf xss      %s\n' "$(code -A "$UA" --resolve $API:443:127.0.0.1 "https://$API/?q=%3Cscript%3Ealert(1)%3C/script%3E")"
  printf 'api .git (444)   %s\n' "$(code -A "$UA" --resolve $API:443:127.0.0.1 https://$API/.git/config)"
  printf 'sni desconocido  %s\n' "$(code --resolve desconocido.example.com:443:127.0.0.1 https://desconocido.example.com/)"
  printf 'csp frontend     %s\n' "$(curl -sk -D - -o /dev/null -m 10 -A "$UA" --resolve $APP:443:127.0.0.1 https://$APP/robots.txt 2>/dev/null | grep -ci '^content-security-policy')"
  printf 'hsts api         %s\n' "$(curl -sk -D - -o /dev/null -m 10 -A "$UA" --resolve $API:443:127.0.0.1 https://$API/api/v1/tickets 2>/dev/null | grep -ci '^strict-transport-security')"
}

rollback() {
  log "ROLLBACK: $*"
  systemctl stop nginx 2>/dev/null
  systemctl disable nginx 2>/dev/null
  docker rm -f nginx >/dev/null 2>&1
  rm -f /etc/systemd/system/nginx.service /usr/local/sbin/nginx /usr/local/sbin/nginx-container
  systemctl daemon-reload
  (cd /opt/nginx && docker compose up -d 2>&1 | tail -1)
  sleep 3
  log "servicio anterior restaurado: $(docker ps --filter name=^nginx\$ --format '{{.Image}} {{.Status}}')"
  exit 1
}

mkdir -p "$BK"
log "1/8 linea base con el servicio actual ($(docker inspect -f '{{.Config.Image}}' nginx))"
probe | tee "$BASE"

log "2/8 respaldo de la configuracion actual"
cp -a /opt/nginx "$BK/opt-nginx"
cp -a /etc/logrotate.d/nginx-docker "$BK/" 2>/dev/null || true
cp -a /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh "$BK/" 2>/dev/null || true
cp -a /usr/local/sbin/backup-vps.sh "$BK/"

log "3/8 preparar la disposicion del sistema: /etc/nginx y /var/www"
if [ -e /etc/nginx ]; then log "/etc/nginx ya existe, abortando sin cambios"; exit 1; fi
install -d -m 755 /etc/nginx /var/www/acme /var/www/default
cp -a /opt/nginx/conf.d /opt/nginx/sites-enabled /opt/nginx/snippets /opt/nginx/geoip /etc/nginx/
cp -a /opt/nginx/nginx.conf /opt/nginx/mime.types /etc/nginx/
# workers sin privilegios (uid 33, el mismo de la imagen)
grep -q '^user ' /etc/nginx/nginx.conf || sed -i '1i user www-data;' /etc/nginx/nginx.conf
chown -R root:root /etc/nginx; chmod -R go-w /etc/nginx
chown -R www-data:www-data /var/log/modsecurity /var/log/modsec_audit.log
chmod 750 /var/log/modsecurity; chmod 640 /var/log/modsec_audit.log
log "   $(head -3 /etc/nginx/nginx.conf | tr '\n' '|')"

log "4/8 instalar envoltorio, script de arranque y unidad systemd"
install -m 755 "$SRC/scripts/nginx-container" /usr/local/sbin/nginx-container
install -m 755 "$SRC/scripts/nginx-wrapper" /usr/local/sbin/nginx
install -m 644 "$SRC/systemd/nginx.service" /etc/systemd/system/nginx.service
systemctl daemon-reload

log "5/8 pre-vuelo: validar la configuracion del sistema con la imagen nueva"
/usr/local/sbin/nginx-container test >/dev/null 2>&1 || { log "pre-vuelo fallido: limpiando"; rm -rf /etc/nginx /var/www/acme /var/www/default /usr/local/sbin/nginx /usr/local/sbin/nginx-container /etc/systemd/system/nginx.service; systemctl daemon-reload; exit 1; }
log "   OK"

log "6/8 conmutar (aqui empieza el corte)"
t0=$(date +%s.%N)
(cd /opt/nginx && docker compose down 2>&1 | tail -1)
systemctl enable --now nginx || rollback "systemctl start fallo"
for _ in $(seq 1 40); do ss -tln | grep -q ':443 ' && break; sleep 0.5; done
t1=$(date +%s.%N)
log "   sirviendo otra vez (corte: $(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.2f", b-a}') s)"
sleep 2

log "7/8 verificar contra la linea base"
probe | tee "$AFTER"
if ! diff -q "$BASE" "$AFTER" >/dev/null; then diff "$BASE" "$AFTER"; rollback "las respuestas difieren de la linea base"; fi
log "   identico a la linea base"
command -v nginx | grep -q '^/usr/local/sbin/nginx$' || rollback "el envoltorio no esta primero en el PATH"
nginx -t >/dev/null 2>&1 || rollback "nginx -t por el envoltorio"
systemctl reload nginx || rollback "systemctl reload"
sleep 1
[ "$(code -A "$UA" --resolve $API:443:127.0.0.1 https://$API/api/v1/tickets)" = 401 ] || rollback "caido tras reload"
log "   envoltorio, nginx -t y systemctl reload OK"
log "   workers: $(ps -eo user,args | grep 'nginx: worker' | grep -v grep | awk '{print $1}' | sort | uniq -c | tr '\n' ' ')"

log "8/8 cerrar: ajustar copias de seguridad, gancho de certbot y dejar la copia vieja inactiva"
sed -i 's#  /opt/nginx /opt/anubis /opt/corelog \\#  /etc/nginx /var/www /opt/anubis /opt/corelog /etc/systemd/system/nginx.service /usr/local/sbin/nginx-container /usr/local/sbin/nginx \\#' /usr/local/sbin/backup-vps.sh
bash -n /usr/local/sbin/backup-vps.sh && log "   backup-vps.sh actualizado"
printf '#!/bin/sh\nsystemctl reload nginx\n' > /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
chmod 755 /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
mv /opt/nginx /opt/nginx.pre-systemd
log "MIGRACION COMPLETA: $(nginx -v 2>&1) en $(docker inspect -f '{{.Config.Image}}' nginx)"
log "datos de retroceso en $BK  (y /opt/nginx.pre-systemd)"
