#!/usr/bin/env bash
# Switches the host nginx service (nginx-container + systemd) to a new nginx-clang image.
# Records a baseline of every server_name before, compares after, rolls back on any difference.
#   NEW=nginx-clang:1.31.9-r2 ./upgrade-nginx-image.sh
set -uo pipefail
NEW="${NEW:?set NEW=nginx-clang:<tag>}"
SCRIPT=/usr/local/sbin/nginx-container
UA="Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537.36"
STAMP=$(date +%Y%m%d-%H%M%S)
BK=/root/nginx-image-upgrade-$STAMP
mkdir -p "$BK"
log() { printf '[%s] %s\n' "$(date +%T)" "$*"; }

OLD=$(grep -m1 -oE 'nginx-clang:[0-9.]+(-r[0-9]+)?' "$SCRIPT")
[ -n "$OLD" ] || { log "cannot read the current image from $SCRIPT"; exit 1; }
[ "$OLD" != "$NEW" ] || { log "already on $NEW"; exit 0; }
docker image inspect "$NEW" >/dev/null 2>&1 || { log "image $NEW is not loaded"; exit 1; }

# hosts from the effective configuration
nginx -T 2>/dev/null | grep -E '^\s*server_name ' | grep -v '^\s*#' | sed 's/^\s*server_name //; s/;.*//' \
  | tr ' ' '\n' | grep -E '^[A-Za-z0-9.-]+\.[A-Za-z]{2,}$' | sort -u > "$BK/hosts.txt"
log "hosts to probe: $(wc -l < "$BK/hosts.txt")"

sample() {  # one line per host: http code, https code, https size
  while read -r h; do
    http=$(curl -sk -o /dev/null -m 10 -w '%{http_code}' -A "$UA" --resolve "$h:80:127.0.0.1" "http://$h/" 2>/dev/null || true)
    read -r https size < <(curl -sk -o /dev/null -m 10 -w '%{http_code} %{size_download}' -A "$UA" --resolve "$h:443:127.0.0.1" "https://$h/" 2>/dev/null || echo "000 0")
    printf '%s %s %s %s\n' "$h" "$http" "$https" "$size"
  done < "$BK/hosts.txt"
}
extra() {   # things that are not plain GETs
  printf 'listening %s\n' "$(ss -tlnpH | grep nginx | awk '{print $4}' | sed 's/.*://' | sort -un | tr '\n' ',')"
  printf 'sni-unknown %s\n' "$(curl -sk -o /dev/null -m 8 -w '%{http_code}' --resolve "zz-unknown.invalid:443:127.0.0.1" https://zz-unknown.invalid/ 2>/dev/null || true)"
  printf 'attack-444 %s\n' "$(curl -sk -o /dev/null -m 8 -w '%{http_code}' -A "$UA" --resolve "$(head -1 "$BK/hosts.txt"):443:127.0.0.1" "https://$(head -1 "$BK/hosts.txt")/.env" 2>/dev/null || true)"
}

log "1/6 baseline with $OLD (two samples to tell stable responses from dynamic ones)"
sample > "$BK/base1.txt"; sleep 2; sample > "$BK/base2.txt"; extra > "$BK/base-extra.txt"
# stable = same code AND size in both samples; sizes are only compared for those
paste -d' ' "$BK/base1.txt" "$BK/base2.txt" | awk '{stable=($4==$8)?1:0; printf "%s %s %s %s %s\n",$1,$2,$3,$4,stable}' > "$BK/baseline.txt"
unstable=$(awk '$5==0' "$BK/baseline.txt" | wc -l)
log "   $(wc -l < "$BK/baseline.txt") hosts recorded, $unstable with dynamic size (only their status codes are compared)"
awk '{print $1, "http="$2, "https="$3}' "$BK/baseline.txt" | column -t | head -30

log "2/6 backup of the start script"
cp -a "$SCRIPT" "$BK/nginx-container.orig"

rollback() {
  log "ROLLBACK: $*"
  cp -a "$BK/nginx-container.orig" "$SCRIPT"
  systemctl restart nginx
  for _ in $(seq 1 40); do ss -tln | grep -q ':443 ' && break; sleep 0.5; done
  log "restored: $(docker inspect -f '{{.Config.Image}}' nginx 2>/dev/null) ($(systemctl is-active nginx))"
  exit 1
}

log "3/6 switch $OLD -> $NEW (downtime starts)"
sed -i "s#$OLD#$NEW#" "$SCRIPT"
t0=$(date +%s.%N)
systemctl restart nginx || rollback "systemctl restart failed"
for _ in $(seq 1 60); do ss -tln | grep -q ':443 ' && break; sleep 0.5; done
t1=$(date +%s.%N)
log "   serving again, downtime $(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.1f", b-a}') s; image: $(docker inspect -f '{{.Config.Image}}' nginx)"
sleep 3

log "4/6 compare with the baseline"
sample >/dev/null 2>&1 || true      # warm-up pass: first requests after a restart can be slower
sample > "$BK/after.txt"; extra > "$BK/after-extra.txt"
fail=0
while read -r h http https size stable; do
  line=$(grep "^$h " "$BK/after.txt")
  a_http=$(echo "$line" | awk '{print $2}'); a_https=$(echo "$line" | awk '{print $3}'); a_size=$(echo "$line" | awk '{print $4}')
  if [ "$http" != "$a_http" ] || [ "$https" != "$a_https" ]; then log "   DIFF  $h  http $http->$a_http  https $https->$a_https"; fail=1
  elif [ "$stable" = 1 ] && [ "$size" != "$a_size" ]; then log "   DIFF  $h  size $size->$a_size (stable page)"; fail=1; fi
done < "$BK/baseline.txt"
diff -q "$BK/base-extra.txt" "$BK/after-extra.txt" >/dev/null || { log "   DIFF  extra checks:"; diff "$BK/base-extra.txt" "$BK/after-extra.txt"; fail=1; }
[ "$fail" = 0 ] || rollback "responses differ from the baseline"
log "   identical to the baseline ($(wc -l < "$BK/baseline.txt") hosts + listening ports, unknown SNI, attack path)"

log "5/6 host integration"
command -v nginx | grep -q '^/usr/local/sbin/nginx$' || rollback "nginx wrapper is not first in PATH"
nginx -V 2>&1 | head -1 | sed 's/^/   /'
nginx -t >/dev/null 2>&1 || rollback "nginx -t via the wrapper"
systemctl reload nginx || rollback "systemctl reload"
sleep 1
log "   workers: $(ps -eo user,args | grep 'nginx: worker' | grep -v grep | awk '{print $1}' | sort | uniq -c | tr '\n' ' ')"
log "   service: $(systemctl is-active nginx), enabled: $(systemctl is-enabled nginx)"

log "6/6 done. Old image $OLD stays loaded for rollback; backup in $BK"
log "   rollback: sed -i 's#$NEW#$OLD#' $SCRIPT && systemctl restart nginx"
