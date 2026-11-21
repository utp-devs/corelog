# Recursos: archivos de configuración

Archivos listos para copiar al servidor. Provienen del servidor real (con dominios, IPs y nombres de usuario reemplazados por valores de ejemplo) y de los proyectos de referencia. **No contienen secretos**: los valores sensibles están como `<…>` o en archivos `*.example`.

Antes de usarlos: reemplazar `example.com` / `app.example.com` / `api.example.com` por el dominio real, `<IP_DEL_VPS>` y `<TU_IP_PUBLICA>`, y revisar los nombres de contenedores (`corelog-db`, `anubis-corelog`).

← [Índice de la guía](../README.md)

| Recurso | Destino en el servidor | Documento |
|---|---|---|
| `scripts/ufw-setup.sh` | Ejecutar una vez | [03](../03-firewall-ufw.md) |
| `ssh/10-hardening.conf` | `/etc/ssh/sshd_config.d/10-hardening.conf` | [02](../02-usuarios-permisos-y-ssh.md) |
| `ssh/crypto-optional.conf` | `/etc/ssh/sshd_config.d/20-crypto.conf` (opcional) | [02](../02-usuarios-permisos-y-ssh.md) |
| `sysctl/99-hardening.conf` | `/etc/sysctl.d/99-hardening.conf` | [04](../04-endurecimiento-del-sistema.md) |
| `docker/daemon.json` | `/etc/docker/daemon.json` | [05](../05-docker.md) |
| `docker/daemon-hardened.json` | `/etc/docker/daemon.json` (variante con `userns-remap`) | [05](../05-docker.md) |
| `nginx-image/Dockerfile` | Equipo de build (`docker build -t nginx-clang:1.31.6 .`) | [06](../06-imagen-nginx-clang.md) |
| `nginx/docker-compose.yml` | `/opt/nginx/docker-compose.yml` | [07](../07-nginx-configuracion.md) |
| `nginx/nginx.conf` | `/opt/nginx/nginx.conf` | [07](../07-nginx-configuracion.md) |
| `nginx/conf.d/security-maps.conf` | `/opt/nginx/conf.d/` | [07](../07-nginx-configuracion.md) |
| `nginx/conf.d/anubis-map.conf` | `/opt/nginx/conf.d/` | [09](../09-anubis.md) |
| `nginx/snippets/*.conf` | `/opt/nginx/snippets/` | [07](../07-nginx-configuracion.md), [09](../09-anubis.md) |
| `nginx/sites-enabled/00-default.conf` | `/opt/nginx/sites-enabled/` | [07](../07-nginx-configuracion.md) |
| `nginx/sites-enabled/app.conf` | `/opt/nginx/sites-enabled/<app>.conf` | [07](../07-nginx-configuracion.md) |
| `modsecurity/modsecurity.conf` | `/etc/modsecurity/modsecurity.conf` | [08](../08-waf-modsecurity-crs.md) |
| `anubis/docker-compose.yml` | `/opt/anubis/docker-compose.yml` | [09](../09-anubis.md) |
| `anubis/politica.yaml` | `/opt/anubis/politica.yaml` (modo 644) | [09](../09-anubis.md) |
| `anubis/env.example` | `/opt/anubis/.env` (modo 600, con tu llave) | [09](../09-anubis.md) |
| `scripts/nginx-container`, `scripts/nginx-wrapper`, `systemd/nginx.service`, `logrotate/nginx-host-variant` | Ver la tabla del documento 10 (modo B) | [10](../10-nginx-como-servicio-del-host.md) |
| `fail2ban/jail.local` | `/etc/fail2ban/jail.local` | [11](../11-fail2ban.md) |
| `fail2ban/filter.d/nginx-attack.conf`, `nginx-404.conf`, `nginx-geoblock.conf`, `nginx-modsec.conf` | `/etc/fail2ban/filter.d/` | [11](../11-fail2ban.md) |
| `fail2ban/filter.d/*-reference.conf` | Referencia del servidor anterior (no activos) | [11](../11-fail2ban.md) |
| `certbot/cloudflare.ini.example` | `/root/.secrets/cloudflare.ini` (modo 600, con tu token) | [12](../12-dns-y-certificados-tls.md) |
| `certbot/reload-nginx.sh` | `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh` (modo 755) | [12](../12-dns-y-certificados-tls.md) |
| `logrotate/nginx-docker` | `/etc/logrotate.d/nginx-docker` | [07](../07-nginx-configuracion.md) |
| `scripts/backup-vps.sh` | `/usr/local/sbin/backup-vps.sh` (modo 750) | [14](../14-backups-monitoreo-y-alertas.md) |
| `scripts/vps-monitor.sh` | `/usr/local/sbin/vps-monitor.sh` (modo 750) | [14](../14-backups-monitoreo-y-alertas.md) |
| `scripts/vps-monitor.env.example` | `/etc/vps-monitor.env` (modo 600) | [14](../14-backups-monitoreo-y-alertas.md) |
| `cron/vps-ops` | `/etc/cron.d/vps-ops` | [14](../14-backups-monitoreo-y-alertas.md) |

## Recursos que **no** se incluyen (y por qué)

| Recurso | Motivo |
|---|---|
| Base `GeoLite2-Country.mmdb` | Licencia de MaxMind: cada operador descarga la suya con su cuenta |
| Reglas OWASP CRS | Se descargan de la release oficial (documento 08) |
| `.env` de la aplicación, llaves SSH, token de Cloudflare | Secretos: se generan en cada servidor |
| `mime.types` | Archivo estándar de Nginx (`/etc/nginx/mime.types` de cualquier instalación) |
