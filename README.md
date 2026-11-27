<p align="center">
  <img alt="CoreLog" src="docs/assets/corelog-banner.png" width="640">
</p>

<h1 align="center">CoreLog - Pequeño pero poderoso gestor de tickets</h1>

<p align="center">
  Gestión de incidencias y proyectos para equipos de soporte: tableros kanban y scrum, flujos de trabajo,
  comentarios, adjuntos y control de acceso por equipo y rol, todo en una sola aplicación.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.27.4-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="chi" src="https://img.shields.io/badge/chi-v5.5.0-007ec6?style=for-the-badge">
  <img alt="sqlx" src="https://img.shields.io/badge/sqlx-v1.4.0-007ec6?style=for-the-badge">
  <img alt="golang-jwt" src="https://img.shields.io/badge/golang--jwt-v5.3.1-000000?style=for-the-badge&logo=jsonwebtokens&logoColor=white">
  <img alt="go-playground validator" src="https://img.shields.io/badge/go--playground_validator-v10.30.5-007ec6?style=for-the-badge">
  <img alt="godotenv" src="https://img.shields.io/badge/godotenv-v1.5.1-007ec6?style=for-the-badge">
  <img alt="Angular" src="https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white">
  <img alt="PrimeNG" src="https://img.shields.io/badge/PrimeNG-21-007ec6?style=for-the-badge">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-6.0.3-3178C6?style=for-the-badge&logo=typescript&logoColor=white">
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-17-4169E1?style=for-the-badge&logo=postgresql&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white">
  <img alt="Docker Compose" src="https://img.shields.io/badge/Docker_Compose-v2-2496ED?style=for-the-badge&logo=docker&logoColor=white">
  <img alt="nginx" src="https://img.shields.io/badge/nginx-1.30-009639?style=for-the-badge&logo=nginx&logoColor=white">
  <img alt="golang-migrate" src="https://img.shields.io/badge/golang--migrate-v4.17.1-007ec6?style=for-the-badge">
  <img alt="Node.js" src="https://img.shields.io/badge/Node.js-24-5FA04E?style=for-the-badge&logo=nodedotjs&logoColor=white">
  <img alt="npm" src="https://img.shields.io/badge/npm-11.19.0-CB3837?style=for-the-badge&logo=npm&logoColor=white">
  <img alt="GitHub Actions" src="https://img.shields.io/badge/GitHub_Actions-2088FF?style=for-the-badge&logo=githubactions&logoColor=white">
  <img alt="OpenAPI" src="https://img.shields.io/badge/OpenAPI-3-6BA539?style=for-the-badge&logo=openapiinitiative&logoColor=white">
</p>

Un minisistema para la gestión de incidentes y la resolución de tickets.
Este proyecto se está desarrollando con fines educativos como parte del curso "Herramientas de Desarrollo" de la Universidad Tecnológica del Perú.

## Tabla de contenidos

