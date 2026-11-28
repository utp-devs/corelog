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

## Organizaciones y miembros

Una organización agrupa personas, equipos, proyectos y catálogos. Cada persona tiene un rol en la organización:

| Rol | Qué puede hacer |
|---|---|
| **Propietario** | Todo, incluida la transferencia de propiedad y la gestión de webhooks y auditoría. Siempre hay al menos uno |
| **Administrador** | Gestiona miembros, invitaciones, catálogos, campos personalizados, estados y plantillas. Actúa como lead en todos los proyectos |
| **Miembro** | Ve los proyectos abiertos y trabaja en los suyos. Puede crear proyectos si la política de la organización lo permite |
| **Invitado** | Solo ve los proyectos en los que tiene un rol directo o a través de un equipo. No crea proyectos |

### Invitar a una persona

1. Entra en **Organización > Invitaciones** (administrador o propietario).
2. Escribe el correo, elige el rol y envía la invitación.
3. La persona abre el enlace (`/invite/<token>`), acepta los documentos legales vigentes y se une. Si no tiene cuenta, la crea en ese paso.
4. Puedes reenviar o cancelar una invitación pendiente. Un enlace caducado o cancelado deja de funcionar.

### Gestionar miembros

1. Entra en **Organización > Miembros**.
2. Cambia el rol de una persona con el selector de su fila, o quítala de la organización.
3. Para transferir la propiedad, el propietario elige a otro miembro en **Configuración > Zona de peligro** y confirma.
4. Cualquier persona puede abandonar una organización, salvo el último propietario.

La pestaña **Equipos** del menú principal permite crear equipos y asignar miembros (rol de equipo `maintainer` o `member`). Un equipo puede recibir un rol en uno o varios proyectos (ver "Acceso de equipos").

### Catálogos y campos de la organización

El administrador define lo que usan todos los proyectos de la organización:

- **Tipos de incidencia, prioridades y resoluciones** (**Organización > Incidencias**): crea, renombra, reordena y cambia icono y color. Hay exactamente un tipo de nivel épica y uno de nivel subtarea.
- **Campos personalizados** (**Organización > Campos**): tipos texto, párrafo, número, fecha, fecha y hora, selección, selección múltiple, casilla, usuario, enlace y etiquetas. Cada campo se activa por proyecto y tipo de incidencia (contexto), y puede ser obligatorio.
- **Estados** (**Organización > Estados**): catálogo de estados con su categoría (por hacer, en curso, hecho), reutilizable en los flujos de trabajo.
- **Auditoría** (**Organización > Auditoría**): ver "Seguridad".

## Incidencias

### Tipos, prioridades y resoluciones

Los tipos son los de la plantilla del proyecto (épica, historia, tarea, error, subtarea, y los de seguimiento de errores). Cada tipo tiene un nivel jerárquico:

- **Épica:** agrupa historias y tareas.
- **Estándar:** historia, tarea, error, etc. Puede colgar de una épica.
- **Subtarea:** cuelga siempre de una incidencia estándar; no puede tener hijos.

La **prioridad** ordena la urgencia (de forma predeterminada: crítica, alta, media y baja). La **resolución** explica por qué una incidencia se terminó (por ejemplo "Hecho"). Pasar a un estado de categoría "hecho" asigna una resolución y volver a abrir la incidencia la quita, según la configuración de la transición.

### Crear y editar

1. Pulsa **Crear** en la cabecera (o la tecla `c`) y elige el proyecto y el tipo.
2. Escribe el resumen (obligatorio, hasta 255 caracteres) y los demás campos: descripción, prioridad, responsable, fechas de inicio y límite, puntos de historia, etiquetas, componentes, versiones y campos personalizados.
3. Guarda. La incidencia recibe una clave con la del proyecto y un número, por ejemplo `CORE-12`. Si la mueves a otro proyecto, la clave anterior sigue funcionando como alias.

Los campos que muestra el formulario y cuáles son obligatorios dependen del tipo y de los campos personalizados activos en el proyecto.

### Jerarquía: épica, historia y subtarea

En el detalle de una incidencia, la sección **Jerarquía** permite elegir la épica o incidencia padre, y ver y crear los hijos. La barra de progreso de una épica o historia cuenta los hijos terminados.

### Vínculos

Un vínculo une dos incidencias con un tipo. Se muestra con la redacción de cada lado:

| Tipo | Desde la incidencia de origen | Desde la de destino |
|---|---|---|
| bloquea | bloquea | bloqueada por |
| relaciona | se relaciona con | se relaciona con |
| duplica | duplica a | es duplicada por |
| clona | clona a | es clonada por |
| causa | causa | es causada por |
| implementa | implementa | es implementada por |

Los vínculos de bloqueo no pueden formar ciclos. Para crear uno: en el detalle, sección **Vínculos**, elige el tipo y busca la incidencia destino.

### Etiquetas, componentes y versiones

