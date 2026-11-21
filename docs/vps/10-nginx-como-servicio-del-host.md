# 10 · Nginx como servicio por defecto del host

← [09 Anubis](09-anubis.md) · [Índice](README.md) · Siguiente: [11 fail2ban](11-fail2ban.md)

**Objetivo:** que, para el sistema operativo y para quien administra, el Nginx del contenedor **sea** el Nginx del servidor: `systemctl start|stop|reload|status nginx` y `nginx -t`, `nginx -V`, `nginx -s reload` funcionan en el host igual que con un paquete instalado, y arranca solo con el sistema.

## 1. Dos modos

| | A · Compose (alternativa simple) | B · Servicio `systemd` + envoltorio (**aplicado**) |
|---|---|---|
| Cómo arranca | `restart: unless-stopped` en Docker | Unidad `nginx.service` de systemd |
| `systemctl status nginx` | No (se usa `docker compose ps`) | Sí |
| `nginx -t` en el host | No (`docker compose exec nginx nginx -t`) | Sí (envoltorio en `/usr/local/sbin/nginx`) |
| certbot, logrotate y scripts que llaman a `nginx` | Hay que adaptarlos | Funcionan sin cambios |
| Disposición de archivos | `/opt/nginx/…` | `/etc/nginx`, `/etc/letsencrypt`, `/var/www`, `/var/log/nginx` (rutas del sistema) |
| Verificado en | Servidor nuevo (Debian 13), hasta la migración | Servidor de referencia (Debian 12) y servidor nuevo (Debian 13), ambos en producción |

Ambos servidores usan hoy el **modo B**: es el que convierte al contenedor en "el Nginx por defecto del host". El modo A (un `docker-compose.yml`, documento [07](07-nginx-configuracion.md), sección 2) es el punto de partida más simple y sigue siendo válido si no se necesita integración con el sistema; el servidor nuevo empezó así y se migró con la sección 3.

## 2. Modo B: el contenedor como servicio del sistema

### 2.1 Piezas

| Archivo del repositorio | Destino en el servidor | Función |
|---|---|---|
| [`recursos/scripts/nginx-container`](recursos/scripts/nginx-container) | `/usr/local/sbin/nginx-container` | Lanza el contenedor (`run`) o valida la configuración con la imagen (`test`) |
| [`recursos/scripts/nginx-wrapper`](recursos/scripts/nginx-wrapper) | `/usr/local/sbin/nginx` | Reenvía cualquier llamada `nginx …` al contenedor |
| [`recursos/systemd/nginx.service`](recursos/systemd/nginx.service) | `/etc/systemd/system/nginx.service` | Unidad que reemplaza a la de la distribución |
| [`recursos/logrotate/nginx-host-variant`](recursos/logrotate/nginx-host-variant) | `/etc/logrotate.d/nginx` | Rotación con `create 0640 www-data adm` y reapertura vía `docker exec` |

### 2.2 `nginx-container`

```sh
#!/bin/sh
# run:  arranca el contenedor en red host con la configuración, certificados, reglas WAF y logs del host
# test: valida la configuración del host con la imagen, sin tocar el servicio
set -eu
IMAGE="${NGINX_IMAGE:-nginx-clang:1.31.9-r2}"
NAME=nginx

MOUNTS="
  -v /etc/nginx:/etc/nginx:ro
  -v /etc/letsencrypt:/etc/letsencrypt:ro
  -v /etc/modsecurity:/etc/modsecurity:ro
  -v /usr/share/modsecurity-crs:/usr/share/modsecurity-crs:ro
  -v /var/www:/var/www:ro
  -v /var/log/nginx:/var/log/nginx
  -v /var/log/modsecurity:/var/log/modsecurity
  -v /var/log/modsec_audit.log:/var/log/modsec_audit.log
  -v /etc/localtime:/etc/localtime:ro
  -v /etc/timezone:/etc/timezone:ro
"

case "${1:-}" in
  run)  docker rm -f "$NAME" >/dev/null 2>&1 || true
        exec docker run --rm --init --name "$NAME" --network host --userns host --pull never $MOUNTS "$IMAGE" ;;
  test) exec docker run --rm --network host --userns host --pull never $MOUNTS "$IMAGE" nginx -t ;;
  *)    echo "usage: $0 run|test" >&2; exit 2 ;;
esac
```

Detalles:

- `--rm --init`: el contenedor se elimina al parar y un proceso `init` recoge zombis.
- `--pull never`: arrancar nunca depende de la red ni de un registro; la imagen ya debe estar cargada.
- `--userns host`: si el demonio usa `userns-remap` ([05](05-docker.md)), es obligatorio para `--network host` y para que el proceso maestro lea las llaves TLS que solo `root` puede leer (los *workers* siguen bajando de privilegios). Si no hay remapeo, no tiene efecto.
- `NGINX_IMAGE` permite probar una etiqueta nueva sin editar el script (`NGINX_IMAGE=nginx-clang:1.31.7 nginx-container test`).

### 2.3 Envoltorio `nginx`

```sh
#!/bin/sh
exec docker exec nginx nginx "$@"
```

