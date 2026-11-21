# 07 · Configuración de Nginx

← [06 Imagen Nginx](06-imagen-nginx-clang.md) · [Índice](README.md) · Siguiente: [08 WAF](08-waf-modsecurity-crs.md)

**Objetivo:** dejar Nginx (en contenedor, imagen `nginx-clang`, servicio del host) sirviendo las aplicaciones con TLS, cabeceras de seguridad, límites de tasa, bloqueo geográfico y de rutas privadas, y **logs en archivos y en consola**.

## 1. Estructura en el servidor

Disposición del sistema (la del servidor, modo B de [10](10-nginx-como-servicio-del-host.md)):

```
/etc/nginx/
├── nginx.conf                    ← configuración principal
├── mime.types
├── conf.d/                       ← se incluye en el bloque http
│   ├── security-maps.conf        ← detección de rutas privadas / escáneres
│   └── anubis-map.conf           ← rutas que saltan el desafío
├── snippets/                     ← fragmentos reutilizables (include)
│   ├── ssl-params.conf  proxy-params.conf  block-attacks.conf
│   ├── anubis-gate.conf  security-headers-app.conf  security-headers-anubis.conf
├── sites-enabled/                ← un archivo por aplicación
│   ├── 00-default.conf           ← servidor por defecto (host desconocido)
│   └── app.conf
└── geoip/GeoLite2-Country.mmdb
/var/www/{acme,default}/                   ← reto ACME y página por defecto
/etc/modsecurity/modsecurity.conf          ← ver documento 08
/usr/share/modsecurity-crs/                ← reglas OWASP CRS
/etc/letsencrypt/                          ← certificados (documento 12)
/var/log/nginx/  /var/log/modsecurity/  /var/log/modsec_audit.log
```

Archivos listos en [`recursos/nginx/`](recursos/nginx/). Copiarlos a `/etc/nginx/` y reemplazar `example.com` por el dominio real. (El modo Compose, más simple, usa `/opt/nginx/` con la misma estructura; ver la sección 2.)

## 2. El contenedor

El contenedor lo arranca la unidad `nginx.service` mediante [`nginx-container`](recursos/scripts/nginx-container) ([10](10-nginx-como-servicio-del-host.md)). Estos son los montajes que define:

| Montaje (host → contenedor) | Modo | Para qué |
|---|---|---|
| `/etc/nginx` → `/etc/nginx` | `ro` | Configuración, mapas, snippets, base GeoIP |
| `/etc/letsencrypt` → `/etc/letsencrypt` | `ro` | Certificados y llaves TLS |
| `/etc/modsecurity` → `/etc/modsecurity` | `ro` | Configuración del WAF |
| `/usr/share/modsecurity-crs` → igual | `ro` | Reglas OWASP CRS |
| `/var/www` → `/var/www` | `ro` | Reto ACME y página por defecto |
| `/var/log/nginx`, `/var/log/modsecurity`, `/var/log/modsec_audit.log` | lectura-escritura | Es lo único que Nginx y ModSecurity necesitan escribir |
| `/etc/localtime`, `/etc/timezone` → igual | `ro` | La imagen no trae `tzdata` ([06](06-imagen-nginx-clang.md)) |

Y las opciones de `docker run`: `--network host` (Nginx ve la IP real del cliente, sin NAT de Docker, escucha 80/443 directamente y alcanza a los demás contenedores por `127.0.0.1`), `--rm --init`, `--pull never` y `--userns host`.

Preparar antes la base GeoIP, los directorios y los archivos de log:

```bash
mkdir -p /etc/nginx/{conf.d,sites-enabled,snippets,geoip} /var/www/{acme,default} /var/log/nginx /var/log/modsecurity
touch /var/log/modsec_audit.log
curl -fsSL https://raw.githubusercontent.com/nginx/nginx/master/conf/mime.types -o /etc/nginx/mime.types   # la imagen no lo trae
chown -R www-data:www-data /var/log/modsec_audit.log /var/log/modsecurity ; chmod 750 /var/log/modsecurity ; chmod 640 /var/log/modsec_audit.log
```