- **Etiquetas:** palabras libres con color, por proyecto. Se crean al escribirlas y las gestiona el lead.
- **Componentes:** partes del producto (por ejemplo "API" o "Frontend"). Cada componente puede tener un responsable (lead) del componente. Los gestiona el lead en **Ajustes > Componentes**.
- **Versiones:** ver "Versiones y lanzamientos". Una incidencia tiene versiones afectadas y versiones de corrección.

### Comentarios

1. Escribe el comentario en el editor del detalle de la incidencia. Admite formato (negrita, listas, código).
2. Para **mencionar** a alguien escribe `@` y elige a la persona. Recibe un aviso si puede leer el comentario.
3. Para **responder** pulsa Responder en un comentario; las respuestas se agrupan bajo el original.
4. Para **reaccionar** elige una de las reacciones: pulgar arriba, corazón, celebración, ojos, cohete o marca de verificación. Pulsa de nuevo para quitarla.
5. Un comentario editado muestra "editado" y conserva sus revisiones anteriores.
6. Eliminar un comentario deja su lugar en el hilo con un marcador y vacía el texto. Quién puede eliminar depende de los permisos del proyecto.

**Comentarios internos:** al escribir un comentario, marca **Interno** y elige el rol mínimo que puede leerlo (contributor o lead). Un viewer o invitado sin ese rol no lo ve ni recibe avisos de él.

### Registro de tiempo y estimaciones

La sección **Tiempo** del detalle muestra la estimación original, el tiempo registrado y el tiempo restante.

1. Escribe la **estimación original** con unidades `w` (semanas), `d` (días), `h` (horas) y `m` (minutos), por ejemplo `1w 2d 3h 30m`. Una semana laboral son 5 días y un día laboral 8 horas, salvo que la organización defina otra jornada.
2. Para **registrar tiempo**, pulsa Registrar, escribe la duración (como máximo 3 días por registro), la fecha y hora de inicio (no más de un día en el futuro) y una descripción opcional.
3. Elige cómo cambia el tiempo restante: **automático** (resta lo registrado), **fijar** un valor, **reducir** en una cantidad o **dejarlo igual**.
4. Edita o elimina tus registros; el lead puede eliminar cualquiera.

**Mi hoja de horas** (`/timesheet`) muestra tus registros por día y semana. En **Informes** hay un resumen de tiempo del proyecto por persona, incidencia o día.

### Clonar, mover y convertir

- **Clonar** (menú de la incidencia): crea una copia con un resumen propio ("Copia de ...") y un vínculo "clona a" con la original. Elige si copiar adjuntos, vínculos, subtareas y seguidores.
- **Mover a otro proyecto:** elige el proyecto destino. Si el estado, el tipo o la prioridad no existen allí, la pantalla pide que asignes un valor equivalente. La incidencia conserva el historial y su clave anterior como alias.
- **Convertir:** cambia el tipo (por ejemplo de tarea a error, o de tarea a subtarea). Convertir a subtarea pide elegir la incidencia padre; convertir una subtarea a otro tipo pide quitar o cambiar el padre.

### Acciones masivas

En el listado de incidencias, selecciona varias con las casillas y usa la barra de acciones: **asignar**, **cambiar prioridad**, **cambiar etiquetas**, **cambiar un campo**, **hacer una transición**, **mover** o **eliminar**. Se admiten hasta 200 incidencias por operación. El resultado indica qué incidencias se cambiaron y cuáles no (por ejemplo por falta de permiso o por una transición no permitida); las que no se pueden cambiar no detienen a las demás.

### Exportar a CSV

En el listado, pulsa **Exportar** para descargar las incidencias de la búsqueda actual en CSV (el archivo empieza con BOM para abrirse bien en hojas de cálculo y neutraliza las celdas que empiezan por `=`, `+`, `-` o `@`). Desde la API: `GET /api/v1/projects/{project}/export/issues.csv` exporta un proyecto, y `/api/v1/issues/export` exporta una búsqueda.

### Eliminar y restaurar

Eliminar una incidencia es un borrado lógico: desaparece de todas las vistas. El lead puede restaurarla (API `POST /issues/{id}/restore`). Con la depuración activada (ver `docs/despliegue.md`), pasados 30 días se elimina de forma definitiva.

## Flujos de trabajo

Un flujo define los **estados** de un proyecto y las **transiciones** entre ellos. Cada plantilla trae un flujo; el lead lo edita en **Ajustes > Flujo de trabajo** (`/projects/<clave>/workflow`).

- **Estados:** cada uno tiene nombre y una categoría: por hacer, en curso o hecho. La categoría decide dónde se cuenta la incidencia en tableros e informes. Debe haber un estado inicial y todos deben ser alcanzables.
- **Transiciones:** una transición tiene un nombre ("Empezar"), uno o varios estados de origen y un estado de destino. El tablero y el selector de estado solo ofrecen las transiciones permitidas.
- **Condiciones:** reglas que deciden quién puede ver y ejecutar la transición: persona con un rol del proyecto, responsable, informador, permiso concreto o un campo con un valor.
- **Validadores:** reglas que se comprueban al ejecutarla y pueden rechazarla con un mensaje: campo obligatorio, comentario obligatorio, resolución obligatoria, subtareas terminadas, responsable asignado.
- **Funciones posteriores:** acciones que se ejecutan después de la transición: asignar la incidencia (al informador, a quien ejecuta, al lead o a nadie), fijar o quitar la resolución, fijar un campo, añadir un comentario y emitir un evento.