- [Integrantes y estado del proyecto](#team-and-status)
- [Arquitectura](#architecture)
- [Tecnologías del backend](#backend-stack)
- [Tecnologías del frontend](#frontend-stack)
- [Requisitos previos](#prerequisites)
- [Primeros pasos](#getting-started)
- [Scripts de configuración](#setup-scripts)
- [Variables de entorno](#environment-variables)
- [Referencia de la API](#api-reference)
- [Comandos disponibles](#available-commands)
- [Docker](#docker)
- [Integración y entrega continua](#integración-y-entrega-continua)
- [Convenciones del proyecto](#project-conventions)
- [Solución de problemas](#troubleshooting)

## Integrantes y estado del proyecto

Grupo 11, curso Herramientas de Desarrollo (UTP).

| Integrante | Rol en el equipo |
|---|---|
| Fernandez Gutierrez, Jhon Samuel | Responsable de repositorio |
| Vacas Neyra, Elton Smith | Responsable funcional |
| Navarro López, Jonathan Fabrizio | Responsable de calidad / automatización |
| Pariachi Herrera, Dalia Sahomi | Responsable de despliegue / documentación |
| Correa Ángeles, Daleth | Tablero de actividades y mejoras de frontend |

Estado actual:

* APF1 (semana 6): base del proyecto, ramas por funcionalidad, merge y conflicto resuelto en el repositorio remoto.
* APF2 (semana 11): completado. Pull Requests con revisión cruzada, tablero de actividades, `docker compose` completo con migraciones automáticas, protección de `main` y primeras Releases.
* APF3 (semana 15): en curso. Integración continua con reporte de cobertura, análisis de calidad y seguridad en cada Pull Request, registro de incidencias en Issues y tablero, y Release automática al fusionar código (última publicada: `v0.2.15`). Pendiente: entrega continua y entorno en la nube.

## Arquitectura

Tanto el backend como el frontend siguen una arquitectura hexagonal, es decir, basada en puertos y adaptadores. La lógica de negocio, incluido el dominio y los casos de uso, no tiene conocimiento de mecanismos de entrega como HTTP, SQL o Angular. Los adaptadores se conectan al núcleo mediante interfaces explícitas llamadas puertos.


```

corelog/
├── cmd/api/                    Punto de composición de Go, conecta todo e inicia el servidor HTTP
├── internal/
│   ├── platform/                infraestructura transversal: configuración, logger, controladores de BD, servidor HTTP, JWT, middleware
│   ├── user/                    hexágono de usuario para autenticación e identidad
│   │   ├── domain/                 entidades y reglas de negocio, sin dependencias externas
│   │   ├── application/
│   │   │   ├── port/                 in contiene interfaces de casos de uso, out contiene interfaces de repositorio
│   │   │   └── usecase/              implementaciones de casos de uso, el núcleo de la aplicación
│   │   ├── adapter/
│   │   │   ├── in/http/              adaptador de entrada: handlers HTTP, DTOs y rutas
│   │   │   ├── out/postgres/         adaptador de salida: repositorio basado en sqlx para producción
│   │   └── module.go                 conecta el dominio, la aplicación y el adaptador
│   ├── team/                    hexágono de equipos para agrupar usuarios, misma estructura que user
│   └── ticket/                  hexágono de tickets para incidentes, misma estructura que user, además de adapter/out/memory y adapter/out/directory
├── migrations/
│   ├── postgres/                 migraciones SQL versionadas para producción, aplicadas con golang-migrate
├── docker/                      Dockerfiles y configuración de nginx
├── docker-compose.yaml           PostgreSQL, migraciones automáticas, API y UI con un solo comando
├── openapi.yaml                  Contrato AbreAPI 3 para toda la API, mantenido sincronizado con la referencia de API de abajo
├── scripts/                      setup.sh and setup.ps1, comandos de configuración del entorno, compilación y empaquetado
└── web/                          frontend Angular (portal, workspace y admin), documentado en web/README.md

```

### Flujo de solicitud, ejemplo: crear ticket

`HTTP request -> adapter/in/http as the Handler -> application/port/in as the Service interface -> application/usecase for business orchestration -> domain for Ticket invariants -> application/port/out as the Repository interface -> adapter/out/postgres via sqlx`

## Tecnologías del backend

- **Lenguaje**: Go
- **Router**: [chi](https://github.com/go-chi/chi)
- **Configuración**: [godotenv](https://github.com/joho/godotenv) loads `.env` into the process environment, then [caarlos0/env](https://github.com/caarlos0/env) parses that environment straight into typed config structs using struct tags, so `internal/platform/config` has no manual parsing code
- **Acceso a la base de datos**: `database/sql` plus [sqlx](https://github.com/jmoiron/sqlx), no ORM, explicit SQL
- **Autenticación**: JWT via [golang-jwt](https://github.com/golang-jwt/jwt) plus bcrypt password hashing
- **Validación**: [go-playground/validator](https://github.com/go-playground/validator)
- **Migraciones**: SQL files under `migrations/postgres/`, run with [golang-migrate](https://github.com/golang-migrate/migrate)

### PostgreSQL, conjunto único de persistencia

`internal/user/module.go`, `internal/team/module.go`, `internal/project/module.go` y `internal/ticket/module.go` reciben repositorios PostgreSQL mediante puertos. El composition root de `cmd/api/main.go` usa `database.NewPostgres` y los adaptadores `adapter/out/postgres`.

Las migraciones versionadas viven en `migrations/postgres/` y se aplican mediante golang-migrate en Docker o desde la CLI. PostgreSQL es obligatorio tanto en desarrollo como en producción.

El ticket hexagon admite opcionalmente `TICKET_REPOSITORY=memory` para demos y pruebas aisladas. Esta opción no sustituye PostgreSQL para el resto de módulos.

Los adjuntos de issues se almacenan en `data/attachments` en desarrollo local. Docker Compose monta el volumen persistente `corelog_attachments` en `/var/lib/corelog/attachments`; `ATTACHMENTS_DIR` permite configurar otra ruta. La API acepta PDF, PNG, JPEG, GIF, WebP, texto plano y CSV de hasta 10 MiB por archivo.

Los miembros de un proyecto pueden seguir o dejar de seguir sus issues. Quien comenta y la persona asignada se agregan automáticamente a los seguidores. Cada usuario puede guardar filtros personales de estado, prioridad, tipo, fechas y texto para reutilizarlos en el listado.

Cada proyecto recibe un tablero predeterminado con columnas para abierto, en progreso, resuelto y cerrado. La migración crea tableros iniciales para los proyectos existentes; la creación de nuevos proyectos guarda el proyecto, su líder, el tablero, sus columnas y el flujo de estados predeterminado en una sola transacción. La vista protegida `GET /api/v1/projects/{id}/board` devuelve las columnas ordenadas, tarjetas con tipo, resumen, responsable, prioridad y puntos, además del conteo por columna. En la UI se abre desde la tarjeta de cada proyecto.

Las pruebas de integración de adjuntos, seguidores, filtros guardados y tableros usan PostgreSQL real y se activan con `CORELOG_TEST_DATABASE_URL`; crean y eliminan esquemas temporales aislados.

### Equipos y asignación

Every user optionally belongs to one team, tracked as a nullable `team_id` on the user. Teams themselves are a small, independent hexagon under `internal/team`, managed by administrators through the `/teams` endpoints.

When a ticket is assigned, `internal/ticket/application/usecase/ticket_service.go` checks that the user performing the assignment and the user being assigned share the same team, using an outbound port, `ticket/application/port.UserDirectory`, implemented in `internal/ticket/adapter/out/directory` sobre PostgreSQL via `sqlx.DB.Rebind`. A user with no team can neither assign tickets nor be assigned one, and a mismatched team returns `403`.

There is no dedicated admin bootstrap step: the very first account ever registered becomes an admin automatically, and every account after that defaults to requester. Admins manage teams and assign other users to them from the Teams screen in the frontend.

## Tecnologías del frontend

Consulta [`web/README.md`](web/README.md) para la estructura del frontend. Stack: Angular 21 con TypeScript, componentes standalone y signals, PrimeNG, chart.js, Angular CDK y SCSS con tokens propios.

## Prerequisitos de trabajo

- Go 1.27 or newer
- Node.js 24 or newer, and npm
- Docker and Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI, needed only to run PostgreSQL migrations manually

## Primeros pasos

Ejecutar CoreLog localmente significa ejecutar dos procesos al mismo tiempo: la API de Go y el frontend de Angular. Usa dos ventanas o pestañas de terminal separadas, una para el backend y otra para el frontend, y mantén ambos procesos ejecutándose mientras trabajas.

### 1. Iniciar el backend

Usa PostgreSQL tanto en desarrollo como en producción. Ejecuta estos comandos desde la raíz del repositorio.

```sh
cp .env.example .env
make docker-up
```

La API queda escuchando en `http://localhost:8080` y el frontend en `http://localhost:5173`. No hace falta correr `make migrate-up` ni el paso 2.

### 2. Iniciar el frontend

Abre una segunda terminal, mantén el backend del paso 1 ejecutándose y, desde la raíz del repositorio, ejecuta:

```sh
cd web
npm install
npm run start:portal

```

El frontend llama a la API en `/api/v1`; el servidor de desarrollo la redirige al backend del paso 1 (`proxy.conf.json`). Deja este proceso ejecutándose: el portal queda escuchando en `http://localhost:4200`.

### 3. Verificar que ambos estén ejecutándose

Abre `http://localhost:5173` in a browser, registra una nueva cuenta, inicia sesión y crea un ticket. La página se comunica con la API at `http://localhost:8080` in the background.

Para comprobar el backend por separado, sin el navegador:

```sh
curl http://localhost:8080/health

```

Esto devuelve `{"status":"ok"}` cuando la API es accesible.

## Scripts de configuración

`scripts/setup.sh`, for bash, and `scripts/setup.ps1`, for PowerShell, wrap the steps above into single commands. Both scripts expose the same commands and behave the same way, so pick whichever matches your shell.

| Comando | Descripción |
| --- | --- |
| `check` | Indica si Go, Node, npm, Docker y golang-migrate están disponibles |
| `env` | Create `.env` from its example file if missing, without overwriting it |
| `dev` | Prepare a local development environment backed by PostgreSQL: runs `env`, starts PostgreSQL and migrations, downloads Go modules and installs frontend dependencies |
| `prod` | Prepare a producción environment backed by PostgreSQL via Docker: verifies Docker, runs `env`, starts the `db` and `api` containers and applies migrations |
| `build` | Compila el binario del backend en `bin/api`, `bin/api.exe` on Windows, y compila el frontend en `web/dist` |
| `package` | Run `build`, then assemble a distributable archive under `dist/` que contiene el binario del backend, las migraciones de PostgreSQL y la compilación del frontend, y comprímelo como `corelog-package.tar.gz` in bash or `corelog-package.zip` in PowerShell |
| `clean` | Elimina `bin/`, `web/dist/`, `dist/`, and the packaged archive |

Ejemplos:

```sh
./scripts/setup.sh dev
./scripts/setup.sh prod
./scripts/setup.sh package

```

```powershell
.\scripts\setup.ps1 dev
.\scripts\setup.ps1 prod
.\scripts\setup.ps1 package

```

Ejecutar cualquiera de los scripts sin un comando o con uno no reconocido muestra la lista de comandos anterior.

## Datos de prueba (seed)

`cmd/seed` siembra la base de datos con datos deterministas: el mismo escenario produce siempre los mismos datos. La contraseña de todas las cuentas viene de la variable `SEED_PASSWORD`, que es obligatoria, no tiene valor por defecto y nunca se imprime.

```sh
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario demo
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario e2e
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario scale --issues 50000
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario demo --force-reset
```

```powershell
$env:SEED_PASSWORD = '<contrasena>'; .\scripts\seed.ps1 --scenario demo
```

`scripts/seed.sh` y `scripts/seed.ps1` son envoltorios que exigen `SEED_PASSWORD` y reenvían el resto de argumentos a `cmd/seed`.

| Escenario | Contenido |
| --- | --- |
| `demo` | 21 cuentas (`admin@corelog.local`, `agent@corelog.local`, `requester@corelog.local` y personas `@corelog.test`), 5 equipos, 4 proyectos (CORE, OPS, SEC, WEB), 120 incidencias con comentarios, seguidores, enlaces, sprints, filtros guardados y notificaciones |
| `e2e` | Cuentas de las pruebas de Playwright (`admin@corelog.local`, `agent@corelog.local`, `requester@corelog.local`, `ana.torres@corelog.test`), 1 equipo, 2 proyectos (SCRUM, KAN), 15 incidencias, 1 sprint activo con incidencias, comentarios y notificaciones |
| `scale` | Organización de `demo` más una cuenta extra por cada 1000 incidencias, y `--issues N` incidencias (50 000 por defecto) con sus comentarios y etiquetas. Tarda unos 30 s con 50 000 |

Opciones:

- `--scenario`: `demo` (por defecto), `e2e` o `scale`.
- `--issues N`: número de incidencias del escenario `scale`.
- `--force-reset`: vacía las tablas de la aplicación (nunca `schema_migrations`) antes de sembrar. Sin esta opción, el comando se niega a sembrar una base de datos que ya tiene usuarios.

Las pruebas de la siembra necesitan PostgreSQL: con `CORELOG_TEST_DATABASE_URL` definida, `go test ./cmd/seed/ -count=1` usa esquemas aislados. Sin esa variable, las pruebas de base de datos se omiten.

## Variables de entorno

All variables live in `.env`, copied from `.env.example`.

| Variable | Predeterminado | Descripción |
| --- | --- | --- |
| `APP_ENV` | `development` | `development` enables verbose text logs; anything else logs JSON |
| `SERVER_PORT` | `8080` | Puerto HTTP en el que escucha la API |
| `SERVER_READ_TIMEOUT` and `SERVER_WRITE_TIMEOUT` | `10s` | Tiempos de espera del servidor HTTP |
| `SERVER_SHUTDOWN_TIMEOUT` | `15s` | Período de gracia para solicitudes en curso durante el apagado |
| `CORS_ALLOWED_ORIGIN` | `http://localhost:5173` | Lista separada por comas de orígenes autorizados para llamar a la API desde el navegador |
| `DB_NAME` | `corelog` | Nombre de la base de datos PostgreSQL |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE` | `localhost`, `5432`, `corelog`, `corelog`, `disable` | Conexión a PostgreSQL |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME` | `25`, `25`, `5m` | Configuración del pool de conexiones de PostgreSQL |
| `JWT_SECRET` | `change-me-in-production` | Secreto de firma HMAC para los tokens de acceso; establece un secreto real fuera del desarrollo local |
| `JWT_EXPIRATION` | `24h` | Duración del token de acceso |
| `TICKET_REPOSITORY` | `postgres` | `postgres` uses PostgreSQL, or `memory` forces the in-memory ticket adapter |

## Referencia de la API

El contrato completo se encuentra en [`openapi.yaml`](https://www.google.com/search?q=openapi.yaml&utm_source=gemini) at the repository root. La tabla siguiente es un resumen.

Ruta base: `/api/v1`. Los endpoints que requieren autenticación esperan `Authorization: Bearer <token>`.

| Método | Path | Autenticación | Descripción |
| --- | --- | --- | --- |
| `POST` | `/auth/register` | No requerida | Crea una cuenta de usuario. La primera cuenta creada se convierte automáticamente en administrador; todas las cuentas posteriores son solicitantes por defecto |
| `POST` | `/auth/login` | No requerida | Autentica al usuario y recibe un JWT |
| `GET` | `/users/me` | Requerida | Usuario autenticado actual |
| `PATCH` | `/users/me` | Requerida | Actualiza el nombre y correo electrónico del usuario actual |
| `POST` | `/users/me/password` | Requerida | Cambia la contraseña del usuario actual proporcionando la contraseña vigente |
| `GET` | `/users/search` | Requerida | Busca usuarios dentro del propio equipo del solicitante mediante `?q=`, for picking a ticket assignee |
| `GET` | `/users` | Solo administrador | Lista o busca todos los usuarios mediante `?q=` and `?team_id=` |
| `PATCH` | `/users/{id}/team` | Solo administrador | Asigna un usuario a un equipo o lo desvincula con `team_id: null` |
| `GET` | `/teams` | Requerida | Lista todos los equipos |
| `POST` | `/teams` | Solo administrador | Crea un equipo |
| `GET` | `/teams/{id}` | Requerida | Obtiene un equipo por ID |
| `PATCH` | `/teams/{id}` | Solo administrador | Cambia el nombre de un equipo |
| `DELETE` | `/teams/{id}` | Solo administrador | Elimina un equipo; sus miembros quedan sin equipo en lugar de ser eliminados |
| `POST` | `/tickets` | Requerida | Crea un ticket |
| `GET` | `/tickets` | Requerida | Lista tickets, filtrables mediante `?status=`, `?priority=`, `?page=`, `?page_size=` |
| `GET` | `/tickets/{id}` | Requerida | Obtiene un ticket por ID |
| `PATCH` | `/tickets/{id}/status` | Requerida | Cambia el estado de un ticket: `open → in_progress → resolved → closed`, with `resolved → in_progress` also allowed |
| `PATCH` | `/tickets/{id}/assign` | Requerida | Asigna un ticket a un usuario; el asignado debe pertenecer al mismo equipo que el solicitante; de lo contrario, devuelve `403` |
| `GET` | `/health` | No requerida | Liveness check, mounted at the root path without the `/api/v1` prefix |

Ticket `status`: `open`, `in_progress`, `resolved`, `closed`. Ticket `priority`: `low`, `medium`, `high`, `critical`. Admin-only endpoints require the caller's JWT to carry the `admin` role.

## Comandos disponibles

Backend, from the `Makefile` at the repo root:

| Comando | Descripción |
| --- | --- |
| `make run` | Ejecuta la API localmente mediante `go run ./cmd/api` |
| `make build` | Compila el binario de la API en `bin/api` |
| `make test` | Ejecuta las pruebas de Go |
| `make lint` | Run `go vet` |
| `make tidy` | Run `go mod tidy` |
| `make docker-up`, `make docker-down`, `make docker-logs` | Administra el stack de Docker Compose de PostgreSQL y la API |
| `make migrate-up`, `make migrate-down` | Aplica o revierte las migraciones de PostgreSQL |
| `make migrate-create name=<name>` | Genera el par de archivos de una nueva migración de PostgreSQL |
| `make ui-install`, `make ui-dev`, `make ui-build`, `make ui-lint`, `make ui-test` | Reenvía a los scripts npm del frontend (`web/`) |

Frontend, desde `web/package.json`, dentro de `web/`: `npm run start:portal`, `npm run build:libs`, `npm run build:all`, `npm run test:all`, `npm run format:check`, `npm run e2e`.

## Docker

`docker-compose.yaml` levanta cuatro servicios con un solo comando:

```sh
docker compose up -d --build

```

| Servicio | Qué hace |
|---|---|
| `db` | PostgreSQL 17 con su volumen de datos |
| `migrate` | Aplica las migraciones de `migrations/postgres/` cuando la base está lista y termina |
| `api` | API de Go; arranca solo después de que `migrate` terminó bien |
| `ui` | Contenedor nginx que sirve el build de producción del frontend en `http://localhost:5173` |

El frontend se compila con la configuración de entorno de Angular (`BUILD_CONFIGURATION`, por defecto `production` en `docker-compose.yaml` y `local` en `docker-compose.local.yaml`) y llama a la API en `/api/v1` a través de nginx, que la redirige a `API_UPSTREAM` (por defecto `http://api:8080`).

Para ejecutar todo el proyecto en local con una configuración y volúmenes independientes, usa `docker-compose.local.yaml`:

```sh
docker compose -f docker-compose.local.yaml up -d --build
```

La UI queda disponible en `http://localhost:5173`, la API en `http://localhost:8080` y PostgreSQL en `localhost:5432`. La primera cuenta registrada obtiene el rol de administrador. La base de datos y los adjuntos persisten en volúmenes locales dedicados. Para detener los servicios sin borrar esos datos:

```sh
docker compose -f docker-compose.local.yaml down
```

Los puertos se pueden cambiar con `CORELOG_LOCAL_UI_PORT`, `CORELOG_LOCAL_API_PORT` y `CORELOG_LOCAL_DB_PORT`. Las credenciales de desarrollo tienen valores predeterminados; define `DB_USER`, `DB_PASSWORD`, `DB_NAME` y `JWT_SECRET` en el entorno o en un archivo `.env` si necesitas personalizarlas.

### Contenedor de base de datos: usuario, base de datos y permisos

The `db` service runs the official `postgres:17-alpine` image unmodified. That image creates the role, the database, and the ownership grant between them automatically, driven entirely by three environment variables set in `docker-compose.yaml`:

| Variable en el contenedor | Sourced from `.env` | Efecto |
| --- | --- | --- |
| `POSTGRES_USER` | `DB_USER`, default `corelog` | Nombre del rol de PostgreSQL que la imagen crea en el primer inicio. A este rol se le concede el privilegio de superusuario dentro del contenedor. |
| `POSTGRES_PASSWORD` | `DB_PASSWORD`, default `corelog` | Contraseña establecida para ese rol. |
| `POSTGRES_DB` | `DB_NAME`, default `corelog` | Name of the database the image creates, owned by `POSTGRES_USER`. |

Como el rol es propietario de la base de datos que crea, ya tiene permisos completos de lectura y escritura. No se requiere manualmente `CREATE ROLE`, `CREATE DATABASE`, or `GRANT` para la configuración predeterminada descrita en este proyecto.

This automatic setup runs only once: the first time the container starts against an empty `corelog_db_data` volume. Changing `DB_USER`, `DB_PASSWORD`, or `DB_NAME` in `.env` after that volume already holds data has no effect on the existing role or database, since the image only runs its initialization scripts against an empty data directory. To apply new credentials from a clean state, remove the volume before starting the containers again:

```sh
docker compose down
docker volume rm corelog_corelog_db_data
docker compose up -d --build

```

El nombre del volumen es el nombre del proyecto de Compose, normalmente el nombre del directorio del repositorio `corelog`, seguido de un guion bajo y del nombre del volumen declarado en `docker-compose.yaml`, `corelog_db_data`. Run `docker volume ls` first to confirm the exact name on your machine, since a different directory name changes the project name prefix.

### Conectarse manualmente a la base de datos

Abre an interactive `psql` shell inside the running container:

```sh
docker compose exec db psql -U corelog -d corelog

```

Reemplaza `corelog` en ambos lugares por tus propios `DB_USER` and `DB_NAME` si los cambiaste respecto de los valores predeterminados. Desde fuera del contenedor, usando una instalación local de `psql` installation o un cliente gráfico como pgAdmin o TablePlus, conéctate con:

```sh
psql "host=localhost port=5432 user=corelog password=corelog dbname=corelog sslmode=disable"

```

Once connected, these `psql` meta-commands are useful for inspecting the setup:

```sql
\du
\l
\dt

```

`\du` lista los roles y sus atributos, `\l` lista las bases de datos y `\dt` lista las tablas de la base de datos actual.

### Crear manualmente un rol adicional

La configuración predeterminada proporciona a la aplicación un único rol con propiedad total de su base de datos, lo cual es suficiente para el desarrollo local y el alcance de este proyecto. Para crear un segundo rol, por ejemplo una cuenta de reportes de solo lectura, conéctate como se describió anteriormente y ejecuta:

```sql
CREATE ROLE reporting WITH LOGIN PASSWORD 'change-me';
GRANT CONNECT ON DATABASE corelog TO reporting;
GRANT USAGE ON SCHEMA public TO reporting;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO reporting;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO reporting;

```

La última instrucción garantiza que las tablas creadas por futuras migraciones también sean legibles por `reporting`, y no solo las tablas que ya existían cuando se ejecutó el permiso.

## Integración y entrega continua

Cada Pull Request hacia `main` ejecuta estos flujos de GitHub Actions:

| Flujo | Qué valida |
|---|---|
| CI (`ci.yml`) | Backend: formato con `gofmt`, `go build`, `go vet` y `go test` con reporte de cobertura. Frontend: `npm run build` |
| Code Quality | `golangci-lint` y ESLint |
| CodeQL | Análisis estático de Go y TypeScript |
| Dependency Review | Dependencias nuevas o modificadas |
| Secret Scan | `gitleaks` y `trufflehog` para detectar secretos |
| Security Scan | `govulncheck`, `nancy` y `npm audit` |
| Supply Chain | Integridad de dependencias y acciones fijadas por versión |
| OpenSSF Scorecard | Evaluación de buenas prácticas del repositorio (semanal) |

La rama `main` está protegida: exige Pull Request, aprobación de otro integrante y checks en verde.

### Releases

El flujo `release.yml` publica una Release con numeración `vX.Y.Z` cada vez que se fusiona un Pull Request que cambia código. Los cambios que solo tocan documentación (`*.md`, `docs/`), `.github/` o archivos de configuración no generan Release.


## Convenciones del proyecto

* No comments in source code. Names, small functions, and the hexagonal layering carry the intent; this file and `web/README.md` are where narrative documentation belongs.
* Ports before adapters. Every use case depends on an interface declared in `application/port`, never directly on `adapter/out/*`. The composition root is the only place concrete adapters are chosen: `cmd/api/main.go` on the backend, `provideCorelog` in `web/projects/core` on the frontend.
* Un adaptador, una responsabilidad de persistencia. `adapter/out/postgres` mapea sus filas al dominio sin almacenar datos fuera de PostgreSQL.
* Frontend imports use the workspace aliases (`@corelog/core`, `@corelog/ui/<name>`, `@corelog/features/<name>`); see `web/README.md`.

## Solución de problemas

* `migrate: command not found`: install [golang-migrate](https://github.com/golang-migrate/migrate).
* `relation "users" does not exist` con Docker: las migraciones no se aplicaron. Revisa `docker compose logs migrate`.
* Errores de CORS en el navegador: asegúrate de que `CORS_ALLOWED_ORIGIN` coincida con el origen exacto en el que se ejecuta el frontend, cuyo valor predeterminado es `http://localhost:5173`.
* `401 Unauthorized` on `/tickets`: the ticket and `/users/me` endpoints require `Authorization: Bearer <token>` from `POST /auth/login`.
