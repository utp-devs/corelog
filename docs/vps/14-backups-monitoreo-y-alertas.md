# 14 · Backups, monitoreo y alertas

← [13 Usuario de despliegue y CI/CD](13-usuario-de-despliegue-y-cicd.md) · [Índice](README.md) · Siguiente: [15 Verificación y auditoría](15-verificacion-y-auditoria.md)

**Objetivo:** poder recuperar el servidor y la base de datos, y enterarse de los problemas **antes** de que los reporte un usuario.

## 1. Backups

Script: [`recursos/scripts/backup-vps.sh`](recursos/scripts/backup-vps.sh) → `/usr/local/sbin/backup-vps.sh` (modo 750).

### Qué respalda

| Contenido | Archivo | Para qué |
|---|---|---|
| `/opt/nginx`, `/opt/anubis`, `/opt/<app>` | `config-AAAAMMDD-HHMMSS.tgz` | Configuración, `.env`, política de Anubis, compose |
| `/etc/letsencrypt`, `/etc/modsecurity` | ídem | Certificados y llaves TLS, WAF |
| `/etc/ssh/sshd_config.d`, `/etc/fail2ban/jail.local`, `/etc/sysctl.d`, `/etc/docker`, `/etc/ufw`, `/etc/cron.d`, logrotate | ídem | Configuración del sistema |
| `/usr/local/sbin/{backup-vps,vps-monitor}.sh`, `/etc/vps-monitor.env`, `/root/.secrets` | ídem | Los propios scripts y secretos |
| Base de datos PostgreSQL | `corelog-db-AAAAMMDD-HHMMSS.sql.gz` | `pg_dump --clean --if-exists`, solo si existe el contenedor `*-db` |

Destino `/var/backups/vps/` con modo **700** y archivos **600**: contienen secretos y llaves privadas.

### Cómo se ejecuta

[`recursos/cron/vps-ops`](recursos/cron/vps-ops) → `/etc/cron.d/vps-ops`:

```
0 3 * * * root /usr/local/sbin/backup-vps.sh
*/5 * * * * root /usr/local/sbin/vps-monitor.sh
```

Diario a las 03:00 (hora local del servidor, ver [01](01-requisitos-y-primeros-pasos.md)) y retención de **14 días** (`find … -mtime +14 -delete`).

```bash
install -m 750 backup-vps.sh /usr/local/sbin/backup-vps.sh
install -d -m 700 /var/backups/vps
/usr/local/sbin/backup-vps.sh && ls -lh /var/backups/vps
tar -tzf /var/backups/vps/config-*.tgz | grep -E 'opt/<app>/.env|letsencrypt/live|opt/nginx/nginx.conf'
zcat /var/backups/vps/corelog-db-*.sql.gz | grep -c 'CREATE TABLE'
```

> Adaptar el script a la aplicación: el nombre del contenedor de base de datos (`corelog-db`), la ruta del `.env` y las variables `DB_USER`/`DB_NAME`/`DB_PASSWORD`.

### Restaurar

**Base de datos:**

```bash
docker exec -i <contenedor-db> sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' \
  < <(zcat /var/backups/vps/corelog-db-AAAAMMDD-HHMMSS.sql.gz)
```

**Configuración** (en un servidor nuevo o tras un desastre): descomprimir sobre `/` solo lo necesario, p. ej. `tar -xzf config-….tgz -C / opt/nginx etc/letsencrypt`, revisar permisos (secciones 6 de [04](04-endurecimiento-del-sistema.md)) y `docker compose up -d`.

> **Un backup que nunca se restauró no es un backup.** Probar la restauración de la base de datos en un contenedor aparte (no en producción) tras el primer despliegue y después cada trimestre.

### Fuera del servidor (obligatorio en producción)

Estos backups están **en el mismo disco**: no protegen de la pérdida del servidor, del disco o de un borrado malicioso. Enviarlos a otro lugar, **cifrados** (contienen llaves privadas y secretos):

| Herramienta | Idea |
|---|---|
| `restic` o `borg` | Copias cifradas, deduplicadas, con retención, hacia un bucket S3/R2 o un servidor SFTP |
| `rclone` | Sincronizar `/var/backups/vps` con almacenamiento en la nube (cifrar antes con `age` o `rclone crypt`) |
| Almacenamiento del proveedor | Snapshots del VPS (complementarios, no sustitutos de los backups lógicos) |

