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
| `nginx-image/Dockerfile` | Equipo de build: `./build.sh` (etiqueta `nginx-clang:1.31.9-r2`). Es siempre la revisión vigente | [06](../06-imagen-nginx-clang.md) |
| `nginx-image/releases/Dockerfile.<tag>` | Archivo de **cada** revisión publicada (`1.31.6-r1`, `1.31.9-r1`, `1.31.9-r2`), sin modificar | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx-image/CHANGELOG.md` | Qué cambió en cada revisión, comparada con la anterior | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx-image/build.sh` | Construye, etiqueta, verifica, prueba y archiva | [06](../06-imagen-nginx-clang.md), [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx-image/smoke-test.sh` | Prueba funcional de 15 comprobaciones sobre un nginx de prueba en puertos altos | [06](../06-imagen-nginx-clang.md) |
| `nginx-image/check-updates.sh` | Compara las versiones fijadas con las últimas publicadas | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx-image/check-docs.sh` | Falla si la documentación no coincide con el Dockerfile | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx-image/repro-clang.sh` | Reproduce la limitación del toolchain (Clang sin `libgcc-dev`) y su solución | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `nginx/docker-compose.yml` | `/opt/nginx/docker-compose.yml` (modo A, alternativa simple; el servidor usa el modo B) | [07](../07-nginx-configuracion.md) |
| `nginx/nginx.conf` | `/etc/nginx/nginx.conf` (con `user www-data;`) | [07](../07-nginx-configuracion.md) |
| `nginx/conf.d/security-maps.conf` | `/etc/nginx/conf.d/` | [07](../07-nginx-configuracion.md) |
| `nginx/conf.d/anubis-map.conf` | `/etc/nginx/conf.d/` | [09](../09-anubis.md) |
| `nginx/snippets/*.conf` | `/etc/nginx/snippets/` | [07](../07-nginx-configuracion.md), [09](../09-anubis.md) |
| `nginx/sites-enabled/00-default.conf` | `/etc/nginx/sites-enabled/` | [07](../07-nginx-configuracion.md) |
| `nginx/sites-enabled/app.conf` | `/etc/nginx/sites-enabled/<app>.conf` | [07](../07-nginx-configuracion.md) |
| `modsecurity/modsecurity.conf` | `/etc/modsecurity/modsecurity.conf` | [08](../08-waf-modsecurity-crs.md) |
| `anubis/docker-compose.yml` | `/opt/anubis/docker-compose.yml` | [09](../09-anubis.md) |
| `anubis/politica.yaml` | `/opt/anubis/politica.yaml` (modo 644) | [09](../09-anubis.md) |
| `anubis/env.example` | `/opt/anubis/.env` (modo 600, con tu llave) | [09](../09-anubis.md) |
| `scripts/nginx-container`, `scripts/nginx-wrapper`, `systemd/nginx.service` | `/usr/local/sbin/nginx-container`, `/usr/local/sbin/nginx`, `/etc/systemd/system/nginx.service` (modo B, aplicado) | [10](../10-nginx-como-servicio-del-host.md) |
| `scripts/upgrade-nginx-image.sh` | Cambia un servidor que ya usa el modo B a una imagen nueva: línea base de todos los `server_name`, conmutación, comparación y retroceso automático (`NEW=nginx-clang:<tag> ./upgrade-nginx-image.sh`) | [16](../16-mantenimiento-y-versionado-de-la-imagen.md) |
| `scripts/migrate-nginx-to-host.sh` | Migra de Compose al modo B con línea base y retroceso automático (editar `APP` y `API`) | [10](../10-nginx-como-servicio-del-host.md) |
| `logrotate/nginx-host-variant` | `/etc/logrotate.d/nginx` (variante del servidor de referencia; el servidor nuevo usa `nginx-docker`) | [10](../10-nginx-como-servicio-del-host.md) |
| `fail2ban/jail.local` | `/etc/fail2ban/jail.local` | [11](../11-fail2ban.md) |
| `fail2ban/filter.d/nginx-attack.conf`, `nginx-404.conf`, `nginx-geoblock.conf`, `nginx-modsec.conf` | `/etc/fail2ban/filter.d/` | [11](../11-fail2ban.md) |
| `fail2ban/filter.d/*-reference.conf` | Referencia del servidor anterior (no activos) | [11](../11-fail2ban.md) |
| `certbot/cloudflare.ini.example` | `/root/.secrets/cloudflare.ini` (modo 600, con tu token) | [12](../12-dns-y-certificados-tls.md) |
| `certbot/reload-nginx.sh` | `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh` (modo 755; `systemctl reload nginx`) | [12](../12-dns-y-certificados-tls.md) |
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
