# QA Checklist - proyectos (autoservicio)

## Entorno

- Stack local: `docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml up -d --build`
- Datos: `SEED_PASSWORD=<contrasena> go run ./cmd/seed --scenario e2e --force-reset`
- Cuentas de prueba (organización Acme salvo que se indique): `owner@corelog.test` (propietario), `orgadmin@corelog.test` (administrador), `lead@corelog.test` (miembro, lead de SCRUM), `member@corelog.test` (miembro), `viewer@corelog.test` (miembro, equipo Design), `guest@corelog.test` (invitado), `newbie@corelog.test` (sin organización).
- Proyectos sembrados: SCRUM (abierto, plantilla Scrum, equipo Design como viewer), KAN (abierto, plantilla Kanban, equipo Platform como contributor), SEC (privado, Kanban), EMP (abierto, seguimiento de tareas, sin incidencias).
- Navegadores: Chromium y, si es posible, Firefox. Anchos de pantalla: 1440, 820 y 390 píxeles.
- Temas: claro y oscuro.

## Plantillas

- [ ] Crear un proyecto con la plantilla Scrum: tiene el tablero de sprints y el tablero predeterminado con columnas Por hacer, En curso, En revisión y Hecho
- [ ] Crear un proyecto con la plantilla Kanban: no ofrece sprints; mismas columnas que Scrum
- [ ] Crear un proyecto con la plantilla Seguimiento de tareas: columnas Por hacer, En curso y Hecho; tipos Tarea y Subtarea
- [ ] Crear un proyecto con la plantilla Seguimiento de errores: columnas Nuevo, Confirmado, En curso, Resuelto y Cerrado; tipos Error, Regresión, Incidencia y Mejora
- [ ] Cada plantilla muestra su nombre y descripción en el asistente de creación
- [ ] Color e icono elegidos en el asistente se muestran en la cabecera y en el menú de proyectos

## Claves

- [ ] Una clave de 2 a 10 caracteres, que empieza por letra y usa letras y números, se acepta
- [ ] La clave se muestra en mayúsculas aunque se escriba en minúsculas
- [ ] Una clave de 1 carácter, de más de 10 o que empieza por número se rechaza con mensaje de validación (422)
- [ ] Una clave ya usada en la misma organización se rechaza con conflicto (409, `project.key_taken`) y el formulario lo indica antes de enviar
- [ ] La misma clave sí puede usarse en otra organización
- [ ] Tras eliminar un proyecto, su clave queda libre y se puede volver a crear
- [ ] Se puede abrir el proyecto por su clave (por ejemplo `/projects/SCRUM`) y por su identificador

## Visibilidad

- [ ] Un proyecto privado (SEC) aparece para `orgadmin@corelog.test` y para el propietario
- [ ] Un miembro común (`member@corelog.test`) no ve SEC: ni en el listado, ni en la búsqueda, ni en favoritos
- [ ] Un miembro común que abre `/projects/SEC` o su API recibe 404 (`project.not_found`), no 403
- [ ] Un proyecto abierto (SCRUM) lo ve cualquier miembro de Acme como viewer
- [ ] Un invitado (`guest@corelog.test`) no ve un proyecto abierto en el que no tiene acceso explícito
- [ ] Al dar al invitado un rol directo en un proyecto abierto, lo ve y su rol se aplica
- [ ] Cambiar un proyecto de abierto a privado oculta el proyecto a los miembros que no tienen acceso explícito

## Permisos: cambios que alteran el comportamiento

- [ ] Como lead de KAN, concede `create_issue` al rol viewer en el esquema: `viewer@corelog.test` (o un viewer de KAN) puede crear incidencias
- [ ] Quitar `create_issue` a viewer otra vez devuelve el 403 al crear
- [ ] Como lead de KAN, revoca `create_issue` al contributor: un contributor (`member@corelog.test`) recibe 403 al crear incidencias y la interfaz oculta o deshabilita el botón
- [ ] Restaurar el permiso al contributor devuelve la capacidad de crear
- [ ] Un intento de quitar `edit_project` o `manage_members` al lead responde 422 (`authorization.invalid_scheme`)
- [ ] Un intento de dejar `browse_project` sin viewer, contributor ni lead responde 422
- [ ] "Restablecer" devuelve el esquema por defecto; la respuesta indica que es el predeterminado
- [ ] Un usuario sin permiso `edit_project` no puede guardar cambios en el esquema (403) y la pantalla lo refleja

