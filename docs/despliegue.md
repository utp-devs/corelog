# Despliegue y operación

Guía operativa del despliegue continuo de Corelog en el VPS. No contiene secretos: los valores sensibles viven en los *secrets* del entorno `production` de GitHub y en `/opt/corelog/.env` dentro del servidor.

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
3. El servicio `migrate` aplica las migraciones pendientes antes de arrancar la API.

### Configuración necesaria en GitHub

| Tipo | Nombre | Uso |
|---|---|---|
| Secret (entorno `production`) | `VPS_HOST`, `VPS_USER` | Destino SSH |
| Secret (entorno `production`) | `VPS_SSH_KEY` | Llave privada de despliegue |
| Secret (entorno `production`) | `VPS_KNOWN_HOSTS` | Huella del servidor (evita MITM) |
| Secret / variable de entorno de la API | `TWO_FACTOR_ENCRYPTION_KEY` | Clave de 32 bytes en base64 que cifra los secretos de la verificación en dos pasos (obligatoria en producción) |
| Variable | `WEB_BUILD_CONFIGURATION` | Configuración de entorno de Angular del build (opcional, por defecto `production`) |

Recomendado: activar *required reviewers* en el entorno `production` para que un merge no despliegue sin aprobación.

## Rollback

Cada despliegue deja la imagen etiquetada con su SHA. Para volver a una versión anterior, en el servidor:

```bash
cd /opt/corelog
IMAGE_TAG=<sha-anterior> docker compose pull
IMAGE_TAG=<sha-anterior> docker compose up -d
```

Las migraciones no se revierten solas: si la versión anterior no es compatible con el esquema actual, ejecutar la migración `down` correspondiente antes.

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
| `SERVER_READ_TIMEOUT`, `SERVER_WRITE_TIMEOUT`, `SERVER_SHUTDOWN_TIMEOUT` | `10s`, `10s`, `15s` | Plazos del servidor HTTP y del apagado ordenado | Subir `SERVER_WRITE_TIMEOUT` solo si hay exportaciones lentas |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | `localhost`, `5432`, `corelog`, `corelog`, `corelog` | Conexión a PostgreSQL | Usuario y contraseña propios, nunca los de desarrollo |
| `DB_SSLMODE` | `disable` | Modo TLS de la conexión | `require` o superior si la base no está en la red interna del compose |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | `25`, `25`, `5m` | Pool de conexiones | Ajustar al `max_connections` de PostgreSQL |
| `CORS_ALLOWED_ORIGINS` | vacío | Orígenes permitidos por CORS, separados por comas. Tiene prioridad sobre `CORS_ALLOWED_ORIGIN` | Solo el dominio del frontend, con `https://` |
| `CORS_ALLOWED_ORIGIN` | `http://localhost:5173` | Variable anterior (un solo origen); se usa si no hay `CORS_ALLOWED_ORIGINS` | Preferir `CORS_ALLOWED_ORIGINS` |
| `JOBS_ENABLED` | `true` | Activa las tareas en segundo plano (envío de correo, resumen, suscripciones, depuración). Con `false` no corre ninguna | `true` |
| `ATTACHMENTS_DIR` | `data/attachments` | Carpeta de los archivos adjuntos | Volumen persistente incluido en el respaldo |
| `IMPORTS_DIR` | `data/imports` | Carpeta de los CSV subidos para importar | Volumen persistente |

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

### Herramienta de datos de ejemplo

`cmd/seed` lee `SEED_PASSWORD` (obligatoria, sin valor por defecto, no se imprime) y `SEED_ALLOW_DATA_LOSS`. Con `SEED_ALLOW_DATA_LOSS=yes`, `--force-reset` también vacía una base que contiene cuentas que no son de la semilla. Nunca definas esta variable ni ejecutes la semilla contra una base de producción.

El `deploy/docker-compose.prod.yml` actual solo pasa a la API las variables básicas (`APP_ENV`, `TWO_FACTOR_ENCRYPTION_KEY`, `CORS_ALLOWED_ORIGIN`, base de datos, `JWT_*` y `ATTACHMENTS_DIR`). Para usar el resto, añádelas a la sección `environment` del servicio `api`, leyéndolas de `/opt/corelog/.env`. La carpeta de `IMPORTS_DIR` también debe vivir en un volumen.

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
