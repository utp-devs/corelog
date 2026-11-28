# Despliegue y operación

Guía operativa del despliegue continuo de Corelog en el VPS. No contiene secretos: los valores sensibles viven en los *secrets* del entorno `production` de GitHub y en `/opt/corelog/.env` dentro del servidor.

## Requisitos previos

- Un servidor Linux con Docker Engine y el plugin Docker Compose v2, y nginx en el host como única entrada pública.
- Un dominio para el frontend y otro para la API (o el mismo, ver más abajo) con certificado TLS vigente.
- Acceso a GHCR (`ghcr.io/utp-devs/corelog-api` y `corelog-ui`) desde el servidor.
- Un servidor SMTP para correo saliente (recuperación de contraseña, verificación, resumen y alertas).
- El archivo `/opt/corelog/.env` con los valores de la sección *Variables de entorno de la API*. Las obligatorias hacen que `docker compose` se niegue a arrancar si faltan: `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET`, `TWO_FACTOR_ENCRYPTION_KEY`, `APP_ENCRYPTION_KEY`, `APP_PUBLIC_URL` y `CORS_ALLOWED_ORIGINS` (o la variable anterior `CORS_ALLOWED_ORIGIN`).

Plantilla mínima de `/opt/corelog/.env` (valores de ejemplo, nunca los publiques):

```dotenv
DB_USER=corelog
DB_PASSWORD=<contraseña larga y única>
DB_NAME=corelog
JWT_SECRET=<salida de: openssl rand -base64 48>
TWO_FACTOR_ENCRYPTION_KEY=<salida de: openssl rand -base64 32>
APP_ENCRYPTION_KEY=<salida de: openssl rand -base64 32>
APP_PUBLIC_URL=https://corelog.example.com
PUBLIC_URL=https://corelog.example.com
CORS_ALLOWED_ORIGINS=https://corelog.example.com
TRUSTED_PROXIES=172.16.0.0/12
SMTP_HOST=smtp.example.com
SMTP_USER=<usuario>
SMTP_PASSWORD=<contraseña>
SMTP_FROM=CoreLog <no-reply@example.com>
```

## Arquitectura

```
Internet ─► nginx (host, TLS wildcard) ─┬─► frontend  127.0.0.1:18081  (contenedor ui)
                                        └─► API       127.0.0.1:18080  (contenedor api) ─► PostgreSQL (red interna)
```

- Frontend: `https://corelog.utpdevs.dev` · API: `https://api.corelog.utpdevs.dev`.
- `api` y `ui` solo escuchan en `127.0.0.1`; nginx es la única entrada pública. PostgreSQL no publica puertos.
- Las imágenes se publican en GHCR: `ghcr.io/utp-devs/corelog-api` y `ghcr.io/utp-devs/corelog-ui`.

## Flujo de despliegue

Workflow: `.github/workflows/deploy.yml`, se ejecuta en cada push a `main` (y manualmente con *workflow_dispatch*).

1. `build-and-push`: construye ambas imágenes y las publica con el SHA del commit y `latest`.
2. `deploy` (entorno `production`): abre SSH con la huella del servidor fijada, copia `deploy/docker-compose.prod.yml` y las migraciones a `/opt/corelog`, ejecuta `docker compose pull && up -d` y verifica que la API responda desde el propio servidor.
3. Antes de actualizar, el flujo hace un volcado de PostgreSQL en `/opt/corelog/backups/pre-deploy-<fecha>.sql.gz` (se conservan los 5 últimos; se omite en el primer despliegue).
4. El servicio `migrate` aplica las migraciones pendientes antes de arrancar la API. `migrate up` es idempotente: sin migraciones nuevas no hace nada.
5. La verificación final llama a `GET /health` de la API desde el propio servidor y exige `200`. La API y la base de datos tienen *healthcheck* en el compose; la imagen del frontend trae el suyo (`/healthz`). La API solo expone `/health` (vivacidad); no existe un endpoint `/ready` separado.

### Configuración necesaria en GitHub