`/usr/local/sbin` va **antes** que `/usr/sbin` en el `PATH`, así que `nginx -t`, `nginx -T`, `nginx -V` y `nginx -s reload|reopen` del host llegan al contenedor. Es lo que mantiene funcionando a `certbot` (plugin de Nginx, ganchos de renovación), a `logrotate` y a cualquier script existente sin modificarlos.

### 2.4 Unidad systemd

```ini
[Unit]
Description=nginx (nginx-clang container, host network)
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
ExecStartPre=/usr/local/sbin/nginx-container test
ExecStart=/usr/local/sbin/nginx-container run
ExecReload=/usr/bin/docker exec nginx nginx -t
ExecReload=/usr/bin/docker exec nginx nginx -s reload
ExecStop=/usr/bin/docker stop -t 30 nginx
Restart=always
RestartSec=3
TimeoutStartSec=120
TimeoutStopSec=40

[Install]
WantedBy=multi-user.target
```

| Línea | Efecto |
|---|---|
| `ExecStartPre=… test` | **No arranca con una configuración rota**: valida antes de arrancar |
| `ExecReload=` doble | Recargar valida primero y solo recarga si `nginx -t` pasa |
| `ExecStop=docker stop -t 30` | Parada ordenada: `SIGQUIT` (`STOPSIGNAL` de la imagen) deja terminar las conexiones en curso hasta 30 s |
| `Restart=always`, `RestartSec=3` | Se levanta solo tras cualquier caída |
| `After=docker.service` | Orden de arranque correcto |

> **Consecuencia de `Requires=docker.service`.** Según la documentación de systemd, parar o reiniciar `docker.service` también para o reinicia a `nginx.service`, aunque el demonio tenga `live-restore`. Actualizar Docker provocaría un corte breve de Nginx. Si eso no es aceptable, usar `Wants=docker.service` (arranque ordenado sin acoplar las paradas). Es una elección de diseño que **no se probó** en el servidor de referencia.

### 2.5 Instalación

```bash
install -m 755 nginx-container /usr/local/sbin/nginx-container
install -m 755 nginx-wrapper   /usr/local/sbin/nginx
install -m 644 nginx.service   /etc/systemd/system/nginx.service
install -m 644 logrotate-nginx /etc/logrotate.d/nginx
systemctl daemon-reload
systemctl enable --now nginx
```

Antes: la configuración debe estar en las rutas del host (`/etc/nginx`, `/etc/modsecurity`, …) en lugar de `/opt/nginx`, o editar la lista `MOUNTS` de `nginx-container` para apuntar a `/opt/nginx`. La sección 3 automatiza este paso.

### 2.6 Desplazar al Nginx de la distribución

Dos servidores no pueden escuchar el mismo puerto. Si existe un `nginx` del paquete de Debian:

```bash
systemctl disable --now nginx            # antes de instalar la unidad nueva
apt-get purge -y 'nginx*'                # o apt-mark hold nginx para que no vuelva
```

La unidad en `/etc/systemd/system/nginx.service` tiene prioridad sobre la de `/lib/systemd/system/`, por lo que **sustituye** a la de la distribución sin borrarla.

## 3. Migrar a este modo (con retroceso automático)

Script: [`recursos/scripts/migrate-nginx-to-host.sh`](recursos/scripts/migrate-nginx-to-host.sh). Se ejecuta como `root` **desde un archivo** (no por `stdin`; ver [15](15-verificacion-y-auditoria.md)) y antes hay que editar las variables `APP` y `API` con los dominios reales. Precondiciones: la imagen cargada (`docker images nginx-clang`), la configuración actual en `/opt/nginx` y que **no exista** `/etc/nginx`.

Qué hace, en orden:

1. **Línea base.** Mide el servicio actual: HTTP→HTTPS, frontend, `robots.txt`, API sin token, SQLi y XSS (WAF), ruta privada (444), SNI desconocido, CSP del frontend y HSTS de la API.
2. **Respaldo** de `/opt/nginx`, del logrotate, del gancho de certbot y del script de backup, en `/root/nginx-pre-systemd-<fecha>/`.
3. **Disposición del sistema.** Copia `conf.d`, `sites-enabled`, `snippets`, `geoip`, `nginx.conf` y `mime.types` a `/etc/nginx`; crea `/var/www/{acme,default}`; añade `user www-data;` al `nginx.conf`; da a `www-data` la propiedad de `/var/log/modsecurity` y `/var/log/modsec_audit.log` (los *workers* ya no son `nobody`).
4. **Instala** `nginx-container`, el envoltorio y la unidad, y recarga systemd.
5. **Pre-vuelo.** `nginx-container test` valida la configuración nueva con la imagen. Si falla, **deshace todo** y sale sin tocar el servicio.
6. **Conmuta**: `docker compose down` del servicio anterior y `systemctl enable --now nginx`. Mide e imprime el tiempo de corte.
7. **Compara** las mismas pruebas con la línea base. Si **cualquiera difiere**, retrocede automáticamente: para y deshabilita la unidad, borra el envoltorio, y levanta de nuevo el contenedor anterior con su configuración. Comprueba además que `nginx` del `PATH` sea el envoltorio, `nginx -t`, `systemctl reload nginx` y que el sitio siga respondiendo tras la recarga.
8. **Cierra:** actualiza las rutas del script de backup y el gancho de certbot (`systemctl reload nginx`), y renombra `/opt/nginx` a `/opt/nginx.pre-systemd` (queda para retroceder, no se borra).