## 2. Monitoreo

Script: [`recursos/scripts/vps-monitor.sh`](recursos/scripts/vps-monitor.sh) → `/usr/local/sbin/vps-monitor.sh`. Corre cada 5 minutos desde cron.

| Comprobación | Umbral / condición |
|---|---|
| Disco `/` | ≥ 85 % (variable `DISK_LIMIT`) |
| Memoria disponible | < 10 % |
| Contenedores en estado `exited`, `restarting`, `dead` o `created` | Cualquiera, **excepto** `migrate` (job de una sola vez) |
| Contenedores esperados (`nginx`, `anubis-corelog`) | Deben estar corriendo |
| Caducidad del certificado | < 14 días |
| Nginx responde en el 443 localmente | Respuesta HTTP distinta de `000` |
| Backup reciente | Existe un `config-*.tgz` de las últimas 26 h |
| Unidades `systemd` fallidas | Ninguna |

> Editar en el script el dominio de la comprobación del 443 (`app.example.com`) y los nombres de contenedores esperados.

### Alerta por **cambio de estado**

El script guarda el último estado en `/var/lib/vps-monitor/last` y solo avisa cuando **cambia**:

- aparece un problema → `PROBLEM: <detalle>` (prioridad `user.err`);
- se resuelve todo → `RECOVERED: all checks OK`.

No repite el mismo aviso cada cinco minutos (evita la fatiga de alertas).

### Destinos de las alertas

1. **Siempre**: al journal con la etiqueta `vps-monitor`:
   ```bash
   journalctl -t vps-monitor --no-pager -n 20
   ```
2. **Opcional**: un *webhook* compatible con Slack, Discord o Teams. En `/etc/vps-monitor.env` (modo 600, plantilla [`vps-monitor.env.example`](recursos/scripts/vps-monitor.env.example)):
   ```
   ALERT_WEBHOOK_URL=https://hooks.example.com/…
   DISK_LIMIT=85
   ```
   El script envía `{"text":"…","content":"…"}` por POST; cubre Slack/Teams (`text`) y Discord (`content`).

### Probar las alertas

```bash
docker stop anubis-corelog
/usr/local/sbin/vps-monitor.sh
journalctl -t vps-monitor -n 2 --no-pager         # PROBLEM: containers down: anubis-corelog …
docker start anubis-corelog && sleep 3
/usr/local/sbin/vps-monitor.sh
journalctl -t vps-monitor -n 1 --no-pager         # RECOVERED: all checks OK
```

Verificado en el servidor documentado, incluida la detección de una unidad `systemd` fallida (cloud-init del proveedor) que obligó a corregir el análisis de `systemctl --failed`: usar siempre `--plain --no-legend --no-pager`, o el símbolo `●` se confunde con el nombre de la unidad.

## 3. Lo que este monitoreo **no** cubre

| Falta | Complemento recomendado |
|---|---|
| Servidor completamente caído (el script corre *en* el servidor) | Monitoreo externo de disponibilidad (UptimeRobot, Better Stack, Uptime Kuma) sobre `https://app.example.com` |
| Latencia y errores de la aplicación | Métricas (Prometheus + Grafana) o un APM |
| Intentos de intrusión | Revisar los jails de fail2ban ([11](11-fail2ban.md)) y `attacks.log`; alertar en cada baneo permanente |
| Cambios en archivos del sistema | `aide` o `auditd` ([04](04-endurecimiento-del-sistema.md)) |

## 4. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `ls -lh /var/backups/vps` | Un `config-*.tgz` y un `*-db-*.sql.gz` recientes, modo 600 |
| `stat -c '%a %U' /var/backups/vps` | `700 root` |
| `cat /etc/cron.d/vps-ops` | Backup y monitoreo programados |
| `cat /var/lib/vps-monitor/last` | Vacío (sin problemas) |
| Prueba de alertas (arriba) | `PROBLEM` y luego `RECOVERED` |
| Restauración de prueba de la base de datos | Las tablas y filas esperadas |

Continuar con [15 · Verificación, auditoría y lecciones aprendidas](15-verificacion-y-auditoria.md).
