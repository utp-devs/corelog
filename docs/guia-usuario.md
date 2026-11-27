# Guía de usuario de CoreLog

Esta guía describe cómo usar CoreLog según lo que puedes hacer en cada proyecto. Las reglas de permisos son las que aplica el servidor; la interfaz oculta o deshabilita lo que no puedes hacer, pero la decisión final siempre la toma la API.

## Roles

Hay un rol global y tres roles por proyecto.

- **Administrador global:** puede hacer todo en todos los proyectos.
- **Lead del proyecto:** gestiona el proyecto, sus miembros, el tablero, el flujo de estados y los sprints.
- **Contributor:** trabaja las incidencias del proyecto: las crea, las edita, las mueve por el tablero y comenta.
- **Viewer:** ve el proyecto y sus incidencias, y puede seguir incidencias. No puede modificar nada.

Quien no es miembro de un proyecto no lo ve: ni en el listado, ni en la búsqueda, ni en el calendario.

## Matriz de permisos

| Acción | Admin | Lead | Contributor | Viewer | Sin acceso |
|---|:-:|:-:|:-:|:-:|:-:|
| Ver proyecto e incidencias | Sí | Sí | Sí | Sí | No |
| Editar datos del proyecto | Sí | Sí | No | No | No |
| Eliminar proyecto | Sí | Sí | No | No | No |
| Gestionar miembros y roles | Sí | Sí | No | No | No |
| Crear y editar incidencias | Sí | Sí | Sí | No | No |
| Comentar y adjuntar | Sí | Sí | Sí | No | No |
| Seguir una incidencia | Sí | Sí | Sí | Sí | No |
| Mover tarjetas y ordenar el backlog | Sí | Sí | Sí | No | No |
| Gestionar tablero y límites WIP | Sí | Sí | No | No | No |
| Gestionar flujo de estados | Sí | Sí | No | No | No |
| Planificar, iniciar y cerrar sprints | Sí | Sí | No | No | No |

Los propietarios y administradores de la organización actúan como lead en todos sus proyectos, incluidos los privados. La matriz describe el esquema de permisos predeterminado; cada proyecto puede tener su propio esquema (ver "Proyectos").

## Navegación

- **Incidencias:** listado con filtros y búsqueda guardada, y detalle de cada incidencia con comentarios, adjuntos, enlaces, etiquetas, historial y seguimiento.
- **Proyectos:** listado de tus proyectos. Desde cada proyecto accedes a su tablero, backlog, sprints y calendario.
- **Tablero:** columnas por estado. Arrastra una tarjeta a otra columna para cambiar el estado de la incidencia. Una columna con límite WIP lleno no acepta más tarjetas. Si eres lead, puedes fijar el límite de cada columna en su cabecera.
- **Backlog:** todas las incidencias del proyecto en orden de prioridad. Usa las flechas para subir o bajar una incidencia.
- **Sprints:** planifica un sprint con nombre, objetivo y fechas. Solo puede haber un sprint activo por proyecto. Al cerrar un sprint, las incidencias no terminadas pasan al backlog o a otro sprint planificado. El gráfico de burndown muestra el trabajo restante frente a la línea ideal, y necesita fechas de inicio y fin.
- **Calendario:** inicios y fines de sprint, y fechas límite de incidencias del mes. Puedes verlo en cuadrícula o en lista.
- **Mi resumen:** tus incidencias abiertas, las vencidas, tus proyectos, los sprints activos de esos proyectos y las incidencias más urgentes.
- **Búsqueda:** en la cabecera, busca proyectos (por nombre o clave) e incidencias (por título o descripción). Solo aparece lo que puedes ver. Escribe al menos dos caracteres.
- **Notificaciones:** la campana de la cabecera muestra cuántos avisos tienes sin leer. Se actualiza cada 30 segundos. Recibes un aviso cuando te asignan una incidencia.

## Proyectos

### Crear un proyecto

