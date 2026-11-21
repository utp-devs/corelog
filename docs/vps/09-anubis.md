# 09 · Anubis (desafío anti-bots)

← [08 WAF](08-waf-modsecurity-crs.md) · [Índice](README.md) · Siguiente: [10 Nginx como servicio del host](10-nginx-como-servicio-del-host.md)

**Objetivo:** que los navegadores reales resuelvan un pequeño desafío de prueba de trabajo antes de ver el frontend, y que scrapers, bots de IA y herramientas automáticas queden fuera.

[Anubis](https://github.com/TecharoHQ/anubis) es un servicio independiente que Nginx consulta con `auth_request` antes de entregar cada página.

## 1. ¿"Anubis dentro" de la imagen de Nginx?

No: Anubis corre en **su propio contenedor** y Nginx lo consulta por `127.0.0.1`. Es una decisión deliberada, no una limitación:

| Alternativa | Problema |
|---|---|
| Meter Anubis en la imagen de Nginx | Dos procesos en un contenedor (necesita un supervisor, que la imagen mínima no tiene), actualizaciones acopladas y más superficie |
| Contenedor aparte (esta guía) | Una responsabilidad por contenedor; se actualiza y reinicia por separado; si Anubis cae, Nginx sigue (ver "Fallo abierto") |

Ambos servicios forman **una sola pila**: se levantan en el mismo servidor, comparten la red `host`/loopback y se vigilan juntos con el monitoreo ([14](14-backups-monitoreo-y-alertas.md)). Se mantienen como dos proyectos de Compose (`/opt/nginx` y `/opt/anubis`); unirlos en un único `docker-compose.yml` es posible pero **no se probó**.

## 2. Flujo

```
Navegador ─► Nginx ──auth_request──► Anubis (127.0.0.1:10020)
                │                         │
                │   204 = tiene cookie válida ──► sigue al frontend
                │   401 = sin cookie ──► 307 a /.within.website/?redir=… (página del desafío)
                ▼
            Frontend (127.0.0.1:18081)
```

1. Cada petición al frontend lanza una sub-petición interna a Anubis (`/_anubis_check`).
2. Si el visitante ya resolvió el desafío, Anubis responde 204 y Nginx entrega la página.
3. Si no, responde 401 y Nginx redirige a la página del desafío. Al resolverlo, Anubis emite una cookie firmada (7 días) y devuelve al visitante a la URL original.

## 3. Contenedor

Archivos: [`recursos/anubis/docker-compose.yml`](recursos/anubis/docker-compose.yml), [`politica.yaml`](recursos/anubis/politica.yaml), [`env.example`](recursos/anubis/env.example).

```yaml
services:
  anubis:
    image: ghcr.io/techarohq/anubis:latest
    container_name: anubis-corelog
    restart: unless-stopped
    env_file: .env
    environment:
      BIND: ":8923"
      TARGET: " "                    # vacío: solo modo auth_request, no hace de proxy
      DIFFICULTY: "4"
      COOKIE_SECURE: "true"
      COOKIE_SAME_SITE: Lax
      COOKIE_EXPIRATION_TIME: 168h
      COOKIE_PARTITIONED: "false"
      REDIRECT_DOMAINS: "app.example.com,example.com"
      POLICY_FNAME: /etc/anubis/politica.yaml
      SLOG_LEVEL: INFO
    volumes:
      - ./politica.yaml:/etc/anubis/politica.yaml:ro
    ports:
      - "127.0.0.1:10020:8923"      # solo loopback
    read_only: true
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
```

Endurecimiento del contenedor: sistema de archivos de solo lectura, sin capacidades de Linux, sin elevación de privilegios y puerto solo en loopback.

### Variables

| Variable | Significado |
|---|---|
| `ED25519_PRIVATE_KEY_HEX` | Llave que firma las cookies. **Obligatoria y secreta**: `openssl rand -hex 32` en `/opt/anubis/.env` (`chmod 600`). Si cambia, todos los visitantes vuelven a ver el desafío |
| `DIFFICULTY` | Ceros hexadecimales exigidos al hash de la prueba de trabajo. 4 es rápido en un navegador moderno; subirlo castiga más a los bots pero también a móviles lentos |
| `COOKIE_SECURE` / `COOKIE_SAME_SITE` | Solo HTTPS, `Lax` |
| `COOKIE_EXPIRATION_TIME` | Validez de la cookie tras resolver |
| `REDIRECT_DOMAINS` | Dominios a los que Anubis puede devolver al visitante (evita redirecciones abiertas) |
| `TARGET: " "` | Modo `auth_request`: solo valida; Nginx hace el proxy |

```bash
mkdir -p /opt/anubis && cd /opt/anubis
printf 'ED25519_PRIVATE_KEY_HEX=%s\n' "$(openssl rand -hex 32)" > .env
chmod 600 .env
chmod 644 politica.yaml docker-compose.yml        # el contenedor sin privilegios debe poder leer la política
docker compose up -d
```

> **`umask` y política ilegible.** Si en la misma sesión se creó `.env` con `umask 077`, `politica.yaml` puede haber quedado en `600` y el contenedor (usuario sin privilegios, `read_only`, sin `CAP_DAC_OVERRIDE`) entra en bucle de reinicios con `permission denied` al leerla. Solución: `chmod 644 politica.yaml`.

## 4. Política

[`politica.yaml`](recursos/anubis/politica.yaml): se evalúa en orden, gana la primera regla que coincide.

| Regla | Acción |
|---|---|
| User-Agent vacío | `DENY` (403) |
| Scrapers de IA (`GPTBot`, `ClaudeBot`, `CCBot`, `PerplexityBot`, `Bytespider`, …) | `DENY` |
| Escáneres y herramientas (`sqlmap`, `nikto`, `nmap`, `masscan`, `python-requests`, `playwright`, …) | `DENY` |
| Todo lo demás | `CHALLENGE` (dificultad 4, algoritmo `fast`) |

Para permitir a un bot legítimo (un monitor, un buscador) se añade **antes** de la regla general una entrada con `action: ALLOW` y su User-Agent. Ojo: un User-Agent se falsifica con facilidad; Anubis frena a los bots perezosos, no a los atacantes decididos (para eso están el WAF y fail2ban).

## 5. Integración con Nginx

### Mapa de exclusiones

[`conf.d/anubis-map.conf`](recursos/nginx/conf.d/anubis-map.conf): rutas que **no** pasan por el desafío (necesarias para que otros sistemas funcionen):

```nginx
map "$host|$request_uri" $anubis_skip {
    default 0;
    "~^[^|]+\|.*(\.\.|%2[eE]|%5[cC]|//)" 0;        # rutas con trucos de traversal: nunca se saltan
    "~^[^|]+\|/(\.well-known/|favicon\.ico($|\?)|robots\.txt($|\?))" 1;
}
```

### Puerta (`anubis-gate.conf`)

[`snippets/anubis-gate.conf`](recursos/nginx/snippets/anubis-gate.conf), incluido en el `server` del frontend:

```nginx
auth_request /_anubis_check;
error_page 401 = @anubis_challenge;

location = /_anubis_check {
    internal;
    if ($anubis_skip) { return 204; }
    proxy_pass http://127.0.0.1:10020/.within.website/x/cmd/anubis/api/check;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    …
    proxy_connect_timeout 2s;
    proxy_read_timeout 5s;
    proxy_intercept_errors on;
    error_page 500 502 503 504 = @anubis_abierto;      # fallo abierto
}
location @anubis_abierto { return 204; }

location ^~ /.within.website/ {                          # la página del desafío y sus recursos
    auth_request off;
    proxy_pass http://127.0.0.1:10020;
    …
    include /etc/nginx/snippets/security-headers-anubis.conf;
}
location @anubis_challenge {
    add_header Cache-Control "no-store" always;
    return 307 /.within.website/?redir=$scheme://$host$request_uri;
}
```

### Decisiones de diseño

- **Fallo abierto (`@anubis_abierto`).** Si Anubis no responde (5xx, caído), Nginx deja pasar en lugar de dejar el sitio fuera de línea. Es una elección de **disponibilidad sobre protección**; si se prefiere lo contrario, quitar esa línea y el sitio devolverá error mientras Anubis esté caído. El monitoreo ([14](14-backups-monitoreo-y-alertas.md)) alerta de que el contenedor `anubis-corelog` se detuvo.
- **Solo el frontend.** La API la consume el navegador con `fetch`/XHR: no puede resolver un desafío, y la cookie de Anubis del dominio del frontend no se envía al dominio de la API. Se protege con WAF, límites de tasa, CORS y fail2ban ([07](07-nginx-configuracion.md)).
- **CSP propia en la página del desafío.** Necesita scripts en línea y *workers* (`worker-src blob:`) para calcular el hash; la CSP estricta del frontend la rompería. [`security-headers-anubis.conf`](recursos/nginx/snippets/security-headers-anubis.conf) repite HSTS y define esa CSP (los `add_header` de un `location` reemplazan a los heredados).
- **Redirección 307**, no 302: conserva el método HTTP.

## 6. Probar

Desde un equipo del país permitido:

```bash
UA="Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537.36"
curl -s -o /dev/null -A "$UA" -w "%{http_code} -> %{redirect_url}\n" https://app.example.com/
# 307 -> https://app.example.com/.within.website/?redir=https://app.example.com/
curl -s -o /dev/null -A "sqlmap/1.7" -w "%{http_code}\n" https://app.example.com/      # 403 (política)
curl -s -o /dev/null -A "" -w "%{http_code}\n" https://app.example.com/                # 403 (sin UA)
curl -s -o /dev/null -A "$UA" -w "%{http_code}\n" "https://app.example.com/.within.website/?redir=https://app.example.com/"   # 200 (desafío)
curl -s -o /dev/null -A "$UA" -w "%{http_code}\n" https://app.example.com/robots.txt   # 200 (excluido)
```

> **Con `curl` no se puede completar el desafío** (requiere ejecutar JavaScript). La prueba final es abrir el sitio en un navegador real, resolverlo y comprobar que la página carga y que el login funciona. Esa comprobación **debe hacerse a mano** tras cada cambio de política o de CSP.

## 7. Operación

```bash
docker logs --tail 50 anubis-corelog
docker compose -f /opt/anubis/docker-compose.yml pull && docker compose -f /opt/anubis/docker-compose.yml up -d
```

- La imagen usa `latest`: fijar una versión concreta si se quiere control total de cuándo cambia el comportamiento.
- El almacén es `memory` (estado en RAM, se pierde al reiniciar). Las cookies ya emitidas dependen de la llave del `.env`, que persiste.
- Probar el sitio con un navegador real después de cada actualización.

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `docker ps --filter name=anubis` | `Up` |
| Navegador sin cookie → frontend | Desafío y luego la aplicación |
| `curl` con UA de navegador | `307` al desafío |
| `curl -A sqlmap` / sin UA | `403` |
| `curl /robots.txt` | `200` |
| `docker stop anubis-corelog` y repetir | El sitio sigue respondiendo (fallo abierto) y el monitoreo alerta |

Continuar con [10 · Nginx como servicio por defecto del host](10-nginx-como-servicio-del-host.md).