### Workers sin privilegios (`user www-data`)

`nginx.conf` empieza con `user www-data;`: el proceso maestro corre como `root` (necesita leer las llaves TLS y abrir los puertos) y los **workers como `www-data`** (uid 33, el mismo de la imagen y del host Debian). Verificado: `ps -eo user,args | grep 'nginx: worker'` muestra `www-data`.

> **Dueño de los logs de ModSecurity.** Los *workers* escriben el log de auditoría, así que `/var/log/modsec_audit.log` y `/var/log/modsecurity` deben pertenecer a `www-data`. Si pertenecen a `root`, ModSecurity no puede escribir y el log queda vacío **sin dar error visible**. (Sin la directiva `user`, los workers correrían como `nobody` y el dueño tendría que ser `nobody:nogroup`.) El resto de logs (`access.log`, `error.log`, `attacks.log`, `geo_blocked.log`) los abre el proceso maestro como `root` y no necesitan cambio.

### Alternativa: Docker Compose (modo A)

Si no se necesita integración con el sistema, un `docker-compose.yml` es más simple ([`recursos/nginx/docker-compose.yml`](recursos/nginx/docker-compose.yml)). Monta lo mismo, pero con la configuración en `/opt/nginx/`:

```yaml
services:
  nginx:
    image: nginx-clang:1.31.9-r2
    container_name: nginx
    network_mode: host
    restart: unless-stopped
    volumes:
      - /opt/nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - /opt/nginx/mime.types:/etc/nginx/mime.types:ro
      - /opt/nginx/conf.d:/etc/nginx/conf.d:ro
      - /opt/nginx/sites-enabled:/etc/nginx/sites-enabled:ro
      - /opt/nginx/snippets:/etc/nginx/snippets:ro
      - /opt/nginx/www/acme:/var/www/acme:ro
      - /opt/nginx/www/default:/var/www/default:ro
      - /opt/nginx/geoip:/etc/nginx/geoip:ro
      - /etc/modsecurity:/etc/modsecurity:ro
      - /usr/share/modsecurity-crs:/usr/share/modsecurity-crs:ro
      - /var/log/modsecurity:/var/log/modsecurity
      - /var/log/modsec_audit.log:/var/log/modsec_audit.log
      - /etc/letsencrypt:/etc/letsencrypt:ro
      - /var/log/nginx:/var/log/nginx
      - /etc/localtime:/etc/localtime:ro
```

Con Compose, `nginx.conf` y `mime.types` se montan como **archivos suelto** y eso tiene una trampa (sección 8). El servidor empezó con este modo y se migró al modo B.

## 3. `nginx.conf` comentado

Archivo: [`recursos/nginx/nginx.conf`](recursos/nginx/nginx.conf).

```nginx
user www-data;                             # workers sin privilegios (el maestro sigue como root)
worker_processes auto;
pid /run/nginx.pid;
error_log /var/log/nginx/error.log warn;   # errores → archivo
error_log stderr warn;                     # errores → consola (docker logs)

load_module modules/ngx_http_headers_more_filter_module.so;
load_module modules/ngx_http_brotli_filter_module.so;
load_module modules/ngx_http_brotli_static_module.so;
load_module modules/ngx_http_geoip2_module.so;
load_module modules/ngx_http_modsecurity_module.so;
```

Bloques importantes dentro de `http { … }`:

| Bloque | Qué hace |
|---|---|
| `server_tokens off; more_clear_headers Server;` | No revela la versión ni el servidor |
| `client_*_timeout`, `client_max_body_size 20m`, `large_client_header_buffers` | Límites de cliente contra conexiones lentas y cuerpos enormes |
| `limit_req_zone … api_limit / auth_limit / req_limit_per_ip`, `limit_conn_zone` | Zonas de límite de tasa (sección 6) |
| `modsecurity on; modsecurity_rules_file …` | Activa el WAF para todo el servidor ([08](08-waf-modsecurity-crs.md)) |
| `geoip2 … map $geoip2_country_code $is_blocked_country` | Marca cada petición cuyo país no está permitido |
| `log_format main/blocked_geo` + `access_log` | Logs (sección 5) |
| `include conf.d/*.conf; include sites-enabled/*;` | Carga mapas y sitios |