### Editar el flujo

1. Abre el editor y pulsa **Plantillas** si quieres partir de una plantilla (Scrum, Kanban, seguimiento de tareas, seguimiento de errores).
2. Añade, renombra y reordena estados. Los estados de la organización se pueden reutilizar.
3. En la tabla de transiciones, añade una y elige origen, destino y sus condiciones, validadores y funciones posteriores.
4. El editor valida el flujo antes de guardar (estado inicial, destinos existentes, estados inalcanzables).
5. Si quitas un estado que tiene incidencias, el editor pide **asignar un estado de destino** a esas incidencias; sin ese paso no se guarda.
6. **Restablecer** devuelve el flujo al de la plantilla del proyecto.

Solo el lead edita el flujo. Un viewer solo puede cambiar el estado de una incidencia de la que es responsable, y solo si el flujo se lo permite.

## Tableros

El tablero (`/projects/<clave>/board`) muestra una columna por grupo de estados. Arrastra una tarjeta a otra columna para ejecutar la transición; si el flujo necesita datos (resolución, comentario), se abre un diálogo.

- **Columnas:** el lead las crea, renombra, reordena y asigna estados a cada una en el panel de configuración del tablero.
- **Límite WIP:** número máximo de tarjetas de una columna. Con el límite lleno la columna no acepta más y la API responde 409. Se cambia en la cabecera de la columna.
- **Calles (swimlanes):** agrupan las tarjetas en filas por **responsable**, **épica**, **historia (subtareas bajo su padre)** o por **consulta** (cada calle es una consulta CoreQL). Elige la agrupación en el selector del tablero. Las calles se pueden contraer.
- **Filtros rápidos:** botones sobre el tablero, cada uno una consulta CoreQL (por ejemplo "Mis incidencias": `assignee = currentUser()`). Activa uno o varios para reducir las tarjetas; el lead los crea en el panel de configuración. Los tableros nuevos traen "Mis incidencias", "Sin responsable" y "Prioridad alta".
- **Varios tableros:** un proyecto puede tener más de un tablero con su propia configuración (estimación en puntos o en tiempo, campos de la tarjeta).

## Backlog y sprints

Solo los proyectos con la plantilla Scrum tienen sprints.

### Backlog

`/projects/<clave>/backlog` lista las incidencias por orden de prioridad, con los sprints planificados y el activo en la parte superior. Arrastra una incidencia para reordenarla o para llevarla a un sprint. Un contributor puede ordenar el backlog; el lead gestiona los sprints.

### Iniciar un sprint

1. Pulsa **Crear sprint** y escribe nombre, objetivo y fechas.
2. Arrastra las incidencias al sprint.
3. Pulsa **Iniciar sprint**. Se guarda una instantánea de lo comprometido (incidencias y puntos). Solo puede haber un sprint activo por tablero.

### Completar un sprint

1. En el sprint activo pulsa **Completar**.
2. Elige el destino de las incidencias no terminadas: **backlog**, otro **sprint planificado**, o **un sprint nuevo** (con nombre propio o el siguiente en la numeración).
3. Confirma. La operación es atómica: si algo falla, nada cambia. Cada incidencia queda registrada como completada o no completada con sus puntos de ese momento, y el sprint guarda los puntos comprometidos y completados.

### Informe del sprint

`/projects/<clave>/sprints/<id>/report` muestra lo comprometido, lo completado, lo no completado y lo añadido o retirado durante el sprint, con los puntos de cada grupo. Está disponible para sprints activos (parcial) y cerrados. El burndown y la velocidad se explican en "Informes".

## Versiones y lanzamientos

Las versiones agrupan las incidencias que llegan en un lanzamiento. Se gestionan en **Ajustes > Versiones** (`/projects/<clave>/versions`).

1. Crea una versión con nombre (único en el proyecto), fechas de inicio y de lanzamiento previstas y una descripción.
2. En cada incidencia elige la **versión de corrección** (en la que se resolverá) y las **versiones afectadas** (en las que ocurre).
3. La tarjeta de la versión muestra el progreso: incidencias y puntos por categoría (por hacer, en curso, hecho).
4. Pulsa **Publicar**. Si quedan incidencias sin terminar, elige qué hacer: dejarlas, moverlas a otra versión sin publicar o crear la siguiente versión y moverlas allí. Publicar emite el evento `version.released`.
5. **Notas de la versión** (`GET /versions/{id}/notes`) genera un texto en Markdown con las incidencias de la versión agrupadas por tipo.
6. **Archivar** oculta una versión que ya no se usa.

Requiere el permiso de gestionar versiones (lead de forma predeterminada).

## Búsqueda y CoreQL

La búsqueda global de la cabecera (atajo `/` o `Ctrl+K`) encuentra proyectos e incidencias por texto. El **navegador de incidencias** (`/issues`) permite además buscar con **CoreQL**, un lenguaje de consultas.