Quien crea un proyecto queda como su lead. Pueden crear proyectos los propietarios y administradores de la organización, y los miembros cuando la política de la organización lo permite. Los invitados no pueden crear proyectos.

Al crear el proyecto indicas:

- **Nombre** (hasta 120 caracteres) y **descripción** opcional.
- **Clave**: de 2 a 10 caracteres, empieza por una letra y usa solo letras y números. Se guarda en mayúsculas (por ejemplo `CORE`). Puedes usar la clave en lugar del identificador en las direcciones de la API.
- **Plantilla** (ver la tabla siguiente).
- **Visibilidad**: abierta o privada.
- **Color** e **icono** para reconocer el proyecto en la interfaz.

La clave debe ser única dentro de la organización entre los proyectos no eliminados. Si está ocupada, el formulario lo indica antes de crear. Al eliminar un proyecto su clave queda libre.

### Plantillas

| Plantilla | Tipo | Sprints | Columnas del tablero | Tipos de incidencia |
|---|---|:-:|---|---|
| Scrum | Scrum | Sí | Por hacer, En curso, En revisión, Hecho | Épica, Historia, Tarea, Error, Subtarea |
| Kanban | Kanban | No | Por hacer, En curso, En revisión, Hecho | Épica, Historia, Tarea, Error, Subtarea |
| Seguimiento de tareas | Kanban | No | Por hacer, En curso, Hecho | Tarea, Subtarea |
| Seguimiento de errores | Kanban | No | Nuevo, Confirmado, En curso, Resuelto, Cerrado | Error, Regresión, Incidencia, Mejora |

La plantilla define el tablero predeterminado, los estados del flujo y los tipos de incidencia. Solo Scrum tiene backlog de sprints. Puedes cambiar un proyecto Scrum a Kanban y al revés. Un proyecto de seguimiento de tareas o de errores no puede pasar a Scrum.

### Visibilidad

- **Abierta:** todos los miembros de la organización ven el proyecto y sus incidencias. Los invitados solo lo ven si tienen acceso explícito (miembro del proyecto o de un equipo con acceso).
- **Privada:** lo ven los propietarios y administradores de la organización, los miembros del proyecto y los miembros de los equipos con acceso. Para cualquier otra persona el proyecto no existe: la API responde 404 y no aparece en listados, búsquedas ni favoritos.

La visibilidad predeterminada de los proyectos nuevos la define la organización.

### Roles en el proyecto

Cada persona con acceso tiene un rol en el proyecto: **lead**, **contributor** o **viewer**.

- **Lead:** gestiona el proyecto: datos, miembros y roles, tablero, flujo de estados, sprints, componentes, borrado de incidencias, archivado y eliminación. Un proyecto siempre conserva al menos un lead.
- **Contributor:** crea, edita y asigna incidencias, las mueve por el tablero, comenta, adjunta archivos, enlaza incidencias, registra horas y usa los cambios masivos. No gestiona el tablero, los sprints, los componentes ni los miembros.
- **Viewer:** ve el proyecto, sigue y vota incidencias, consulta informes y exporta. No modifica nada. Solo puede cambiar de estado una incidencia de la que es responsable.

Un usuario puede tener un rol directo y, además, heredar el de un equipo. Se aplica el más amplio.

Los invitados de una organización no reciben rol por pertenecer a ella: necesitan un rol directo o a través de un equipo. Un miembro de la organización ve los proyectos abiertos como viewer aunque no esté en el proyecto.

### Acceso de equipos

El lead puede dar a un equipo de la organización un rol en el proyecto (lead, contributor o viewer). Ese rol se aplica a todos los miembros del equipo, incluidos los que se incorporen después. Solo se pueden dar equipos de la misma organización. Para quitar el acceso, elimina la concesión del equipo; los miembros que tengan un rol directo lo conservan.

### Esquema de permisos