## Equipos

- [ ] El equipo Design aparece con rol viewer en SCRUM en la pestaña de equipos del proyecto
- [ ] El equipo Platform aparece con rol contributor en KAN
- [ ] Un miembro de Platform que no está en KAN puede crear incidencias en KAN por el rol de equipo
- [ ] Un rol directo y un rol de equipo se combinan: se aplica el más amplio
- [ ] Quitar la concesión de un equipo elimina su acceso; los miembros con rol directo lo conservan
- [ ] Conceder un equipo de otra organización responde 422 (`project.team_not_in_org`)
- [ ] Solo quien puede gestionar miembros ve y cambia las concesiones de equipo

## Archivado

- [ ] El lead archiva SCRUM: aparece en el filtro de archivados y no en el listado normal
- [ ] Un proyecto archivado no permite editar nombre, descripción, visibilidad, plantilla ni configuración: responde 409 (`project.archived`) y la interfaz muestra el aviso
- [ ] Restaurar el proyecto lo devuelve al listado normal y vuelve a permitir cambios
- [ ] Archivar un proyecto ya archivado responde 409 (`project.already_archived`)
- [ ] Pendiente de implementación: crear o editar incidencias de un proyecto archivado debería responder 409. En esta versión no se bloquea; anotar el resultado real
- [ ] Pendiente de implementación: mover tarjetas de un tablero archivado debería responder 409. Anotar el resultado real

## Eliminación con nombre escrito

- [ ] El diálogo de eliminar exige escribir el nombre exacto del proyecto
- [ ] Un nombre distinto (incluso en mayúsculas o minúsculas diferentes) responde 422 (`project.confirmation_mismatch`) y no elimina
- [ ] Con el nombre exacto, el proyecto desaparece de listados, búsqueda, favoritos y recientes
- [ ] Un miembro común no puede eliminar un proyecto (403 o 404 según su visibilidad) y el nombre no se revela
- [ ] El propietario de la organización puede eliminar cualquier proyecto de la organización

## Favoritos y recientes

- [ ] Marcar SCRUM con la estrella lo añade a favoritos del menú de proyectos; quitar la estrella lo elimina
- [ ] `member@corelog.test` ya tiene SCRUM marcado y KAN en recientes al entrar por primera vez
- [ ] Abrir un proyecto lo sitúa al principio de recientes
- [ ] Un favorito de un proyecto privado al que pierdes el acceso deja de aparecer
- [ ] Un proyecto eliminado desaparece de favoritos y recientes

## Roles

Para cada rol, comprobar qué puede hacer en SCRUM o KAN y que la interfaz coincide con la API.

- [ ] Propietario (`owner@corelog.test`): todo en el proyecto, incluido eliminar; ve SEC
- [ ] Administrador de organización (`orgadmin@corelog.test`): actúa como lead en SEC y en SCRUM
- [ ] Lead (`lead@corelog.test`): edita proyecto, miembros, tablero, sprints y esquema; no ve cambios de otros proyectos sin acceso
- [ ] Contributor (`member@corelog.test`): crea y edita incidencias; no gestiona el tablero, los sprints ni los miembros
- [ ] Viewer (`viewer@corelog.test`): ve y consulta informes; no crea incidencias ni mueve tarjetas
- [ ] Invitado (`guest@corelog.test`): solo ve los proyectos en los que tiene acceso explícito; no crea proyectos
- [ ] Sin organización (`newbie@corelog.test`): no ve ningún proyecto de Acme ni de CoreLog y recibe 404

## Diseño responsivo y tema

Repetir para claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Asistente de creación: pasos y botones visibles, sin desbordamiento horizontal
- [ ] Tarjetas de proyecto y menú de proyectos: texto legible, sin solapes
- [ ] Ajustes del proyecto y esquema de permisos: tablas o listas navegables sin scroll horizontal oculto
- [ ] Aviso de proyecto archivado y diálogo de eliminación: contraste suficiente y foco visible
- [ ] Contraste de los colores de proyecto (palette) suficiente en tema oscuro
- [ ] Iconos de plantilla visibles en ambos temas

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa con `CORELOG_TEST_DATABASE_URL`
- [ ] `go run ./cmd/coverage-check -min 50` pasa
- [ ] Playwright: journey `project-lifecycle` pasa en el entorno compose
