# 10 · Nginx como servicio por defecto del host

← [09 Anubis](09-anubis.md) · [Índice](README.md) · Siguiente: [11 fail2ban](11-fail2ban.md)

**Objetivo:** que, para el sistema operativo y para quien administra, el Nginx del contenedor **sea** el Nginx del servidor: `systemctl start|stop|reload|status nginx` y `nginx -t`, `nginx -V`, `nginx -s reload` funcionan en el host igual que con un paquete instalado, y arranca solo con el sistema.

## 1. Dos modos

| | A · Compose (servidor nuevo) | B · Servicio `systemd` + envoltorio (servidor de referencia) |
|---|---|---|
| Cómo arranca | `restart: unless-stopped` en Docker | Unidad `nginx.service` de systemd |
| `systemctl status nginx` | No (se usa `docker compose ps`) | Sí |
| `nginx -t` en el host | No (`docker compose exec nginx nginx -t`) | Sí (envoltorio en `/usr/local/sbin/nginx`) |
| certbot, logrotate y scripts que llaman a `nginx` | Hay que adaptarlos | Funcionan sin cambios |
| Disposición de archivos | `/opt/nginx/…` | `/etc/nginx`, `/etc/letsencrypt`, `/var/www`, `/var/log/nginx` (rutas del sistema) |
| Verificado en | Servidor nuevo (Debian 13) | Servidor de referencia (Debian 12), en producción |

El modo A es el más simple y es el que corre en el servidor nuevo. El modo B es el que convierte al contenedor en "el Nginx por defecto del host"; se documenta completo para adoptarlo (no se aplicó en el servidor nuevo).

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
IMAGE="${NGINX_IMAGE:-nginx-clang:1.31.6}"
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

Antes: la configuración debe estar en las rutas del host (`/etc/nginx`, `/etc/modsecurity`, …) en lugar de `/opt/nginx`, o editar la lista `MOUNTS` de `nginx-container` para apuntar a `/opt/nginx`.

### 2.6 Desplazar al Nginx de la distribución

Dos servidores no pueden escuchar el mismo puerto. Si existe un `nginx` del paquete de Debian:

```bash
systemctl disable --now nginx            # antes de instalar la unidad nueva
apt-get purge -y 'nginx*'                # o apt-mark hold nginx para que no vuelva
```

La unidad en `/etc/systemd/system/nginx.service` tiene prioridad sobre la de `/lib/systemd/system/`, por lo que **sustituye** a la de la distribución sin borrarla.

## 3. Migrar un servidor en producción (con retroceso automático)

El servidor de referencia pasó de Nginx de paquete al contenedor con un script que garantiza volver atrás si algo cambia. Esqueleto:

1. **Antes de empezar**, registrar una *línea base*: por cada sitio, el código HTTP y HTTPS y el tamaño de la respuesta (`curl -sk --resolve host:443:127.0.0.1 …`).
2. **Copia de seguridad** de la unidad actual (`systemctl cat nginx`), de `nginx -V` y de `logrotate.d/nginx`.
3. **Pre-vuelo**: `nginx-container test` valida la configuración *viva* con la imagen. Si falla, no se toca nada.
4. **Parar** el Nginx del paquete (aquí empieza el corte) y anotar la hora.
5. **Instalar** wrapper, script, unidad y logrotate; `daemon-reload`; `systemctl start nginx`.
6. **Esperar** a que escuche el 443 y calcular e imprimir el tiempo de corte (el script lo mide; conviene anotarlo).
7. **Verificar cada sitio** contra la línea base: mismo código y tamaño. Comprobar también WebSocket (`101 Switching Protocols`), túneles TLS en `stream`, `nginx -t` por el envoltorio y `systemctl reload nginx`.
8. **Retroceso automático** si cualquier comprobación difiere: parar y borrar el contenedor, quitar unidad y envoltorios, restaurar el logrotate original y arrancar el Nginx del paquete.

Los datos de retroceso se conservan en un directorio con fecha.

## 4. Operación diaria (modo B)

```bash
systemctl status nginx          # estado del contenedor como servicio
systemctl reload nginx          # valida y recarga sin cortar
systemctl restart nginx         # parada ordenada + arranque
journalctl -u nginx -f          # salida del servicio (incluye la consola del contenedor)
nginx -t                        # valida la configuración desde el host
nginx -V                        # módulos y versión del binario del contenedor
```

Cambiar de versión de la imagen: construir o cargar la etiqueta nueva, `NGINX_IMAGE=nginx-clang:NUEVA nginx-container test` y, si pasa, editar `IMAGE` en el script (o exportar `NGINX_IMAGE` en la unidad) y `systemctl restart nginx`.

## 5. Gancho de certbot

Tras renovar un certificado hay que recargar Nginx:

```sh
# /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh   (chmod 755)
#!/bin/sh
docker exec nginx nginx -s reload       # modo A
# systemctl reload nginx                # modo B
```

Con el envoltorio del modo B también sirve `nginx -s reload`. Ver [12](12-dns-y-certificados-tls.md).

## 6. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `systemctl is-active nginx` (modo B) | `active` |
| `systemctl cat nginx \| head -3` | Ruta `/etc/systemd/system/nginx.service` |
| `command -v nginx` | `/usr/local/sbin/nginx` |
| `nginx -V 2>&1 \| head -2` | `nginx/1.31.x`, `built by clang` |
| `systemctl stop nginx; systemctl start nginx` | Vuelve y responde |
| Reinicio del servidor | El servicio arranca solo |
| Configuración rota a propósito + `systemctl reload nginx` | La recarga **falla** y el sitio sigue sirviendo con la configuración anterior |

Continuar con [11 · fail2ban](11-fail2ban.md).