### Gramática

```
consulta  := [ condición ] [ ORDER BY campo [ASC|DESC] { , campo [ASC|DESC] } ]
condición := expresión { OR expresión }
expresión := término { AND término }
término   := NOT término | ( condición ) | cláusula
cláusula  := campo operador valor
           | campo IN ( valor, ... )  |  campo NOT IN ( valor, ... )
           | campo IS EMPTY  |  campo IS NOT EMPTY   (también NULL)
```

- Las palabras clave (`AND`, `OR`, `NOT`, `IN`, `IS`, `EMPTY`, `ORDER BY`, `ASC`, `DESC`) y los nombres de campo no distinguen mayúsculas.
- Los valores con espacios van entre comillas: `status = "En curso"`.
- Los campos personalizados se escriben `cf[clave]`.
- Operadores: `=`, `!=`, `>`, `>=`, `<`, `<=`, `~` (contiene) y `!~` (no contiene). Texto admite `=`, `!=`, `~`, `!~`; números y fechas admiten comparaciones; el resto de tipos admiten `=`, `!=`, `IN` e `IS`.
- Fechas: `2026-11-30`, o relativas como `-7d`, `+3d`, `1M`.

### Campos

| Tipo | Campos |
|---|---|
| Texto | `key`, `summary`, `description`, `comment`, `text` (busca en resumen, descripción y comentarios) |
| Catálogos | `project`, `issuetype` (o `type`), `status`, `statusCategory`, `resolution`, `priority` |
| Personas | `assignee`, `reporter`, `creator`, `watcher` |
| Varios valores | `labels`, `component`, `fixVersion`, `affectedVersion`, `sprint` |
| Jerarquía | `epic`, `parent` |
| Fechas | `created`, `updated`, `resolved`, `due`, `start` |
| Números | `votes`, `storyPoints`, `originalEstimate`, `remainingEstimate`, `timeSpent` |
| Otros | `flagged` (sí o no) y los campos personalizados `cf[...]` |

Se puede ordenar por: `project`, `key`, `issuetype`, `status`, `statusCategory`, `resolution`, `priority`, `assignee`, `reporter`, `creator`, `summary`, las fechas y los números.

### Funciones

| Función | Devuelve |
|---|---|
| `currentUser()` | La persona que consulta |
| `membersOf("equipo")` | Los miembros de un equipo |
| `now()` | El instante actual |
| `startOfDay()`, `endOfDay()`, `startOfWeek()`, `endOfWeek()`, `startOfMonth()`, `endOfMonth()` | Límites de periodos; admiten un desplazamiento, por ejemplo `startOfDay(-7d)` |
| `openSprints()`, `closedSprints()`, `futureSprints()` | Sprints por estado |
| `releasedVersions()`, `unreleasedVersions()` | Versiones por estado |
| `currentProject()` | El proyecto en el que estás |

### Ejemplos

```
assignee = currentUser() AND statusCategory != done ORDER BY priority ASC, updated DESC
project = CORE AND type = Error AND priority IN (Alta, Crítica) AND resolution IS EMPTY
due < startOfDay() AND statusCategory != done
labels IN (urgente, infra) AND NOT status = "En revisión"
sprint IN openSprints() AND assignee IS EMPTY
fixVersion IN unreleasedVersions() AND created >= -30d
summary ~ "certificado" OR comment ~ "certificado"
cf[severidad] = Alta
```

El navegador valida la consulta mientras escribes: subraya el error con su posición y sugiere campos, operadores y valores. Una consulta que no se puede interpretar nunca ejecuta una búsqueda parcial. Solo ves incidencias de proyectos a los que tienes acceso.

## Filtros guardados y suscripciones

Un filtro guarda una consulta CoreQL, las columnas y el orden.

1. Escribe la consulta en `/issues` y pulsa **Guardar filtro**; ponle nombre y descripción.
2. **Compartir:** abre Compartir y elige con quién: personas (pueden ver o editar), equipos, un proyecto o toda la organización (solo ver).
3. **Suscribirte:** abre Suscripción y elige la frecuencia (diaria a una hora, o semanal en un día). Recibes por correo y en la aplicación las incidencias que cumplen el filtro. Cada persona gestiona su suscripción.
4. Desde `/filters` puedes ver, abrir, renombrar y eliminar tus filtros y los compartidos contigo. Los filtros se pueden usar en gadgets de paneles y en webhooks.

## Notas personales

`/notes` es un bloc privado: solo tú ves tus notas.

1. Pulsa **Nueva nota** (o la tecla `n`) y escribe título, texto y etiquetas (hasta 10, de 30 caracteres).
2. Elige un color y **fija** las importantes para que queden arriba.
3. Enlaza una nota a una incidencia con el selector de incidencias; desde la incidencia se llega a la nota.
4. Filtra por etiqueta o busca por texto. Un título admite hasta 120 caracteres y el texto hasta 20 000.

## Informes

