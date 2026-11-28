<p align="center">
  <img alt="CoreLog" src="docs/assets/corelog-banner.png" width="640">
</p>

<h1 align="center">CoreLog - Plataforma de gestión de trabajo</h1>

<p align="center">
  Organizaciones, equipos, proyectos, incidencias, flujos de trabajo, tableros, sprints, búsqueda, informes,
  automatización e integraciones en una sola aplicación.
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

CoreLog es una plataforma de gestión de trabajo en la línea de Jira: organiza personas, proyectos e incidencias con flujos configurables, tableros kanban y scrum, búsqueda avanzada, informes y automatización. Se desarrolla con fines educativos en el curso "Herramientas de Desarrollo" de la Universidad Tecnológica del Perú.

## Tabla de contenidos

- [Funcionalidades](#funcionalidades)
- [Arquitectura](#arquitectura)
- [Prerequisitos](#prerequisitos)
- [Inicio rápido](#inicio-rápido)
- [Datos de prueba (seed)](#datos-de-prueba-seed)
- [Scripts y comandos](#scripts-y-comandos)
- [Variables de entorno](#variables-de-entorno)
- [Pruebas](#pruebas)
- [Integración y entrega continua](#integración-y-entrega-continua)
- [Despliegue](#despliegue)
- [Solución de problemas](#solución-de-problemas)
- [Documentación](#documentación)
- [Convenciones del proyecto](#convenciones-del-proyecto)
- [Integrantes](#integrantes)

## Funcionalidades

| Área | Qué incluye |
|---|---|
| Organizaciones y equipos | Organizaciones con miembros e invitaciones, roles, equipos con acceso a proyectos, catálogos y campos personalizados |
| Proyectos | Plantillas Scrum, Kanban, tareas y errores; clave única, visibilidad, roles y permisos por proyecto, archivar, favoritos, versiones y componentes |
| Incidencias | Jerarquía épica, historia y subtarea, vínculos, comentarios, adjuntos, seguidores, registro de tiempo, acciones masivas y exportación |
| Flujos de trabajo | Estados y transiciones por proyecto con editor visual, condiciones y acciones posteriores |
| Tableros, backlog, sprints y lanzamientos | Tablero con límites WIP, backlog ordenable, ciclo de sprint con informe y burndown, versiones |
| Búsqueda | Lenguaje de consulta CoreQL, filtros guardados y suscripciones |
| Notas | Notas personales vinculables a incidencias |
| Tiempo e informes | Hojas de horas e informes por proyecto |
| Paneles, hoja de ruta y calendario | Paneles con gadgets, hoja de ruta, calendario con eventos recurrentes y suscripción ICS |
| Notificaciones | Esquemas por organización y proyecto, preferencias y resumen por correo, flujo en tiempo real (SSE) y alerta de seguridad por nuevo inicio de sesión ("No fui yo") |
| Automatización e integraciones | Reglas con constructor visual, webhooks firmados, tokens de API, asistente de importación CSV y exportación CSV y JSON |
| Auditoría y seguridad | Verificación en dos pasos, sesiones revocables, bloqueo por intentos y límites de frecuencia, consentimientos legales en el registro, teléfono con código de país, auditoría exportable y borrado lógico |

La guía de uso de cada función está en [`docs/guia-usuario.md`](docs/guia-usuario.md). La lista de cambios, en [`CHANGELOG.md`](CHANGELOG.md).

## Arquitectura

Backend y frontend siguen una arquitectura hexagonal (puertos y adaptadores): el dominio y los casos de uso no conocen HTTP, SQL ni Angular, y los adaptadores se conectan mediante interfaces explícitas.

```mermaid
flowchart LR
  subgraph Web["web/ (Angular 21)"]
    P["portal /"]
    W["workspace /workspace/"]
    A["admin /admin/"]
    L["libs: core, ui, features"]
    P --- L
    W --- L
    A --- L
  end
  N["nginx (imagen ui)"]
  subgraph API["API Go (cmd/api)"]
    H["adapter/in/http (chi)"]
    U["application/usecase"]
    D["domain"]
    R["adapter/out/postgres (sqlx)"]
    H --> U --> D
    U --> R
  end
  DB[("PostgreSQL 17")]
  O["outbox, eventos y trabajos"]
  Web --> N -->|"/api/v1"| H
  R --> DB
  U --> O
  O --> DB
  O -->|"SSE, correo, webhooks"| Web
```

Estructura del repositorio:

```
corelog/
├── cmd/
│   ├── api/                  punto de composición de la API y servidor HTTP
│   ├── seed/                 siembra de datos deterministas
│   └── coverage-check/       verifica el umbral de cobertura
├── internal/
│   ├── platform/             infraestructura transversal: configuración, base de datos, eventos (outbox), trabajos,
│   │                         correo, flujo SSE, límites de frecuencia, JWT, middleware
│   └── <módulo>/             un hexágono por área: user, organization, team, project, issue, workflow, board (sprint,
│                             release), search, note, report, dashboard, roadmap, calendar, notification, automation,
│                             webhook, apitoken, importexport, audit, legal, authorization, worklog, favorite
│       ├── domain/               entidades y reglas, sin dependencias externas
│       ├── application/          port (interfaces) y usecase (orquestación y autorización)
│       ├── adapter/in/http/      handlers, DTOs y rutas
│       ├── adapter/out/postgres/ repositorios sqlx con SQL explícito
│       └── module.go             conecta el hexágono
├── migrations/postgres/      migraciones SQL versionadas (golang-migrate)
├── web/                      workspace Angular: apps portal, workspace y admin; libs core, ui y features
├── docker/                   Dockerfile de la API
├── deploy/                   compose de producción
├── docs/                     despliegue, guía de usuario, estándares de la API
├── qa/                       listas de verificación y cobertura E2E
└── openapi.yaml              contrato de la API (OpenAPI 3)
```

Los módulos no se importan entre sí: cada uno declara los puertos que necesita y `cmd/api` conecta los adaptadores. Los eventos de dominio se publican en una tabla *outbox* dentro de la misma transacción; trabajos en segundo plano (correo, resumen, suscripciones, depuración) y el flujo SSE los consumen. Detalles del frontend en [`web/README.md`](web/README.md).

Flujo de una solicitud, por ejemplo crear una incidencia: `adapter/in/http` -> puerto de entrada -> `usecase` (autorización y orquestación) -> `domain` -> puerto de salida -> `adapter/out/postgres`.

## Prerequisitos

- Go 1.27 o superior
- Node.js 24 y npm
- Docker y Docker Compose v2
- CLI de [golang-migrate](https://github.com/golang-migrate/migrate), solo para ejecutar migraciones a mano (`make migrate-up`)

## Inicio rápido

### Con Docker Compose (recomendado)

Levanta base de datos, migraciones, API y la interfaz con una configuración local independiente:

```sh
docker compose -f docker-compose.local.yaml up -d --build
```

- Interfaz: `http://localhost:5173` · API: `http://localhost:8080` · PostgreSQL: `localhost:5432`.
- La primera cuenta registrada obtiene el rol de administrador de la plataforma.
- Los puertos se cambian con `CORELOG_LOCAL_UI_PORT`, `CORELOG_LOCAL_API_PORT` y `CORELOG_LOCAL_DB_PORT`; las credenciales de desarrollo tienen valores por defecto y se personalizan con `DB_USER`, `DB_PASSWORD`, `DB_NAME` y `JWT_SECRET`.
- Para detener sin perder datos: `docker compose -f docker-compose.local.yaml down` (nunca uses `-v` si quieres conservar la base).

Con recarga en caliente del frontend (contenedor de desarrollo de Angular), o con el atajo del Makefile:

```sh
docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml up -d --build
make stack-up
```

`docker-compose.yaml` levanta el mismo conjunto (`db`, `migrate`, `api`, `ui`) con la configuración por defecto (`make docker-up`).

### Frontend nativo y API en Docker

El flujo de desarrollo habitual ejecuta solo la API y la base en Docker, y el frontend directamente en tu máquina:

```sh
cp .env.example .env
docker compose -f docker-compose.local.yaml up -d db migrate api
cd web
npm ci --ignore-scripts
sh scripts/dev-host.sh            # servidor de desarrollo en http://localhost:5173
```

Alternativa de una sola aplicación: `npm run start:portal` (también `start:workspace` y `start:admin`).

### Verificar

```sh
curl http://localhost:8080/health
```

Devuelve `{"status":"ok"}` cuando la API es accesible. Abre la interfaz, registra una cuenta, crea una organización y un proyecto.

## Datos de prueba (seed)

`cmd/seed` siembra la base con datos deterministas: el mismo escenario produce siempre los mismos datos. La contraseña de todas las cuentas viene de la variable `SEED_PASSWORD`, obligatoria, sin valor por defecto y nunca impresa.

```sh
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario demo
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario e2e
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario scale --issues 50000
```

```powershell
$env:SEED_PASSWORD = '<contrasena>'; .\scripts\seed.ps1 --scenario demo
```

`scripts/seed.sh` y `scripts/seed.ps1` exigen `SEED_PASSWORD` y reenvían el resto de argumentos a `cmd/seed`. También existe `make seed`.

| Escenario | Contenido |
|---|---|
| `demo` | 21 cuentas (`admin@corelog.local`, `agent@corelog.local`, `requester@corelog.local` y personas `@corelog.test`), 5 equipos, 4 proyectos (CORE, OPS, SEC, WEB), 120 incidencias con comentarios, seguidores, enlaces, sprints, filtros guardados y notificaciones |
| `e2e` | Cuentas y organizaciones de las pruebas de Playwright, equipos, proyectos (SCRUM con sprint activo, KAN, SEC privado y EMP vacío) e incidencias de ejemplo |
| `scale` | Organización de `demo` y `--issues N` incidencias (50 000 por defecto) para pruebas de rendimiento |

Opciones: `--scenario` (`demo` por defecto, `e2e` o `scale`), `--issues N` (solo `scale`) y `--force-reset`.

> **Advertencia sobre `--force-reset`.** Vacía todas las tablas de la aplicación (nunca `schema_migrations`) antes de sembrar. Sin esa opción, `cmd/seed` se niega a sembrar una base que ya tiene usuarios. Además, `--force-reset` se **niega** a ejecutarse si la base contiene cuentas reales (correos fuera de `@corelog.test` y `@corelog.local`), para no borrar el trabajo de una persona por accidente. Solo se salta esa protección con `SEED_ALLOW_DATA_LOSS=yes`: úsalo únicamente en una base desechable, tras hacer un volcado, y nunca contra producción.

Las pruebas de la siembra necesitan PostgreSQL: con `CORELOG_TEST_DATABASE_URL` definida usan esquemas aislados; sin ella se omiten.

## Scripts y comandos

Backend, desde el `Makefile` de la raíz:

| Comando | Descripción |
|---|---|
| `make run` / `make build` | Ejecuta la API con `go run ./cmd/api` / compila `bin/api` |
| `make test` | Pruebas de Go (`./cmd/... ./internal/...`) |
| `make test-race`, `make test-coverage` | Con detector de carreras y `-p 2` / con cobertura y umbral del 70 % |
| `make authz-audit`, `make perf-check` | Auditoría de autorización de rutas y rendimiento con 20 000 incidencias (necesitan `CORELOG_TEST_DATABASE_URL`) |
| `make lint`, `make tidy` | `go vet` / `go mod tidy` |
| `make seed` | Siembra de datos (ver arriba) |
| `make stack-up`, `make stack-down` | Stack local con recarga en caliente |
| `make docker-up`, `make docker-down`, `make docker-logs` | Stack de `docker-compose.yaml` |
| `make migrate-up`, `make migrate-down`, `make migrate-create name=<nombre>` | Migraciones con golang-migrate |

Frontend (`make web-*` o directamente en `web/`):

| Comando | Descripción |
|---|---|
| `make web-install` | `npm ci --ignore-scripts` |
| `make web-build` | `npm run build:libs` y `npm run build:all:local` |
| `make web-test` | `npm run test:all` |
| `make web-format` | `npm run format:check` (Prettier) |
| `make web-serve` | Sirve `web/dist` como nginx y redirige `/api` |
| `make e2e-install`, `make e2e-native`, `make e2e` | Instala Chromium / ejecuta Playwright contra `web-serve` / `npm run e2e` |

`scripts/setup.sh` y `scripts/setup.ps1` envuelven la preparación del entorno (`check`, `env`, `dev`, `prod`, `build`, `package`, `clean`); ejecútalos sin argumentos para ver la ayuda.

## Variables de entorno

Todas viven en `.env` (copia de `.env.example`) o en el entorno del proceso. Las principales:

| Variable | Predeterminado | Descripción |
|---|---|---|
| `APP_ENV` | `development` | Con `production` se exigen las claves de cifrado y se ignoran `RATE_LIMIT_DISABLED` y `WEBHOOK_ALLOW_PRIVATE` |
| `SERVER_PORT` | `8080` | Puerto HTTP de la API |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | `localhost`, `5432`, `corelog`, `corelog`, `corelog`, `disable` | Conexión a PostgreSQL |
| `JWT_SECRET`, `JWT_EXPIRATION` | `change-me-in-production`, `24h` | Firma y vigencia de la sesión |
| `TWO_FACTOR_ENCRYPTION_KEY`, `APP_ENCRYPTION_KEY` | vacío | Claves AES de 32 bytes en base64 (`openssl rand -base64 32`) |
| `CORS_ALLOWED_ORIGINS` | vacío | Orígenes permitidos (la variable anterior `CORS_ALLOWED_ORIGIN` sigue funcionando) |
| `SMTP_*`, `APP_PUBLIC_URL`, `PUBLIC_URL` | ver `.env.example` | Correo saliente y enlaces públicos |
| `TRUSTED_PROXIES`, `RATE_LIMIT_*`, `LOCKOUT_*` | ver `.env.example` | Proxies de confianza, límites de frecuencia y bloqueo de cuentas |
| `ATTACHMENTS_DIR`, `IMPORTS_DIR`, `AVATARS_DIR`, `LOGOS_DIR` | `data/...` | Carpetas de archivos |
| `JOBS_ENABLED`, `PURGE_ENABLED`, `PURGE_DRY_RUN`, `AUDIT_RETENTION_DAYS` | ver `.env.example` | Tareas en segundo plano y retención |

La tabla completa con el valor recomendado para producción está en [`docs/despliegue.md`](docs/despliegue.md).

## Pruebas

```sh
gofmt -l .                              # debe quedar vacío
go build ./...
go vet ./cmd/... ./internal/...
go test -p 2 ./cmd/... ./internal/...   # no uses ./... (recorre node_modules)
```

- **Base de datos real:** con la base arriba, define `CORELOG_TEST_DATABASE_URL=postgres://corelog:corelog@localhost:5432/corelog?sslmode=disable`; las pruebas crean y eliminan esquemas aislados. Sin la variable se omiten.
- **Cobertura:** `make test-coverage` exige al menos 70 %.
- **Frontend unitarias (Vitest):** en `web/`, `npm run build:libs` y `npm run test:all`; formato con `npx prettier --check .`.
- **Playwright (nativo, sin Docker para el web):** con la API arriba, el escenario `e2e` sembrado y el servidor en `http://localhost:5173`, ejecuta `sh scripts/e2e-local.sh` desde `web/` (una ejecución a la vez). Cubre tres tamaños de pantalla en tema claro y oscuro. El mapa de pantallas y huecos está en [`qa/E2E-COVERAGE.md`](qa/E2E-COVERAGE.md).
- **Accesibilidad:** `web/e2e/specs/a11y/screens.spec.ts` ejecuta axe sobre las pantallas principales; ver [`qa/ACCESIBILIDAD.md`](qa/ACCESIBILIDAD.md).
- **Listas de QA manuales:** carpeta [`qa/`](qa/).

## Integración y entrega continua

Cada Pull Request hacia `main` ejecuta estos flujos de GitHub Actions:

| Flujo | Qué valida |
|---|---|
| CI (`ci.yml`) | Backend: `gofmt`, `go build`, `go vet`, `go test -race -p 2` con cobertura mínima del 70 %. Frontend: Prettier, bibliotecas, pruebas y compilación de las tres aplicaciones |
| E2E (`e2e.yml`) | Base de datos, migraciones y API con Docker, escenario `e2e` sembrado y suite de Playwright |
| Code Quality | `golangci-lint` y ESLint |
| CodeQL | Análisis estático de Go y TypeScript |
| Dependency Review | Dependencias nuevas o modificadas |
| Secret Scan | `gitleaks` y `trufflehog` |
| Security Scan | `govulncheck`, `nancy` y `npm audit` |
| Supply Chain | Integridad de dependencias y acciones fijadas por versión |
| OpenSSF Scorecard | Buenas prácticas del repositorio (semanal) |

La rama `main` está protegida: exige Pull Request, aprobación y checks en verde. `release.yml` publica una Release `vX.Y.Z` al fusionar cambios de código (los cambios solo de documentación o configuración no la generan). `deploy.yml` construye y publica las imágenes en GHCR y despliega al VPS.

## Despliegue

El despliegue de producción (compose, variables, actualización desde la versión anterior, proxy inverso con SSE, TLS, backups) está en [`docs/despliegue.md`](docs/despliegue.md) y los archivos [`deploy/docker-compose.prod.yml`](deploy/docker-compose.prod.yml) y [`docker/Dockerfile.api`](docker/Dockerfile.api). La preparación del servidor está en [`docs/vps/`](docs/vps/README.md).

## Solución de problemas

- **Pantalla en blanco o error al navegar tras un despliegue (fragmentos obsoletos):** el navegador conserva una versión anterior de la aplicación. La interfaz se recarga sola al detectarlo; si no, recarga con `Ctrl+Shift+R`.
- **Puerto ocupado (5173, 8080 o 5432):** cambia `CORELOG_LOCAL_UI_PORT`, `CORELOG_LOCAL_API_PORT` o `CORELOG_LOCAL_DB_PORT`, o detén el proceso que lo usa (`docker ps` para ver los contenedores).
- **`migrate: command not found`:** instala golang-migrate o usa el servicio `migrate` del compose.
- **`relation "users" does not exist` con Docker:** las migraciones no se aplicaron; revisa `docker compose logs migrate`.
- **Migración atascada (`Dirty database version N`):** una migración falló a medias. Restaura un respaldo o corrige el estado con `migrate ... force <versión>` solo tras revisar qué quedó aplicado.
- **Errores de CORS:** `CORS_ALLOWED_ORIGINS` debe coincidir con el origen exacto del frontend (por defecto `http://localhost:5173`).
- **`401 Unauthorized`:** las rutas protegidas exigen `Authorization: Bearer <token>` de `POST /api/v1/auth/login`, o un token de API personal.
- **Las notificaciones en tiempo real tardan o se cortan detrás de un proxy:** desactiva el búfer y sube el plazo de lectura para `/api/v1/notifications/stream` (ver `docs/despliegue.md`).
- **`seed` se niega a ejecutar:** la base ya tiene usuarios o cuentas reales; lee la advertencia de `--force-reset` más arriba.
- **Cuentas bloqueadas durante pruebas manuales:** el bloqueo por intentos fallidos se desactiva en local con `RATE_LIMIT_DISABLED=true` (el compose local ya lo hace; se ignora en producción).

## Documentación

| Documento | Contenido |
|---|---|
| [`docs/guia-usuario.md`](docs/guia-usuario.md) | Guía de usuario de todas las funciones |
| [`docs/despliegue.md`](docs/despliegue.md) | Despliegue, variables de entorno, actualización y operación |
| [`docs/api-standards.md`](docs/api-standards.md) | Convenciones de la API (formato, errores, paginación, autenticación) |
| [`openapi.yaml`](openapi.yaml) | Contrato completo de la API |
| [`web/README.md`](web/README.md) | Estructura y comandos del frontend |
| [`docs/vps/`](docs/vps/README.md) | Preparación y endurecimiento del servidor |
| [`qa/`](qa/) | Listas de verificación, cobertura E2E y accesibilidad |
| [`CHANGELOG.md`](CHANGELOG.md) | Cambios por versión |

## Convenciones del proyecto

- Puertos antes que adaptadores: cada caso de uso depende de una interfaz de `application/port`, nunca de `adapter/out/*`. La raíz de composición es el único lugar donde se eligen los adaptadores concretos (`cmd/api` en el backend, `provideCorelog` en `web/projects/core` en el frontend).
- Un adaptador, una responsabilidad de persistencia: `adapter/out/postgres` mapea filas al dominio.
- La autorización se decide en los casos de uso; el middleware solo autentica.
- Frontend: cada componente tiene su plantilla `.html` y sus estilos `.scss` en archivos propios, con BEM y los tokens `--cl-*`; los imports usan los alias del workspace (`@corelog/core`, `@corelog/ui/<nombre>`, `@corelog/features/<nombre>`).
- Mensajes de la interfaz, errores de la API y documentación de producto en español; código y mensajes de commit en inglés.

## Integrantes

Grupo 11, curso Herramientas de Desarrollo (UTP).

| Integrante | Rol en el equipo |
|---|---|
| Fernandez Gutierrez, Jhon Samuel | Responsable de repositorio |
| Vacas Neyra, Elton Smith | Responsable funcional |
| Navarro López, Jonathan Fabrizio | Responsable de calidad / automatización |
| Pariachi Herrera, Dalia Sahomi | Responsable de despliegue / documentación |
| Correa Ángeles, Daleth | Tablero de actividades y mejoras de frontend |
