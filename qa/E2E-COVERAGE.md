# Cobertura E2E de pantallas

Mapa de cada pantalla de las tres aplicaciones web (portal, espacio de trabajo y administración) frente a las pruebas de Playwright que la cubren. Las pruebas viven en `web/e2e/specs/**` (funcionales por área) y `web/e2e/journeys/**` (recorridos). El escaneo de accesibilidad (`web/e2e/specs/a11y/screens.spec.ts`) se documenta en `qa/ACCESIBILIDAD.md`; aquí solo se marca qué pantallas recorre.

Los estados de las tablas se dedujeron de los nombres y el contenido de las pruebas; al ampliar una prueba, actualiza su fila.

## Cómo se ejecutan

Todas las pruebas, salvo las de autoevaluación, se repiten en seis proyectos de Playwright: `desktop`, `tablet` y `mobile` (1440, 820 y 390 píxeles) en tema claro y en tema oscuro (`desktop-dark`, `tablet-dark`, `mobile-dark`). Por eso la columna de "oscuro y móvil" distingue entre lo que solo se comprueba de forma genérica (auditoría de diseño del recorrido de pantallas) y lo que tiene una aserción propia.

Requisitos: API y base de datos arriba, escenario `e2e` sembrado y la contraseña de pruebas en un archivo local `web/.env.e2e` (nunca se versiona).

```sh
# API y base de datos (Docker), escenario de pruebas y servidor web nativo
docker compose -f docker-compose.local.yaml up -d db migrate api
SEED_PASSWORD='<contrasena>' go run ./cmd/seed --scenario e2e --force-reset   # solo en una base de pruebas
cd web
sh scripts/dev-host.sh                      # http://localhost:5173 (otra terminal)
sh scripts/e2e-local.sh                     # toda la suite, una ejecución a la vez
sh scripts/e2e-local.sh --project=desktop e2e/specs/auth   # un proyecto y una carpeta
```

Alternativa sin servidor de desarrollo: `make web-build && make web-serve` y luego `make e2e-native`. Las pruebas visuales (`npm run e2e:visual`, con `E2E_VISUAL=1`) comparan contra imágenes de referencia de Linux y se regeneran con `scripts/e2e-docker.sh`; en Windows se omiten.

Las pruebas `test.skip` actuales son condicionales, no cuarentenas: dependen de la variable `E2E_USER_PASSWORD` (login, registro, onboarding), del proyecto `desktop` (flujos de arrastre del tablero y backlog) o del ancho (calendario móvil). Hay una omisión incondicional: la invitación caducada en `specs/auth/invite.spec.ts`.

## Lista de pruebas inestables en cuarentena

Ninguna. Una prueba inestable no se deja en la suite: se corrige la causa (esperas por estado de la interfaz, nunca por tiempo) o se elimina.

## Portal (`/`)

| Ruta | Pruebas | Estados cubiertos | Huecos |
|---|---|---|---|
| `/login` | `specs/auth/login.spec.ts`, `specs/auth/two-factor.spec.ts`, a11y | Error de credenciales, 2FA, tema oscuro y móvil con aserciones propias | Bloqueo de cuenta por intentos (solo prueba de API) |
| `/register` | `specs/auth/register.spec.ts`, `register-height.spec.ts`, `register-legal-dialog.spec.ts`, `phone-country-overlay.spec.ts`, a11y | Validaciones, consentimientos legales, teléfono con país, altura del formulario | Alta con SMS real (no hay proveedor) |
| `/forgot-password`, `/reset-password` | `specs/auth/password-reset.spec.ts`, a11y | Token inválido y caducado, política de contraseña | - |
| `/verify-email` | `specs/auth/password-reset.spec.ts`, `specs/notifications/verify-banner.spec.ts`, a11y | Token inválido, aviso de correo sin verificar | - |
| `/invite/:token` | `specs/auth/invite.spec.ts`, a11y | Invitación válida, inválida | Invitación caducada (prueba omitida) |
| `/onboarding` | `specs/auth/onboarding.spec.ts`, journey `smoke-all-screens` (rol sin organización), a11y | Alta de la primera organización | - |
| `**` (no encontrada) | a11y | - | Sin aserción funcional propia |
| Compuerta de consentimiento legal | `specs/auth/consent-gate.spec.ts` | Documento nuevo pendiente de aceptar | - |