Cada proyecto puede ajustar qué puede hacer cada rol. El esquema predeterminado es el de la matriz de arriba. El lead edita el esquema por permiso: elige qué titulares lo tienen (lead, contributor, viewer, org_admin, reporter, assignee, watcher). Los cambios se aplican desde la siguiente petición.

Reglas que el servidor siempre comprueba:

- El rol lead debe conservar `manage_members` y `edit_project`.
- `delete_project` debe seguir asignado al lead o al org_admin.
- `browse_project` debe incluir a viewer, contributor o lead, para que alguien pueda ver el proyecto.

Ejemplo: si concedes "crear incidencias" al viewer, un viewer puede crear incidencias en ese proyecto. Si quitas "crear incidencias" al contributor, deja de poder crearlas, aunque siga siendo miembro. Para volver al esquema predeterminado, usa "Restablecer": se borran todas las reglas propias del proyecto.

### Favoritos y recientes

- **Favoritos:** marca un proyecto con la estrella para tenerlo arriba en el menú de proyectos. Solo aparecen los que puedes ver todavía.
- **Recientes:** se registra un proyecto cada vez que lo abres. El menú muestra los últimos.

### Archivar y eliminar

- **Archivar** deja el proyecto en solo lectura: no puedes cambiar su nombre, su configuración, su visibilidad ni su plantilla, y la API responde 409 (`project.archived`). El proyecto desaparece del listado normal y se puede ver con el filtro de archivados. **Restaurar** lo devuelve a la lista activa. En esta versión, el bloqueo cubre la configuración del proyecto; las incidencias y el tablero todavía no se bloquean al archivar.
- **Eliminar** es un borrado lógico. Solo lo pueden hacer el lead y los propietarios o administradores de la organización. Debes escribir el nombre exacto del proyecto para confirmarlo. El proyecto deja de aparecer en todas las vistas y su clave queda libre. La interfaz no ofrece recuperarlo.

### Qué pueden hacer los invitados y los viewers

- **Invitado sin acceso explícito:** no ve el proyecto. No aparece en listados, búsquedas ni favoritos.
- **Invitado con rol viewer:** ve el proyecto, sigue y vota incidencias, consulta informes y exporta. No crea ni modifica incidencias.
- **Viewer:** igual que el invitado con rol viewer. Un administrador del proyecto puede concederle permisos adicionales en el esquema, por ejemplo crear incidencias.

## Tareas frecuentes

### Asignar una incidencia

Un contributor o lead asigna a alguien de su mismo equipo. La persona asignada recibe una notificación.

### Planificar y cerrar un sprint

1. Como lead, crea el sprint en la pantalla de sprints con nombre, objetivo y fechas.
2. Añade incidencias desde el backlog.
3. Inicia el sprint. Si ya hay otro activo en el proyecto, debes cerrarlo primero.
4. Al cerrarlo, elige qué hacer con lo no terminado: volver al backlog o pasar a un sprint planificado del mismo proyecto.

### Cambiar el límite WIP de una columna

Como lead, escribe el número en el campo de la cabecera de la columna y pulsa Intro o sal del campo. Deja el campo vacío para quitar el límite. El límite debe ser un entero mayor que cero.

### Cambiar el flujo de estados

Desde la API, el lead puede reemplazar las transiciones del flujo por defecto. Cada transición indica los roles de proyecto que pueden ejecutarla. No es posible saltar de un estado a sí mismo ni repetir una transición.

## Errores frecuentes

- **Prohibido:** no tienes el rol que requiere la acción. Pide a un lead que te cambie el rol o que haga la acción.
- **Conflicto (409):** la acción no es válida en el estado actual. Por ejemplo, la columna destino tiene su límite WIP lleno, el sprint no está planificado, o ya hay otro sprint activo.
- **Datos no válidos (422):** revisa las fechas (formato AAAA-MM-DD, y el fin no puede ser anterior al inicio) y los campos obligatorios.
- **No encontrado (404):** el recurso no existe o no pertenece al proyecto indicado.