| Tipo | Nombre | Uso |
|---|---|---|
| Secret (entorno `production`) | `VPS_HOST`, `VPS_USER` | Destino SSH |
| Secret (entorno `production`) | `VPS_SSH_KEY` | Llave privada de despliegue |
| Secret (entorno `production`) | `VPS_KNOWN_HOSTS` | Huella del servidor (evita MITM) |
| Secret / variable de entorno de la API | `TWO_FACTOR_ENCRYPTION_KEY` | Clave de 32 bytes en base64 que cifra los secretos de la verificación en dos pasos (obligatoria en producción) |
| Variable | `WEB_BUILD_CONFIGURATION` | Configuración de entorno de Angular del build (opcional, por defecto `production`) |

Recomendado: activar *required reviewers* en el entorno `production` para que un merge no despliegue sin aprobación.

## Actualizar desde la versión anterior a la plataforma de gestión

La versión anterior era una aplicación de un solo espacio. Las migraciones nuevas crean la organización predeterminada y asignan a ella a las personas, equipos, proyectos e incidencias existentes, y añaden roles, catálogos, flujos, notificaciones, automatización, tokens, webhooks, importación, índices y borrado lógico. Algunas reescriben datos, por lo que el orden importa.

1. **Respaldar primero.** Aunque el flujo automático hace un volcado, haz uno manual y guárdalo fuera del servidor:
   ```bash
   cd /opt/corelog
   docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > corelog-antes-de-actualizar-$(date +%F).sql.gz
   ```
   Respalda también el volumen de adjuntos (`corelog_attachments`).
2. **Completar `/opt/corelog/.env`** con las variables obligatorias nuevas (`JWT_SECRET`, `TWO_FACTOR_ENCRYPTION_KEY`, `APP_ENCRYPTION_KEY`, `APP_PUBLIC_URL`, `CORS_ALLOWED_ORIGINS`). Guarda las dos claves de cifrado en un gestor de secretos.
3. **Desplegar** (push a `main` o *workflow_dispatch*). Orden de operaciones: volcado previo, `docker compose pull`, `up -d`; el servicio `migrate` aplica las migraciones en orden y la API arranca solo si terminaron bien. Si una migración falla la API no arranca: revisa `docker compose logs migrate`.
4. **Verificar:** `curl -s http://127.0.0.1:18080/health` devuelve `{"status":"ok"}`; inicia sesión con la cuenta administradora, confirma que existe la organización predeterminada con sus proyectos y que las personas conservan sus roles.
5. **Revisar el registro de la API** (`docker compose logs api`) en busca de avisos de configuración.

**Reversión.** Existen migraciones `down`, pero deshacer la organización predeterminada pierde la relación entre los datos y su organización. La vía segura es restaurar el volcado previo: detener `api` y `ui`, restaurar la base (ver *Backups y restauración*) y volver a la imagen anterior con `IMAGE_TAG`. Hazlo cuanto antes: lo escrito con la versión nueva después del volcado se perdería.

## Rollback

Cada despliegue deja la imagen etiquetada con su SHA. Para volver a una versión anterior, en el servidor:

```bash
cd /opt/corelog
IMAGE_TAG=<sha-anterior> docker compose pull
IMAGE_TAG=<sha-anterior> docker compose up -d
```

Las migraciones no se revierten solas: si la versión anterior no es compatible con el esquema actual, restaurar el volcado previo (preferido) o ejecutar la migración `down` correspondiente antes de arrancar la imagen anterior.

## Agregar un subdominio

El DNS (`*.utpdevs.dev` y `*.corelog.utpdevs.dev`) y el certificado wildcard ya existen, así que solo falta un `server` en nginx que use ese certificado:

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name nuevo.utpdevs.dev;
    ssl_certificate     /etc/letsencrypt/live/utpdevs.dev/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/utpdevs.dev/privkey.pem;
    location / { proxy_pass http://127.0.0.1:<puerto>; }
}
```

Validar y recargar: `docker compose exec nginx nginx -t && docker compose exec nginx nginx -s reload`.

Un subdominio de tercer nivel distinto (por ejemplo `x.otro.utpdevs.dev`) requiere ampliar el certificado con `*.otro.utpdevs.dev`.

Los puertos de los contenedores deben publicarse siempre en `127.0.0.1`: Docker ignora las reglas de `ufw` para puertos publicados en `0.0.0.0`.

## Backups y restauración

Un cron diario respalda la configuración del servidor y un volcado de PostgreSQL (retención de 14 días). Para restaurar la base de datos:

```bash
zcat corelog-db-<fecha>.sql.gz | docker exec -i corelog-db-1 sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