### Resultado verificado en el servidor nuevo

| Prueba | Línea base | Después |
|---|---|---|
| `http://app` | 301 | 301 |
| frontend (navegador) | 307 | 307 |
| `robots.txt` | 200 | 200 |
| API sin token | 401 | 401 |
| SQLi / XSS contra la API | 403 / 403 | 403 / 403 |
| `/.git/config` (444) | sin respuesta | sin respuesta |
| SNI desconocido | sin respuesta | sin respuesta |
| CSP frontend / HSTS API | 1 / 1 | 1 / 1 |

- **Corte medido: 1,5 s.** Respuestas idénticas a la línea base; `nginx -t` y `systemctl reload nginx` funcionan desde el host.
- Proceso maestro como `root` y **4 workers como `www-data`** (uid 33).
- Después: el log de acceso y el de consola reciben cada petición, `modsec_audit.log` (de `www-data`) registra los bloqueos, la rotación de logs forzada reabre los archivos, `certbot renew --dry-run` pasa con el gancho nuevo, el backup incluye `/etc/nginx` y la unidad, y fail2ban sigue leyendo los mismos logs.

> **Otros procesos `nginx` en `ps`.** El contenedor de la aplicación web (`corelog-ui`) es una imagen oficial de nginx sirviendo archivos estáticos; sus *workers* corren como uid 101, que el host muestra con el nombre de la cuenta que casualmente tiene ese uid (`uuidd`). **No** son el Nginx de este documento. El nuestro: maestro `root` con `nginx: master process /usr/sbin/nginx -g daemon off;` y workers `www-data`.

### Retroceso manual

```bash
systemctl stop nginx && systemctl disable nginx
docker rm -f nginx
rm -f /etc/systemd/system/nginx.service /usr/local/sbin/nginx /usr/local/sbin/nginx-container && systemctl daemon-reload
mv /opt/nginx.pre-systemd /opt/nginx && (cd /opt/nginx && docker compose up -d)
```

(La imagen anterior sigue cargada: ver la política de etiquetas en [16](16-mantenimiento-y-versionado-de-la-imagen.md).)

## 4. Operación diaria (modo B)

```bash
systemctl status nginx          # estado del contenedor como servicio
systemctl reload nginx          # valida y recarga sin cortar
systemctl restart nginx         # parada ordenada + arranque
journalctl -u nginx -f          # salida del servicio (incluye la consola del contenedor)
nginx -t                        # valida la configuración desde el host
nginx -V                        # módulos y versión del binario del contenedor
```

Cambiar de versión de la imagen: construir o cargar la etiqueta nueva, `NGINX_IMAGE=nginx-clang:NUEVA nginx-container test` y, si pasa, editar `IMAGE` en el script (o exportar `NGINX_IMAGE` en la unidad) y `systemctl restart nginx`. El procedimiento completo, con línea base y retroceso, está en [16, sección 7](16-mantenimiento-y-versionado-de-la-imagen.md#7-desplegar-una-revisión-nueva-y-volver-atrás).

> **Editar la configuración.** `/etc/nginx` se monta `:ro` como **directorio** y la unidad hace `nginx -t` antes de recargar, así que `sed -i` y los editores funcionan con `systemctl reload nginx`. Ya no existe el problema del *bind mount* de un archivo suelto del modo A (ver [07](07-nginx-configuracion.md)).

## 5. Gancho de certbot

Tras renovar un certificado hay que recargar Nginx:

```sh
# /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh   (chmod 755)
#!/bin/sh
systemctl reload nginx
```

Es el gancho que usa el servidor (modo B): valida la configuración y recarga. En el modo A se usaría `docker exec nginx nginx -s reload`. Verificado con `certbot renew --dry-run --no-random-sleep-on-renew`. Ver [12](12-dns-y-certificados-tls.md).

## 6. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `systemctl is-active nginx` (modo B) | `active` |
| `systemctl cat nginx \| head -3` | Ruta `/etc/systemd/system/nginx.service` |
| `command -v nginx` | `/usr/local/sbin/nginx` |
| `nginx -V 2>&1 \| head -2` | `nginx/1.31.x`, `built by clang` |
| `systemctl is-enabled nginx` | `enabled` (arranca con el sistema) |
| Configuración rota a propósito + `systemctl reload nginx` | **Verificado:** la recarga falla (código 1), el servicio sigue `active` y el sitio responde con la configuración anterior |
| `systemctl restart nginx` | Vuelve y responde en ~1-2 s |
| Reinicio completo del servidor | El servicio debe arrancar solo. **No probado todavía** en el servidor nuevo (está habilitado y depende de `docker.service`); hacerlo en una ventana de mantenimiento |

Continuar con [11 · fail2ban](11-fail2ban.md).