## Espacio de trabajo (`/workspace/`)

| Ruta | Pruebas | Estados cubiertos | Huecos |
|---|---|---|---|
| `/dashboard` | journey `smoke-all-screens`, a11y | Diseño en todos los roles y proyectos de viewport | Sin prueba funcional de "Mi resumen" |
| `/dashboards`, `/dashboards/:id` | `specs/dashboards/dashboards.spec.ts`, journey, a11y | Crear, editar y compartir paneles, gadgets | Estado de error de un gadget |
| `/issues`, `/issues/new`, `/browse/:key` | `specs/issues/create.spec.ts`, `view.spec.ts`, `comments.spec.ts`, `worklogs.spec.ts`, journey, a11y | Vacío (sin resultados), creación, edición, comentarios, tiempo registrado | Acciones masivas, vínculos y clonado |
| `/timesheet` | `specs/issues/worklogs.spec.ts`, a11y | Registro y totales | - |
| `/filters` y navegador de búsqueda (CoreQL) | `specs/search/navigator.spec.ts`, a11y | Consultas válidas y no válidas, guardar filtro | Suscripciones a filtros |
| `/projects` | `specs/projects/list.spec.ts`, journey, a11y | Vacío, archivados, favoritos, permisos por rol | - |
| `/projects/new` | `specs/projects/wizard.spec.ts`, `journeys/project-lifecycle.spec.ts`, a11y | Plantillas, validación de clave | - |
| `/projects/:key/board` | `specs/boards/board.spec.ts`, journey, a11y | Vacío, límite WIP, arrastre (solo `desktop`) | Arrastre táctil en móvil |
| `/projects/:key/backlog`, `/sprints`, `/sprints/:id/report` | `specs/boards/backlog.spec.ts`, journey, a11y | Planificar, iniciar y completar sprint | Informe del sprint con datos extremos |
| `/projects/:key/roadmap` | `specs/roadmap/roadmap.spec.ts`, journey, a11y | Vacío, dependencias, móvil | - |
| `/projects/:key/calendar`, `/calendar` | `specs/calendar/calendar.spec.ts`, `calendar-alignment.spec.ts`, journey, a11y | Eventos, alineación de la cuadrícula, vista de día en móvil | Eventos recurrentes y suscripción ICS |
| `/projects/:key/reports`, `/reports/:kind` | `specs/reports/reports.spec.ts`, a11y | Vacío, filtros, exportación | Informes con 20 000 incidencias (solo prueba de API) |
| `/projects/:key/settings/details`, `access`, `roles`, `permissions`, `danger` | `specs/projects/settings.spec.ts`, `journeys/project-lifecycle.spec.ts`, a11y | Permisos por rol, archivar, restaurar, eliminar con confirmación | Esquema de permisos personalizado de extremo a extremo |
| `.../settings/issue-types`, `versions`, `components`, `fields` | `specs/projects/versions.spec.ts`, `specs/org/catalogs.spec.ts`, a11y | Versiones y componentes: vacío y alta | Editor de campos del proyecto sin prueba propia |
| `.../settings/workflow` | `specs/workflow/editor.spec.ts`, `transitions.spec.ts`, a11y | Edición y transiciones | - |
| `.../settings/notifications` | `specs/notifications/scheme-editor.spec.ts`, a11y | Esquema propio del proyecto | - |
| `.../settings/automation` (lista, `new`, `:ruleId`) | `specs/integrations/automation.spec.ts`, a11y | Vacío, crear y editar con el constructor | Historial de ejecuciones |
| `.../settings/import`, `export` | `specs/integrations/import-export.spec.ts`, a11y | Asistente de importación, exportación CSV y JSON | Errores por fila con archivos grandes |
| `/teams`, `/teams/:id` | `specs/org/teams.spec.ts`, journey, a11y | Altas y permisos | - |
| `/notes` | `specs/notes/notes.spec.ts`, journey, a11y | Crear, buscar, vincular, borrar | - |
| `/people`, `/people/:id` | `specs/org/people.spec.ts`, journey, a11y | Listado y perfil | - |
| `/profile/general`, `security`, `privacy`, `appearance` | `specs/org/identity.spec.ts`, `specs/auth/two-factor.spec.ts`, `specs/shell/shell.spec.ts`, journey, a11y | Nombre, teléfono, 2FA, tema | Sesiones: cierre de una sesión ajena |
| `/profile/notifications` | `specs/notifications/preferences.spec.ts`, a11y | Canales y resumen | - |
| `/profile/tokens` | `specs/integrations/tokens.spec.ts`, a11y | Crear, revocar, mostrar revocados | - |
| Campana y alerta de nuevo inicio de sesión | `specs/notifications/live-stream.spec.ts` | Aviso en tiempo real, vacío, "No fui yo" | Reconexión tras cortar la red |
| Barra lateral, contexto de proyecto, fragmentos obsoletos | `specs/shell/shell.spec.ts`, `project-context.spec.ts`, `stale-chunk.spec.ts` | Tema oscuro, móvil, recuperación tras un despliegue | - |
| `**` (no encontrada) | a11y | - | Sin aserción funcional propia |