Probar la restauración periódicamente en un entorno aparte.

Además de la base de datos, respalda los volúmenes de archivos: `corelog_attachments` (adjuntos), `corelog_avatars` (fotos de perfil), `corelog_logos` (logotipos de organización) y `corelog_imports` (CSV de importación). Ejemplo para un volumen:

```bash
docker run --rm -v corelog_corelog_attachments:/data:ro -v "$PWD":/backup alpine tar czf /backup/attachments-$(date +%F).tgz -C /data .
```

(El nombre real del volumen lleva el prefijo del proyecto de Compose: confírmalo con `docker volume ls`.)

## Proxy inverso y notificaciones en tiempo real

El flujo `GET /api/v1/notifications/stream` es una conexión SSE que queda abierta durante horas. Si un proxy la almacena en búfer o la corta por inactividad, las notificaciones llegan con retraso o la interfaz reconecta sin parar. La API responde con `X-Accel-Buffering: no`, envía un comentario de latido cada 20 s y exime esa ruta del plazo de 30 s de las peticiones normales, pero conviene configurar explícitamente cada nginx por el que pasa. En el nginx del host:

```nginx
location /api/v1/notifications/stream {
    proxy_pass http://127.0.0.1:18080;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
}
```

Si el dominio de la API es distinto del del frontend, esta regla va en el `server` de la API. Si el navegador usa el frontend como puerta de entrada (`/api/` hacia el contenedor `ui`), el nginx de la imagen del frontend también intermedia la conexión: su bloque `location /api/` usa `proxy_read_timeout 60s` y no desactiva el búfer de forma explícita. Con el latido de 20 s y la cabecera `X-Accel-Buffering` funciona, pero se recomienda añadir allí un bloque equivalente al anterior para el flujo.

## TLS

- Termina TLS en el nginx del host con certificado vigente y renovación automática; la API y el contenedor `ui` solo escuchan en `127.0.0.1`.
- Activa `HSTS_ENABLED=true` (es el valor por defecto del compose de producción) solo cuando todo el tráfico va por HTTPS.
- Usa `https://` en `APP_PUBLIC_URL`, `PUBLIC_URL` y `CORS_ALLOWED_ORIGINS`.
- `DB_SSLMODE` puede quedarse en `disable` mientras la base viva en la red interna del compose; el servicio `migrate` también se conecta sin TLS por esa red.
- `TRUSTED_PROXIES` debe incluir el rango de la red de Docker por la que llegan las peticiones (por ejemplo `172.16.0.0/12`), no solo `127.0.0.1`: nginx del host llega al contenedor desde la puerta de enlace del *bridge*.

## Registros y datos sensibles

La API escribe registros estructurados (JSON fuera de `development`) con el identificador de cada petición. No registra contraseñas, tokens ni secretos de webhooks, y el cuerpo de los correos solo se muestra en `development`. Aun así: no envíes registros a terceros sin revisarlos, rota los de Docker (`max-size`/`max-file` en el demonio o `logrotate`) y limita su acceso, porque contienen correos y direcciones IP. Nunca pegues el contenido de `/opt/corelog/.env` en incidencias o chats.


## Rotación de la llave de despliegue

1. Generar un par nuevo: `ssh-keygen -t ed25519 -f deploy_key -N ""`.
2. Reemplazar la línea correspondiente en `authorized_keys` del usuario de despliegue.
3. Actualizar el secret `VPS_SSH_KEY` y borrar la llave privada local.

## Variables de entorno de la API

La API lee su configuración del entorno (y de un archivo `.env` si existe). `.env.example` contiene la lista completa con valores de desarrollo. Las tablas resumen cada variable, su valor por defecto, para qué sirve y qué valor usar en producción.

### Servidor y base de datos

