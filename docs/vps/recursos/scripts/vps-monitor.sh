#!/bin/bash
# Health checks every 5 min. Alerts only on state change (problem / recovery).
# Optional webhook: set ALERT_WEBHOOK_URL in /etc/vps-monitor.env (Slack/Discord/Teams style JSON).
set -uo pipefail
[ -f /etc/vps-monitor.env ] && . /etc/vps-monitor.env
STATE=/var/lib/vps-monitor/last
mkdir -p "$(dirname "$STATE")"
HOST=$(hostname)
problems=()

# Disk
use=$(df --output=pcent / | tail -1 | tr -dc 0-9)
[ "$use" -ge "${DISK_LIMIT:-85}" ] && problems+=("disk ${use}%")

# Memory available < 10%
avail=$(awk '/MemAvailable/{a=$2}/MemTotal/{t=$2}END{printf "%d", a*100/t}' /proc/meminfo)
[ "$avail" -lt 10 ] && problems+=("memory available ${avail}%")

# Containers that should run (migrate is a one-shot job)
bad=$(docker ps -a --filter status=exited --filter status=restarting --filter status=dead --filter status=created --format "{{.Names}}" | grep -v migrate | tr "\n" " ")
[ -n "$bad" ] && problems+=("containers down: $bad")
for c in nginx anubis-corelog; do
  docker ps --format "{{.Names}}" | grep -qx "$c" || problems+=("container $c not running")
done

# Backup freshness (< 26 h)
lastb=$(find /var/backups/vps -name "config-*.tgz" -mmin -1560 2>/dev/null | head -1)
[ -z "$lastb" ] && problems+=("no backup in the last 26h")

# TLS certificate expiry
end=$(openssl x509 -enddate -noout -in /etc/letsencrypt/live/example.com/fullchain.pem 2>/dev/null | cut -d= -f2)
if [ -n "$end" ]; then
  days=$(( ( $(date -d "$end" +%s) - $(date +%s) ) / 86400 ))
  [ "$days" -lt 14 ] && problems+=("certificate expires in ${days} days")
else problems+=("certificate not readable"); fi

# Nginx answers locally on 443 (any HTTP code counts; 000 = no answer)
code=$(curl -sk -o /dev/null -m 8 -w "%{http_code}" --resolve app.example.com:443:127.0.0.1 https://app.example.com/ || true)
[ "$code" = "000" ] && problems+=("nginx not answering on 443")

# Failed systemd units
failed=$(systemctl --failed --plain --no-legend --no-pager | awk "{print \$1}" | tr "\n" " ")
[ -n "$failed" ] && problems+=("failed units: $failed")

now="${problems[*]:-}"
last=$(cat "$STATE" 2>/dev/null || true)
if [ "$now" != "$last" ]; then
  if [ -n "$now" ]; then msg="[$HOST] PROBLEM: $now"; prio=user.err
  else msg="[$HOST] RECOVERED: all checks OK"; prio=user.notice; fi
  logger -p "$prio" -t vps-monitor "$msg"
  if [ -n "${ALERT_WEBHOOK_URL:-}" ]; then
    curl -s -m 10 -X POST -H "Content-Type: application/json" \
      -d "{\"text\":\"$msg\",\"content\":\"$msg\"}" "$ALERT_WEBHOOK_URL" >/dev/null || true
  fi
  printf "%s" "$now" > "$STATE"
fi