## Administración (`/admin/`)

| Ruta | Pruebas | Estados cubiertos | Huecos |
|---|---|---|---|
| `/organization/overview` | `specs/org/overview.spec.ts`, a11y | Vacío, tarjetas | - |
| `/organization/settings` | `specs/org/settings.spec.ts`, journey, a11y | Edición y permisos | - |
| `/organization/members`, `/invitations` | `specs/org/members.spec.ts`, `specs/org/invitations.spec.ts`, journey, a11y | Roles, invitar, revocar, vacío | - |
| `/organization/catalogs`, `/custom-fields`, `/statuses` | `specs/org/catalogs.spec.ts`, journey, a11y | Altas, vacío | Campos personalizados sin prueba propia |
| `/organization/legal` | `specs/org/legal-admin.spec.ts`, a11y | Publicar versión, consentimientos | - |
| `/organization/notifications` | `specs/notifications/scheme-editor.spec.ts`, a11y | Esquema de la organización | - |
| `/organization/audit` | `specs/audit/audit.spec.ts`, a11y | Filtros, vacío, exportación | - |
| `/organization/webhooks` | `specs/integrations/webhooks.spec.ts`, a11y | Crear, probar, entregas | Rotación del secreto |
| `/organization/platform-audit`, `email-outbox`, `jobs` | `specs/audit/audit.spec.ts`, `specs/org/platform-admin.spec.ts`, a11y | Solo administradores de la plataforma, `403` para el resto | Trabajos programados sin prueba funcional |
| `/403`, `**` | `specs/org/platform-admin.spec.ts`, a11y | Acceso denegado | - |

## Estados transversales

- **Permisos:** `fixtures/roles.ts` define los roles (propietario, administrador de organización, administrador de la plataforma, miembro, invitado, sin organización). El recorrido `smoke-all-screens` visita cada pantalla con cada rol y declara qué respuestas 403/404 son esperables.
- **Tema oscuro y móvil:** el recorrido y la auditoría de diseño (`lib/layout-audit.ts`) se ejecutan en los seis proyectos; aserciones específicas en `login.spec.ts`, `shell.spec.ts`, `backlog.spec.ts` y `roadmap.spec.ts`.
- **Carga:** no hay pruebas dedicadas de esqueletos; se cubren indirectamente al esperar el estado final de cada pantalla.
- **Error de red o 5xx:** cubierto solo en la recuperación de fragmentos obsoletos y en el aviso del flujo en tiempo real.

## Huecos explícitos (resumen)

1. Reconexión del flujo de notificaciones tras cortar la red y verificación detrás de un proxy real.
2. Invitación caducada (prueba omitida) y bloqueo de cuenta en la interfaz.
3. Acciones masivas de incidencias, vínculos, clonado y movimiento entre proyectos.
4. Eventos recurrentes y suscripción ICS del calendario.
5. Campos personalizados (organización y proyecto), rotación de secreto de webhooks y historial de ejecuciones de reglas.
6. Trabajos programados de la plataforma y cierre de sesiones ajenas.
7. Arrastre táctil en el tablero y el backlog (las pruebas de arrastre solo corren en `desktop`).
8. Pruebas de estado de carga y de error por pantalla.