| Variable | Por defecto | Para qué sirve | Producción |
|---|---|---|---|
| `APP_ENV` | `development` | Modo de ejecución. Con `production` se ignoran `RATE_LIMIT_DISABLED` y `WEBHOOK_ALLOW_PRIVATE`, y se exige `TWO_FACTOR_ENCRYPTION_KEY` | `production` |
| `SERVER_PORT` | `8080` | Puerto de la API | `8080` (publicado solo en `127.0.0.1`) |
| `SERVER_READ_HEADER_TIMEOUT` | `5s` | Tiempo que tiene el cliente para enviar las cabeceras (defensa contra conexiones lentas) | Mantener |
| `SERVER_READ_TIMEOUT`, `SERVER_WRITE_TIMEOUT`, `SERVER_SHUTDOWN_TIMEOUT` | `10s`, `10s`, `15s` | Plazos del servidor HTTP y del apagado ordenado. El flujo de notificaciones fija su propio plazo de escritura | Subir `SERVER_WRITE_TIMEOUT` solo si hay exportaciones lentas |
| `SERVER_IDLE_TIMEOUT` | `120s` | Cierra las conexiones *keep-alive* sin uso | Mantener |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | `localhost`, `5432`, `corelog`, `corelog`, `corelog` | Conexión a PostgreSQL | Usuario y contraseña propios, nunca los de desarrollo |
| `DB_SSLMODE` | `disable` | Modo TLS de la conexión | `require` o superior si la base no está en la red interna del compose |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | `25`, `25`, `5m` | Pool de conexiones | Ajustar al `max_connections` de PostgreSQL |
| `CORS_ALLOWED_ORIGINS` | vacío | Orígenes permitidos por CORS, separados por comas. Tiene prioridad sobre `CORS_ALLOWED_ORIGIN` | Solo el dominio del frontend, con `https://` |
| `CORS_ALLOWED_ORIGIN` | `http://localhost:5173` | Variable anterior (un solo origen); se usa si no hay `CORS_ALLOWED_ORIGINS` | Preferir `CORS_ALLOWED_ORIGINS` |
| `JOBS_ENABLED` | `true` | Activa las tareas en segundo plano (envío de correo, resumen, suscripciones, depuración). Con `false` no corre ninguna | `true` |
| `ATTACHMENTS_DIR` | `data/attachments` | Carpeta de los archivos adjuntos | Volumen persistente incluido en el respaldo |
| `IMPORTS_DIR` | `data/imports` | Carpeta de los CSV subidos para importar | Volumen persistente |
| `AVATARS_DIR` | `data/avatars` | Carpeta de las fotos de perfil | Volumen persistente |
| `LOGOS_DIR` | `data/logos` | Carpeta de los logotipos de organización | Volumen persistente |

### Autenticación y cifrado

| Variable | Por defecto | Para qué sirve | Producción |
|---|---|---|---|
| `JWT_SECRET` | `change-me-in-production` | Firma de los tokens de sesión y de los enlaces de invitación | Cadena aleatoria larga (32 bytes o más); el valor por defecto no es aceptable |
| `JWT_EXPIRATION` | `24h` | Vigencia de la sesión | `8h` a `24h` |
| `TWO_FACTOR_ENCRYPTION_KEY` | vacío | Clave AES de 32 bytes en base64 que cifra los secretos de la verificación en dos pasos. Fuera de `development` la API no arranca sin ella | `openssl rand -base64 32`; no rotarla sin un plan, porque invalida los secretos guardados |
| `APP_ENCRYPTION_KEY` | vacío | Clave AES de 32 bytes en base64 que cifra los secretos de firma de los webhooks. Sin ella se deriva de `JWT_SECRET` y se registra un aviso | Definirla y distinta de `JWT_SECRET`, para que cambiar `JWT_SECRET` no invalide los webhooks |
| `LEGAL_ENFORCEMENT` | `warn` | `warn` solo informa de los consentimientos pendientes; `block` responde 403 `legal.consent_pending` hasta que la persona acepte | `block` si hay obligación legal de consentimiento |
| `REQUIRE_EMAIL_VERIFICATION` | `false` | Exige verificar el correo antes de iniciar sesión | `true` si el SMTP está configurado y probado |

### Protección contra abuso

