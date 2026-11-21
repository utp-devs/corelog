#!/bin/bash
# Daily local backup of VPS config + PostgreSQL dump. Retention: 14 days.
set -uo pipefail
DEST=/var/backups/vps
STAMP=$(date +%Y%m%d-%H%M%S)
KEEP_DAYS=14
umask 077
mkdir -p "$DEST"

# 1) Configuration and secrets (directory is root-only, mode 700)
tar -czf "$DEST/config-$STAMP.tgz" --warning=no-file-changed \
  /opt/nginx /opt/anubis /opt/corelog \
  /etc/letsencrypt /etc/modsecurity /usr/share/modsecurity-crs/crs-setup.conf \
  /etc/ssh/sshd_config.d /etc/fail2ban/jail.local /etc/sysctl.d/99-hardening.conf \
  /etc/docker /etc/ufw /etc/logrotate.d/nginx-docker /etc/cron.d \
  /usr/local/sbin/backup-vps.sh /usr/local/sbin/vps-monitor.sh /etc/vps-monitor.env \
  /root/.secrets 2>/dev/null
rc=$?
[ $rc -le 1 ] || { echo "backup: tar failed rc=$rc" >&2; exit 1; }

# 2) PostgreSQL (only if the corelog db container exists)
DBC=$(docker ps --filter "name=corelog-db" --format "{{.Names}}" | head -1)
if [ -n "$DBC" ]; then
  # shellcheck disable=SC1091
  set -a; . /opt/corelog/.env; set +a
  docker exec -e PGPASSWORD="$DB_PASSWORD" "$DBC" pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists \
    | gzip > "$DEST/corelog-db-$STAMP.sql.gz" || { echo "backup: pg_dump failed" >&2; exit 1; }
fi

# 3) Retention
find "$DEST" -type f \( -name "config-*.tgz" -o -name "corelog-db-*.sql.gz" \) -mtime +$KEEP_DAYS -delete
logger -t backup-vps "ok: $(ls -1 "$DEST" | wc -l) files, $(du -sh "$DEST" | cut -f1)"
