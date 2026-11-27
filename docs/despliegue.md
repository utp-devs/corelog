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