En **Informes** (`/projects/<clave>/reports`) hay once informes. Casi todos aceptan un rango de fechas (por defecto los últimos 30 días, máximo 366), un filtro CoreQL opcional y una agrupación.

| Informe | Qué muestra |
|---|---|
| **Burndown** | Trabajo restante del sprint frente a la línea ideal. Necesita fechas de inicio y fin |
| **Burnup** | Trabajo completado y alcance total del sprint a lo largo del tiempo |
| **Velocidad** | Puntos comprometidos y completados de los últimos sprints cerrados (6 por defecto, hasta 20) |
| **Informe del sprint** | Resumen de un sprint concreto: completado, no completado, añadido y retirado |
| **Flujo acumulado** | Número de incidencias por estado a lo largo del tiempo; detecta cuellos de botella |
| **Tiempo de ciclo** (control) | Tiempo que tarda cada incidencia en llegar a terminada, con la media y la dispersión |
| **Creadas frente a resueltas** | Incidencias abiertas y cerradas por día, semana o mes |
| **Tiempo de resolución** | Cuánto tarda en resolverse lo creado, por agrupación |
| **Seguimiento de tiempo** | Estimado frente a registrado y restante |
| **Carga de trabajo** | Incidencias y tiempo pendiente por responsable |
| **Versión** | Progreso de una versión (hay que elegir cuál) |

Los informes de sprint piden elegir el sprint. Las gráficas tienen una tabla equivalente. Si el filtro abarca demasiadas incidencias, el informe responde con un aviso para acotarlo. Los viewers pueden consultar informes.

## Paneles y gadgets

Un panel es una página de gadgets. **Mi resumen** (`/dashboard`) es tu panel predeterminado; en **Paneles** (`/dashboards`) creas más.

1. Pulsa **Nuevo panel**, ponle nombre y elige una distribución en columnas.
2. En el editor, añade gadgets (hasta 20), configura cada uno y arrástralos entre columnas.
3. Guarda. **Duplicar** crea una copia que puedes editar libremente.

Gadgets disponibles: asignadas a mí, creadas por mí, resultados de un filtro, estadísticas de incidencias, gráfico circular, dos dimensiones, creadas frente a resueltas, burndown del sprint, flujo de actividad, proyectos recientes, favoritos, texto (Markdown) y próximos eventos del calendario.

**Compartir:** elige personas, equipos, un proyecto o la organización, con permiso de **ver** o de **editar**. Quien solo puede ver no modifica el panel.

## Hoja de ruta

`/projects/<clave>/roadmap` muestra una línea de tiempo del proyecto.

- **Agrupar por:** épica, versión o sprint.
- **Filtrar:** todas, abiertas o terminadas.
- Las barras de las épicas usan su fecha de inicio y límite; si no las tienen, se deducen de sus incidencias hijas. Una barra sin fechas se muestra como "sin planificar".
- El orden manual de las épicas es independiente del backlog (API `PUT /projects/{id}/roadmap/rank`). Las fechas de una épica se editan en la propia incidencia.
- Las **dependencias** entre épicas se dibujan como flechas y salen de los vínculos de bloqueo.

## Calendario

`/calendar` muestra tus eventos y los del proyecto, junto con sprints, versiones y fechas límite de incidencias. Hay vistas de mes, semana, día y agenda.

### Crear un evento

1. Pulsa **Nuevo evento** o haz clic en una celda.
2. Escribe título, fecha y hora de inicio y fin (o marca **Todo el día**), zona horaria, lugar, descripción y color.
3. Elige la **visibilidad**: privado (solo tú y los invitados), proyecto (los miembros de un proyecto) u organización.
4. Añade invitados (hasta 100). Cada uno responde: acepto, rechazo, tal vez o sin respuesta.

### Eventos recurrentes

En **Repetir** elige diaria, semanal (con días de la semana), mensual (por fecha o por posición, por ejemplo "el segundo lunes") o anual, con un intervalo y un final: nunca, tras N veces (hasta 5000) o en una fecha. Al editar o eliminar una ocurrencia, elige si afecta **solo a esa ocurrencia** (queda como excepción o cancelada) o **a toda la serie**.

### Suscripción ICS

1. En el calendario abre **Suscripción** y pulsa **Crear enlace**.
2. Copia la URL privada y añádela como calendario en tu aplicación (Google Calendar, Outlook, Apple). El enlace solo se muestra una vez.
3. La suscripción incluye los eventos visibles de los 30 días anteriores y los 90 siguientes. **Rotar** el enlace invalida el anterior.
4. Para descargar un archivo suelto: **Exportar .ics** (`GET /api/v1/calendar/export.ics`).

## Notificaciones

La campana de la cabecera muestra los avisos sin leer (se actualiza cada 30 segundos). Márcalos como leídos uno a uno o todos a la vez.

### Qué avisa y a quién

Un **esquema de notificaciones** decide, para cada evento, a quién avisar y por qué canal (en la aplicación, por correo o ambos). Los eventos son: incidencia creada, asignada, comentada, mencionada, transicionada, resuelta o actualizada; comentario editado; tiempo registrado; versión publicada; sprint iniciado o completado; y suscripción de filtro. Los destinatarios pueden ser el informador, el responsable, los seguidores, el lead del proyecto, el lead del componente, las personas mencionadas, el autor de un comentario al que se responde, una persona, un equipo o un rol del proyecto.