| Variable | Por defecto | Para qué sirve | Producción |
|---|---|---|---|
| `RATE_LIMIT_DISABLED` | `false` | Apaga límites de frecuencia y bloqueo de cuentas (solo pruebas locales; se ignora en producción) | Sin definir |
| `RATE_LIMIT_LOGIN_PER_MIN` | `10` | Intentos de inicio de sesión por minuto y dirección | `10` o menos |
| `RATE_LIMIT_REGISTER_PER_HOUR` | `20` | Registros por hora y dirección | `5` a `20` |
| `RATE_LIMIT_FORGOT_PER_HOUR` | `5` | Solicitudes de recuperación de contraseña por hora | `5` |
| `RATE_LIMIT_PHONE_PER_HOUR` | `10` | Solicitudes de código de verificación de teléfono por hora | `10` |
| `API_TOKEN_RATE_LIMIT_PER_MIN` | `600` | Peticiones por minuto de cada token de API | Según la carga esperada de las integraciones |
| `LOCKOUT_THRESHOLD` | `5` | Intentos fallidos de una cuenta que activan el bloqueo | `5` |
| `LOCKOUT_DURATION` | `15m` | Duración del primer bloqueo; se duplica en cada bloqueo consecutivo hasta 24 horas | `15m` |
| `TRUSTED_PROXIES` | vacío | Direcciones o CIDR cuyo `X-Forwarded-For` se acepta | La dirección del nginx (por ejemplo `127.0.0.1`). Vacío hace que todas las peticiones parezcan venir del proxy y que los límites se apliquen a todas juntas |
| `HSTS_ENABLED` | `false` | Envía `Strict-Transport-Security` | `true` cuando todo el tráfico va por TLS |
| `MAX_JSON_BODY_BYTES` | `1048576` | Tamaño máximo de un cuerpo JSON (1 MiB) | Mantener salvo necesidad real |
| `WEBHOOK_ALLOW_PRIVATE` | `false` | Permite webhooks hacia direcciones privadas o locales (pruebas); se ignora en producción | Sin definir |

### Correo, enlaces y retención

| Variable | Por defecto | Para qué sirve | Producción |
|---|---|---|---|
| `SMTP_HOST` | vacío | Servidor SMTP. Sin valor, los correos solo se escriben en el log (el cuerpo solo se muestra en `development`) | Obligatorio |
| `SMTP_PORT` | `587` | Puerto SMTP | `587` o el de tu proveedor |
| `SMTP_USER`, `SMTP_PASSWORD` | vacío | Credenciales SMTP. La contraseña nunca se registra | Secretos del entorno, no del repositorio |
| `SMTP_FROM` | `CoreLog <no-reply@localhost>` | Remitente | Una dirección del dominio con SPF y DKIM configurados |
| `APP_PUBLIC_URL` | `http://localhost:5173` | Dirección pública del frontend para los enlaces de los correos (recuperación, verificación, resumen) | `https://` y el dominio real |
| `PUBLIC_URL` | vacío | Dirección pública base de los enlaces de invitación y de la suscripción ICS del calendario. Sin ella se usa el origen de la petición | El mismo dominio público |
| `EMAIL_DIGEST_SCHEDULE` | `daily at 08:00 UTC` | Cuándo se envía el resumen por correo. Acepta `daily at 08:00 America/Bogota`, `every 10m` o `cron 0 8 * * *` | La hora local de la mayoría de tus usuarios |
| `AUDIT_RETENTION_DAYS` | `365` | Días que se conserva la auditoría | Según la política de cumplimiento |
| `PURGE_ENABLED` | `false` | Activa la depuración diaria (04:00 UTC) de incidencias y proyectos eliminados hace más de 30 días | `true` tras probar con `PURGE_DRY_RUN=true` |
| `PURGE_DRY_RUN` | `true` | Con `true` la depuración solo registra lo que borraría; con `false` borra de verdad | `false` solo cuando el registro de prueba sea el esperado |
| `PURGE_RETENTION_DAYS` | `30` | Días que se conserva un elemento eliminado antes de depurarse (retención en desarrollo; revisa el valor vigente en `.env.example`) | `30` o la política de la organización |
| `PURGE_NOTIFICATION_RETENTION_DAYS` | `180` | Días que se conservan las notificaciones antiguas | Según la política de privacidad |
| `PURGE_FINGERPRINT_RETENTION_DAYS` | `90` | Días que se conservan las huellas de dispositivos de las sesiones | Según la política de privacidad |
| `PURGE_DELETE_ORPHANS` | `false` | Borra archivos huérfanos de las carpetas de adjuntos | `false` hasta probar en modo de prueba |