### País permitido (GeoIP)

```nginx
map $geoip2_country_code $is_blocked_country {
    default 1;      # cualquier país → bloqueado
    PE      0;      # Perú → permitido
    ""      0;      # sin dato (IP privada / loopback) → permitido
}
```

Cambiar `PE` por el país o países necesarios (una línea por país). La base `GeoLite2-Country.mmdb` es de MaxMind (cuenta gratuita y licencia propia); debe actualizarse periódicamente. El servidor de referencia usa un servicio propio (`geoip-updater`) con un temporizador diario.

> **Efecto lateral importante.** Los *runners* de GitHub Actions están fuera del país permitido: la verificación del despliegue **no puede** consultar la URL pública. Debe hacerse desde dentro del servidor (ver [13](13-usuario-de-despliegue-y-cicd.md)).

## 4. Bloqueo de rutas privadas y escáneres

Archivos: [`conf.d/security-maps.conf`](recursos/nginx/conf.d/security-maps.conf) y [`snippets/block-attacks.conf`](recursos/nginx/snippets/block-attacks.conf).

Tres `map` clasifican cada petición:

- `$attack_uri`: `1` si la ruta apunta a algo privado: `/.env`, `/.git`, `/.aws`, `/.ssh`, `/wp-admin`, `/wp-login.php`, `phpmyadmin`, `/cgi-bin`, `/actuator`, `/etc/passwd`, `../`, `%00`, y archivos `.sql .bak .old .pem .key .log .conf .yml .php .sh …`.
- `$attack_agent`: `1` si el User-Agent es de un escáner conocido (`sqlmap`, `nikto`, `nmap`, `nuclei`, `wpscan`, `masscan`…).
- `$is_attack`: `1` si cualquiera de los dos lo es.

Y dos efectos:

```nginx
log_format attack '$remote_addr [$time_local] "$request" $status host="$host" ua="$http_user_agent"';
access_log /var/log/nginx/attacks.log attack if=$is_attack;   # alimenta a fail2ban
```

```nginx
# snippets/block-attacks.conf — incluido en cada server
if ($is_attack) { return 444; }                               # cierra sin responder
```

`444` es un código propio de Nginx: cierra la conexión sin enviar nada, de modo que el atacante no obtiene ni siquiera un error que analizar. El registro en `attacks.log` es lo que lee fail2ban ([11](11-fail2ban.md)) para el baneo permanente.

> **Adaptarlo a cada aplicación.** Si la aplicación publica rutas como `/admin` o sirve archivos `.yml` públicamente, hay que quitarlas del mapa. El mapa actual **no** bloquea `/admin` a propósito.

## 5. Logs: archivos y consola

| Log | Archivo | Consola (`docker logs nginx`) | Contenido |
|---|---|---|---|
| Acceso | `/var/log/nginx/access.log` | Sí (`access_log /dev/stdout main`) | Todas las peticiones, con país, host, tiempos y User-Agent |
| Errores y WAF | `/var/log/nginx/error.log` | Sí (`error_log stderr warn`) | Errores de Nginx y bloqueos de ModSecurity |
| Intentos de ataque | `/var/log/nginx/attacks.log` | No | Rutas privadas / escáneres (entrada de fail2ban) |
| Bloqueos por país | `/var/log/nginx/geo_blocked.log` | No | `BLOCKED ip=… country=…` |
| Auditoría WAF | `/var/log/modsec_audit.log` + `/var/log/modsecurity/` | No | Detalle de cada petición bloqueada |

Formato de acceso:

```
"$remote_addr [$time_local] \"$request\" $status $body_bytes_sent country=\"$geoip2_country_code\" host=\"$host\" rt=$request_time urt=\"$upstream_response_time\" ua=\"$http_user_agent\""
```