- **Esquema predeterminado:** avisa en la aplicación al responsable asignado, a quien es mencionado y al autor de un comentario al que responden.
- **Esquema de la organización:** sustituye el predeterminado evento por evento. Solo API: `GET`/`PUT /organizations/{org}/notification-scheme`.
- **Esquema del proyecto:** sustituye al de la organización evento por evento. Solo API: `GET`/`PUT /projects/{project}/notification-scheme`.

Quien comenta o cambia algo no recibe aviso de sus propias acciones, salvo que active esa preferencia.

### Preferencias personales

En **Perfil > Notificaciones** (`/profile/notifications`) cada persona decide:

1. **En la aplicación:** si se guardan avisos en la campana.
2. **Correo:** **inmediato** (un correo por aviso), **resumen diario** (un solo correo con todos los avisos del día, a la hora que configura el administrador) o **desactivado**. Solo afecta a los eventos que el esquema envía por correo.
3. **Mis propias acciones:** recibir también avisos de lo que haces tú.

Los correos salen por una cola con reintentos. Los administradores de la plataforma ven la cola y los envíos fallidos en `GET /admin/email-outbox` y pueden reintentar uno (solo API).

## Automatización

Las reglas de automatización ejecutan acciones cuando ocurre algo en un proyecto. El lead las gestiona; hoy se administran por la API (`/projects/{project}/automation-rules`) — solo API.

Una regla tiene un **disparador** ("cuando"), **condiciones** ("si", todas deben cumplirse, hasta 10) y **acciones** ("entonces", de 1 a 10).

### Disparadores

| Disparador | Cuándo se activa |
|---|---|
| `issue_created` | Se crea una incidencia |
| `issue_updated` | Se edita una incidencia; opcionalmente solo si cambia un campo concreto (`field`) |
| `issue_transitioned` | Cambia el estado; opcionalmente de un estado (`from`) a otro (`to`) |
| `comment_added` | Se añade un comentario |
| `sprint_started`, `sprint_completed` | Cambia el estado de un sprint |
| `scheduled` | Cada `every_minutes` (de 5 minutos a 7 días); la regla actúa sobre cada incidencia (máximo 50) que cumpla la consulta CoreQL `query` |

### Condiciones

`query` (la incidencia cumple una consulta CoreQL), `field_equals` y `field_in` (un campo vale o está entre varios valores), `actor_role` (la persona que provocó el evento tiene un rol) e `issue_type`.

### Acciones

| Acción | Qué hace |
|---|---|
| `transition_issue` | Cambia el estado (`status`) respetando el flujo |
| `assign_issue` | Asigna a una persona, al informador, al lead o por turnos (`round_robin` entre un equipo) |
| `set_field` | Cambia un campo |
| `add_comment` | Añade un comentario |
| `add_label`, `remove_label` | Añade o quita una etiqueta |
| `create_subtask` | Crea una subtarea con el texto como resumen |
| `send_webhook` | Envía una petición HTTP a una URL pública |
| `send_notification` | Avisa al usuario, responsable, informador o lead |

### Valores inteligentes

Los textos de las acciones admiten marcadores `{{...}}` que se sustituyen al ejecutar: `{{issue.key}}`, `{{issue.summary}}`, `{{issue.status}}`, `{{issue.priority}}`, `{{issue.type}}`, `{{actor.name}}`, `{{project.key}}`, `{{project.name}}`, `{{trigger.from}}`, `{{trigger.to}}` y `{{rule.name}}`. Un marcador desconocido hace que la regla no se pueda guardar. Los valores se escapan y no se evalúan de nuevo.

### Seguridad de las reglas

Para evitar bucles, una regla que se dispara por el efecto de otra regla se detiene tras 5 encadenamientos. Cada regla se limita a 100 ejecuciones por hora. Cada ejecución queda registrada (correcta, omitida o fallida) en `GET /automation-rules/{id}/runs`. Una regla se puede **desactivar** sin borrarla, y `POST /automation-rules/validate` comprueba una definición sin guardarla. Si dos personas editan una regla a la vez, la segunda recibe un conflicto (409) y debe recargar.

## Webhooks

Un webhook envía a una URL externa cada evento al que se suscribe. Los gestionan los propietarios y administradores de la organización por la API (`/organizations/{org}/webhooks`) — solo API.

1. Crea el webhook con nombre, URL `http` o `https` pública, los eventos y, si quieres, un filtro CoreQL. Eventos disponibles: `issue.created`, `issue.updated`, `issue.transitioned`, `issue.deleted`, `issue.restored`, `comment.created`, `sprint.started`, `sprint.completed` y `version.released` (o `*` para todos). Las URLs privadas, locales o internas se rechazan.
2. La respuesta incluye el **secreto de firma** (`whsec_...`) una sola vez. Guárdalo; `POST .../rotate-secret` genera uno nuevo.
3. Envía un evento de prueba con `POST .../test`.
4. Revisa las entregas con `GET .../deliveries` y reenvía una con `POST .../deliveries/{delivery}/redeliver`.