### Herramienta de datos de ejemplo

`cmd/seed` lee `SEED_PASSWORD` (obligatoria, sin valor por defecto, no se imprime) y `SEED_ALLOW_DATA_LOSS`. Con `SEED_ALLOW_DATA_LOSS=yes`, `--force-reset` también vacía una base que contiene cuentas que no son de la semilla. Nunca definas esta variable ni ejecutes la semilla contra una base de producción.

El `deploy/docker-compose.prod.yml` pasa a la API todas las variables de las tablas anteriores (excepto `RATE_LIMIT_DISABLED` y `WEBHOOK_ALLOW_PRIVATE`, que no deben existir en producción) y monta volúmenes persistentes para adjuntos, avatares, logotipos e importaciones. Las obligatorias usan `${VAR:?mensaje}`; el resto tiene un valor por defecto seguro. Los valores se leen de `/opt/corelog/.env`.

## Lista de endurecimiento para producción

Marca cada punto antes de abrir el servicio a usuarios reales.

- [ ] **TLS:** todo el tráfico público va por HTTPS (nginx con certificado vigente) y `HSTS_ENABLED=true`. `DB_SSLMODE` acorde a la red de la base.
- [ ] **Secretos:** `JWT_SECRET`, `TWO_FACTOR_ENCRYPTION_KEY` y `APP_ENCRYPTION_KEY` son aleatorios, distintos entre sí y guardados en los secretos del entorno, no en git. Hay una copia en un gestor de secretos: sin ellas no se pueden descifrar los secretos de 2FA ni de webhooks.
- [ ] **Contraseñas:** `DB_PASSWORD` propia; la semilla (`cmd/seed`) nunca se ha ejecutado en esta base.
- [ ] **Proxies:** `TRUSTED_PROXIES` contiene solo al nginx; la API y la base no están publicadas fuera de `127.0.0.1` o de la red interna.
- [ ] **CORS:** `CORS_ALLOWED_ORIGINS` lista solo el dominio del frontend.
- [ ] **Límites:** `RATE_LIMIT_*` y `LOCKOUT_*` con los valores por defecto o más estrictos; `RATE_LIMIT_DISABLED` sin definir.
- [ ] **Correo:** `SMTP_*` configurado, remitente con SPF y DKIM, y probadas la recuperación de contraseña y la verificación de correo con una cuenta real. Si activas `REQUIRE_EMAIL_VERIFICATION=true`, confirma antes que los correos llegan.
- [ ] **Enlaces:** `APP_PUBLIC_URL` y `PUBLIC_URL` apuntan al dominio público con `https://`.
- [ ] **Retención y privacidad:** `AUDIT_RETENTION_DAYS` acorde a la política; documentos legales publicados y `LEGAL_ENFORCEMENT` decidido; depuración (`PURGE_*`) probada primero en modo de prueba.
- [ ] **Backups:** el volcado de PostgreSQL y los volúmenes de adjuntos (`ATTACHMENTS_DIR`) e importaciones (`IMPORTS_DIR`) están en el respaldo diario, y la restauración se probó en un entorno aparte.
- [ ] **Tareas en segundo plano:** con varias réplicas de la API, confirma en el registro que el correo y el resumen no se envían duplicados; ante la duda, deja `JOBS_ENABLED=true` en una sola.
- [ ] **Integraciones:** `WEBHOOK_ALLOW_PRIVATE` sin definir; los tokens de API tienen caducidad y el alcance mínimo.
- [ ] **Observación:** revisa los registros de la API tras el despliegue (avisos como "APP_ENCRYPTION_KEY is not set" indican configuración pendiente) y comprueba que `GET /health` responde.