Nginx admite **varias directivas `error_log` y `access_log` en el mismo contexto**: la primera escribe a archivo y la segunda a consola. En un contenedor, `/dev/stdout` y `stderr` los recoge Docker; con `log-opts max-size/max-file` ([05](05-docker.md)) la consola queda acotada a 3 × 10 MB.

Verificado en el servidor: una misma petición aparece en `access.log` y en `docker logs nginx`; un bloqueo del WAF aparece en `error.log` y en consola.

```bash
docker logs -f nginx                       # consola, en vivo
tail -f /var/log/nginx/access.log          # archivo
tail -f /var/log/nginx/attacks.log         # intentos de ataque
```

### Rotación

[`recursos/logrotate/nginx-docker`](recursos/logrotate/nginx-docker) → `/etc/logrotate.d/nginx-docker`:

```
/var/log/nginx/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    sharedscripts
    postrotate
        docker exec nginx nginx -s reopen >/dev/null 2>&1 || true
    endscript
}

# ModSecurity mantiene su propio descriptor: truncar en lugar de reabrir
/var/log/modsec_audit.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

`nginx -s reopen` hace que Nginx abra de nuevo los archivos tras renombrarlos, pero **no** reabre el log de auditoría de ModSecurity: ese se rota con `copytruncate`. Probar siempre con una rotación forzada:

```bash
logrotate -f /etc/logrotate.d/nginx-docker
ls -la /var/log/nginx
# después de una petición, access.log debe tener líneas nuevas
```

## 6. Límites de tasa

```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=600r/m;        # API general
limit_req_zone $binary_remote_addr zone=auth_limit:10m rate=10r/m;        # login/registro
limit_req_zone $binary_remote_addr zone=req_limit_per_ip:10m rate=50r/s;  # global
limit_conn_zone $binary_remote_addr zone=conn_limit_per_ip:10m;
limit_req zone=req_limit_per_ip burst=100 nodelay;
limit_conn conn_limit_per_ip 100;
```

En el vhost de la API:

```nginx
limit_req zone=api_limit burst=60 nodelay;
location /api/v1/auth/ {
    limit_req zone=auth_limit burst=5 nodelay;
    limit_req_status 429;
    …
}
```

Verificado: 25 intentos de login seguidos dan 401 los primeros y **429** al superar la ráfaga; fail2ban (jail `nginx-limit-req`) banea a quien insiste. Ajustar `rate` y `burst` a la carga real.

## 7. Vhosts

[`recursos/nginx/sites-enabled/app.conf`](recursos/nginx/sites-enabled/app.conf) define tres bloques:

1. **Puerto 80:** aplica los bloqueos, atiende `/.well-known/acme-challenge/` y redirige el resto a HTTPS (301).
2. **Frontend** (`app.example.com`): TLS, bloqueo por país y por ataque, **Anubis** ([09](09-anubis.md)), CSP y proxy a `127.0.0.1:18081`.
3. **API** (`api.example.com`): TLS, bloqueos, límite de tasa (general y de login) y proxy a `127.0.0.1:18080`. **Sin Anubis**: la consume el navegador por `fetch` y un desafío la rompería.

El orden dentro de cada `server` importa: primero bloqueo por país, después por ataque, y solo entonces el trabajo real.

### Servidor por defecto

[`sites-enabled/00-default.conf`](recursos/nginx/sites-enabled/00-default.conf):

```nginx
server {                                   # IP directa u host desconocido en el puerto 80
    listen 80 default_server;
    server_name _;
    include /etc/nginx/snippets/block-attacks.conf;
    location /.well-known/acme-challenge/ { root /var/www/acme; }
    location / { return 444; }
}
server {                                   # SNI desconocido en el 443
    listen 443 ssl default_server;
    server_name _;
    ssl_reject_handshake on;               # corta el handshake TLS, no entrega ningún certificado
}
```

Sin este segundo bloque, una petición HTTPS a un subdominio sin vhost cae en el **primer** `server` que escuche en el 443 (el frontend) y recibe su certificado. Con `ssl_reject_handshake` el cliente recibe `tlsv1 unrecognized name` y nada más.

> **Cómo probarlo.** `curl https://127.0.0.1/` sin nombre de host **falla a propósito** (no hay SNI). Probar siempre con `--resolve app.example.com:443:127.0.0.1`.