### Firma y cabeceras

Cada entrega es un `POST` con el cuerpo JSON y estas cabeceras:

| Cabecera | Contenido |
|---|---|
| `X-CoreLog-Event` | Tipo de evento (por ejemplo `issue.created`) |
| `X-CoreLog-Delivery` | Identificador de la entrega |
| `X-CoreLog-Timestamp` | Instante del envío, en segundos Unix |
| `X-CoreLog-Signature` | `sha256=` seguido del HMAC-SHA256 en hexadecimal de `<timestamp>.<cuerpo>` con el secreto |

Comprueba siempre la firma sobre el **cuerpo sin modificar** y rechaza marcas de tiempo con más de 5 minutos de antigüedad.

```js
// Node.js
const crypto = require('crypto');
function verify(secret, timestamp, rawBody, signature) {
  const expected = 'sha256=' + crypto.createHmac('sha256', secret).update(`${timestamp}.${rawBody}`).digest('hex');
  const a = Buffer.from(expected), b = Buffer.from(signature);
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}
```

```go
// Go
func verify(secret, timestamp string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

```python
# Python
import hmac, hashlib
def verify(secret: str, timestamp: str, body: bytes, signature: str) -> bool:
    mac = hmac.new(secret.encode(), timestamp.encode() + b"." + body, hashlib.sha256)
    return hmac.compare_digest("sha256=" + mac.hexdigest(), signature)
```

### Reintentos

Responde con un código 2xx en menos de 10 segundos. Si no, CoreLog reintenta hasta 6 intentos en total, esperando 1 minuto, 5 minutos, 30 minutos, 2 horas y 12 horas entre ellos. Tras el último intento la entrega queda como `dead` y se puede reenviar a mano. Procesa las entregas de forma idempotente usando `X-CoreLog-Delivery`.

## Tokens de API

Un token de API personal permite que un script o una integración use la API con tus permisos, sin contraseña.

1. Crea el token en `POST /api/v1/me/api-tokens` (solo API; la gestión de tokens exige sesión iniciada) con un nombre, los alcances y, opcionalmente, una caducidad (máximo 366 días).
2. Copia el valor (`cl_pat_...`): solo se muestra una vez; en el sistema se guarda únicamente su huella.
3. Úsalo como `Authorization: Bearer cl_pat_...`.
4. Revoca el token con `DELETE /api/v1/me/api-tokens/{id}`. Puedes tener hasta 20 tokens activos.

| Alcance | Permite |
|---|---|
| `read` | Consultas (GET) |
| `write` | Consultas y cambios |
| `admin` | Como `write`; solo para administradores de la plataforma |

Un token nunca puede hacer más que su dueño y no sirve para iniciar sesión, cambiar contraseñas, gestionar sesiones, dos pasos ni otros tokens (responde 403 `api_token.session_required`). Un token con alcance insuficiente recibe 403 `api_token.scope_insufficient`. Los tokens tienen un límite de peticiones por minuto (429 al superarlo).

## Importación y exportación

- **Importar incidencias desde CSV** (solo API: `POST /projects/{project}/import/csv`): sube el archivo (hasta 10 MiB y 20 000 filas), revisa la correspondencia sugerida de columnas a campos (reconoce los nombres de las exportaciones de Jira y los equivalentes en español) y arranca la importación con `POST .../import/csv/{job}/start`. Campos importables: resumen (obligatorio), descripción, tipo, prioridad, estado, responsable, informador, etiquetas, componentes, versiones de corrección, fechas de inicio y límite, puntos, padre y campos personalizados (`custom:<nombre>`). Consulta el avance en `GET .../import/jobs/{job}`; las filas con error se descargan en `.../errors.csv`.
- **Exportar incidencias a CSV:** desde el listado de incidencias o por la API (ver "Incidencias").
- **Copia completa de un proyecto en JSON** (solo API): `GET /projects/{project}/export/project.json` incluye proyecto, incidencias, comentarios, registros de trabajo y vínculos, con límites de tamaño (el campo `truncated` avisa si se recortó). Solo el lead y los administradores.
- **Exportar auditoría a CSV:** ver "Seguridad".

## Seguridad

### Verificación en dos pasos (2FA)

1. Entra en **Perfil > Seguridad** y pulsa **Activar verificación en dos pasos**.
2. Escanea el código QR con una aplicación de autenticación (o escribe la clave manualmente) y escribe el código de 6 dígitos para confirmar.
3. Guarda los **10 códigos de recuperación**: se muestran una sola vez y cada uno sirve una vez.
4. Al iniciar sesión se pide el código de 6 dígitos (cambia cada 30 segundos) o uno de recuperación. Tras 5 intentos fallidos o 5 minutos, hay que volver a empezar el inicio de sesión.
5. Para desactivarla o regenerar los códigos de recuperación se pide la contraseña y un código.

### Contraseñas

La contraseña debe tener al menos 10 caracteres combinando tres de: mayúsculas, minúsculas, números y símbolos (o 14 caracteres o más), como máximo 72 bytes, no contener la parte local de tu correo y no ser una contraseña muy común.

- **Cambiar la contraseña:** en **Perfil > Seguridad** escribe la actual y la nueva dos veces.
- **Recuperarla:** en el inicio de sesión pulsa "¿Olvidaste tu contraseña?", escribe el correo y abre el enlace que llega por correo (caduca y solo sirve una vez). La respuesta es la misma exista o no la cuenta. Al restablecerla se cierran las demás sesiones.

### Verificación del correo

Tras registrarte llega un correo con un enlace de verificación (`/verify-email`). Puedes pedir otro desde la pantalla de inicio de sesión. Si el administrador de la plataforma exige correo verificado, no podrás iniciar sesión hasta verificarlo.

### Bloqueo por intentos fallidos

Tras 5 intentos fallidos de iniciar sesión en una cuenta en 15 minutos, la cuenta se bloquea 15 minutos; cada bloqueo consecutivo duplica la espera, hasta 24 horas. La pantalla indica el tiempo restante. Además hay límites de frecuencia por dirección en el inicio de sesión, el registro, la recuperación de contraseña y la verificación del teléfono (429).

### Sesiones

En **Perfil > Seguridad** ves tus sesiones abiertas (dispositivo y última actividad). Puedes cerrar una o **cerrar todas las demás**. Desactivar tu cuenta (**Perfil > Privacidad**) cierra todas las sesiones.

### Auditoría

La auditoría registra quién hizo qué y cuándo: cambios de roles y permisos, inicios de sesión relevantes, 2FA, tokens, webhooks, borrados y cambios de configuración. Cada entrada tiene una descripción legible, la persona, la dirección y el identificador de la petición.

- **Organización > Auditoría** (propietarios y administradores): eventos de la organización, con filtros por fecha, persona, tipo y entidad, y **exportación a CSV**.
- **Organización > Auditoría de la plataforma** (administradores de la plataforma): todos los eventos.
- Los registros se conservan `AUDIT_RETENTION_DAYS` días (365 por omisión).

## Privacidad y datos personales

En **Perfil > Privacidad** (`/profile/privacy`) gestionas lo que CoreLog sabe de ti.

- **Nombre:** se guarda como nombre y apellidos, más un nombre visible que puedes cambiar.
- **Consentimientos:** CoreLog publica documentos legales (política de privacidad, tratamiento de datos, términos de uso, cookies y comunicaciones). Los tres primeros son obligatorios por omisión; **comunicaciones** siempre es opcional. Al registrarte o aceptar una invitación debes aceptar los obligatorios. Cuando se publica una versión nueva de un documento obligatorio, se te pide aceptarla al iniciar sesión (o se bloquea el uso hasta aceptarla, según la configuración). Puedes ver lo que aceptaste y cuándo, y retirar un consentimiento opcional.
- **Teléfono:** elige el país y escribe tu número. Para verificarlo: pulsa Verificar, recibe un código de 6 dígitos (vale 10 minutos, hasta 5 intentos y hasta 3 envíos por hora) y escríbelo. Cambiar el número quita la verificación. Con el proveedor de SMS de desarrollo, el código no se envía a ningún teléfono.
- **Desactivar la cuenta:** cierra todas tus sesiones y bloquea el acceso. Se pide la contraseña.
- Los administradores de la plataforma pueden publicar documentos legales y ver los consentimientos de las personas en `/organization/legal`.

## Atajos de teclado

Los atajos funcionan fuera de los campos de texto. Pulsa `?` para ver la lista.

| Atajo | Acción |
|---|---|
| `c` | Crear incidencia |
| `/` o `Ctrl+K` (`Cmd+K` en Mac) | Abrir la búsqueda global |
| `?` | Mostrar los atajos |
| `n` | Notas rápidas |
| `g` y luego `i` | Ir a Incidencias |
| `g` y luego `d` | Ir a Mi resumen |
| `g` y luego `p` | Ir a Proyectos |
| `g` y luego `b` | Ir al tablero del proyecto abierto |

En una secuencia (`g` y luego otra tecla) la segunda tecla debe pulsarse enseguida.

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

El lead abre **Ajustes > Flujo de trabajo** y edita estados y transiciones en el editor (ver "Flujos de trabajo"). Cada transición puede limitarse por rol, responsable u otras condiciones. No es posible saltar de un estado a sí mismo ni repetir una transición.

## Errores frecuentes

- **Prohibido:** no tienes el rol que requiere la acción. Pide a un lead que te cambie el rol o que haga la acción.
- **Conflicto (409):** la acción no es válida en el estado actual. Por ejemplo, la columna destino tiene su límite WIP lleno, el sprint no está planificado, o ya hay otro sprint activo.
- **Datos no válidos (422):** revisa las fechas (formato AAAA-MM-DD, y el fin no puede ser anterior al inicio) y los campos obligatorios.
- **No encontrado (404):** el recurso no existe o no pertenece al proyecto indicado.