### Cabeceras de seguridad

[`snippets/ssl-params.conf`](recursos/nginx/snippets/ssl-params.conf) (todos los vhosts): TLS 1.2/1.3, suites ECDHE con AES-GCM y ChaCha20, HSTS, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`. No lleva `ssl_stapling`: Let's Encrypt retiró el OCSP y Nginx solo emite avisos.

[`snippets/security-headers-app.conf`](recursos/nginx/snippets/security-headers-app.conf) (solo frontend):

```nginx
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' https://api.example.com; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()" always;
add_header Cross-Origin-Opener-Policy "same-origin" always;
```

La CSP es estricta porque la aplicación no carga nada externo: solo scripts propios y conexión a su API. **Antes de aplicar una CSP a otra aplicación** comprobar qué recursos externos usa (fuentes, analíticas, CDN) y ampliarla en consecuencia, o se rompe la página. Comprobar el HTML construido: no debe haber scripts en línea.

> `add_header` dentro de un `location` **reemplaza** (no suma) a los del nivel superior. Por eso la página del desafío de Anubis lleva su propio snippet con HSTS y su CSP.

## 8. Aplicar cambios

Modo B (servicio del host, el del servidor):

```bash
nginx -t                          # valida la configuración desde el host (envoltorio)
systemctl reload nginx            # valida de nuevo y recarga sin cortar conexiones
systemctl restart nginx           # parada ordenada + arranque (~1-2 s de corte)
```

La unidad ejecuta `nginx -t` **antes** de recargar: si la configuración es inválida la recarga **falla**, el servicio sigue activo y el sitio sigue sirviendo con la configuración anterior (verificado). `/etc/nginx` se monta como **directorio**, así que `sed -i` y los editores funcionan con un simple `reload`.

### Trampas del modo Compose (A)

> **Trampa del *bind mount* de un solo archivo.** En el modo A, `nginx.conf` y `mime.types` se montan como **archivos**. Editores y `sed -i` crean un archivo **nuevo** (otro *inode*): el contenedor sigue viendo el anterior y el `reload` no aplica el cambio, sin ningún error. Tras editar `nginx.conf` hay que **recrear** el contenedor: `docker compose up -d --force-recreate`. Los directorios (`conf.d`, `sites-enabled`, `snippets`) sí se refrescan con un `reload`. El modo B evita el problema.

> **`docker compose exec` en scripts.** Dentro de un script que se lee por `stdin` (por ejemplo `ssh host 'bash -s' < script.sh`), `docker compose exec` consume el resto del script como su entrada. Usar siempre `docker compose exec -T … </dev/null`, o ejecutar el script desde un archivo en el servidor.

## 9. Verificación

```bash
nginx -t
curl -sk -D - -o /dev/null --resolve app.example.com:443:127.0.0.1 https://app.example.com/robots.txt
```

| Prueba (desde un equipo del país permitido) | Esperado |
|---|---|
| `https://app.example.com/` con User-Agent de navegador | `307` hacia el desafío de Anubis |
| `https://api.example.com/api/v1/…` sin token | `401` (llega a la API) |
| `https://api.example.com/.git/config` | Sin respuesta (444) |
| SNI desconocido | `tlsv1 unrecognized name` |
| `http://…` | `301` a HTTPS |
| Cabeceras del frontend | `strict-transport-security`, `content-security-policy`, `permissions-policy` |
| `ps -eo user,args \| grep 'nginx: worker'` | `www-data` |
| `docker logs nginx` y `tail /var/log/nginx/access.log` | La misma petición en ambos |

Continuar con [08 · WAF](08-waf-modsecurity-crs.md).
