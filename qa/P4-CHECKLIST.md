# QA Checklist - incidencias

## Entorno

- Stack local: `docker compose -f docker-compose.local.yaml -f docker-compose.dev.yaml up -d --build`
- Datos: `SEED_PASSWORD=<contraseña> go run ./cmd/seed --scenario e2e --force-reset` (solo en una base de pruebas)
- Cuentas (organización Acme): `owner@corelog.test`, `orgadmin@corelog.test`, `lead@corelog.test` (lead de SCRUM), `member@corelog.test` (contributor), `viewer@corelog.test` (viewer de SCRUM), `guest@corelog.test` (invitado, viewer de KAN), `newbie@corelog.test` (sin organización).
- Datos sembrados útiles: SCRUM tiene una épica con hijas, comentarios con respuesta, mención, nota interna y reacciones en las incidencias 2 y 4, y tiempo registrado en las incidencias 2, 4 y 10. Las versiones 1.0 (publicada) y 1.1 existen en cada proyecto.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390 píxeles. Temas: claro y oscuro.

## Catálogos y campos (administrador de la organización)

- [ ] Crear, renombrar, reordenar y archivar un tipo de incidencia, una prioridad y una resolución; el cambio se ve en los formularios
- [ ] Solo existe un tipo de nivel épica y uno de nivel subtarea; crear un segundo responde conflicto (`catalog.hierarchy_level_taken`)
- [ ] Un nombre repetido en el mismo catálogo se rechaza
- [ ] Crear un campo personalizado de cada tipo (texto, párrafo, número, fecha, fecha y hora, selección, selección múltiple, casilla, usuario, enlace, etiquetas)
- [ ] Un campo con contexto solo aparece en los proyectos y tipos configurados
- [ ] Un campo obligatorio impide crear la incidencia sin valor y muestra el error junto al control
- [ ] Un enlace con esquema distinto de http o https se rechaza
- [ ] Un miembro común no ve las pantallas de catálogos ni campos (403 o ausencia en el menú)

## Crear, editar y detalle

- [ ] Crear una incidencia con solo el resumen; recibe clave `SCRUM-n` y aparece en el backlog y en el listado
- [ ] Resumen vacío o de más de 255 caracteres: error de validación (422) en el campo
- [ ] Fechas: inicio posterior a la fecha límite se rechaza
- [ ] Los puntos de historia admiten decimales (por ejemplo 0,5) y rechazan negativos
- [ ] Editar el resumen desde el detalle guarda al salir del campo y registra el cambio en el historial
- [ ] Asignar a una persona sin acceso al proyecto se rechaza
- [ ] El historial muestra quién cambió qué y cuándo, con los valores anterior y nuevo
- [ ] Abrir una incidencia por su clave (`/browse/SCRUM-2`) y por una clave antigua tras moverla (alias) funciona
- [ ] Votar y dejar de votar actualiza el contador; seguir y dejar de seguir añade y quita al seguidor

## Jerarquía y vínculos

- [ ] Una épica lista sus hijas y muestra el progreso de las terminadas
- [ ] Una subtarea exige padre; una subtarea no puede tener hijos
- [ ] Una épica no puede colgar de otra incidencia
- [ ] Crear vínculos de los seis tipos; cada lado muestra su redacción (bloquea / bloqueada por, etc.)
- [ ] Un vínculo de bloqueo que cierra un ciclo se rechaza
- [ ] Vincular una incidencia consigo misma se rechaza; un vínculo duplicado se rechaza
- [ ] Eliminar un vínculo lo quita de ambos lados

## Etiquetas, componentes y versiones

- [ ] Añadir y quitar etiquetas en una incidencia; una etiqueta nueva se crea con el proyecto
- [ ] El lead crea, renombra y elimina componentes; un contributor no puede (403)
- [ ] Un componente muestra su lead sembrado y filtra el listado
- [ ] Elegir versión de corrección y versión afectada; solo se ofrecen versiones del mismo proyecto

## Comentarios

- [ ] Comentar con formato; el comentario aparece al instante y suma al contador
- [ ] Responder a un comentario lo agrupa bajo el original (hilo de un nivel)
- [ ] Mencionar a una persona con `@`: recibe un aviso; una mención a alguien sin acceso al proyecto no genera aviso
- [ ] Reaccionar con cada una de las seis reacciones; pulsar otra vez la quita; el contador y los nombres coinciden
- [ ] Editar un comentario muestra "editado" y conserva las revisiones; solo el autor edita
- [ ] Eliminar deja el marcador en el hilo sin texto
- [ ] Un comentario interno (contributor o lead) lo ve `member@corelog.test` y `lead@corelog.test`, pero no `viewer@corelog.test` ni `guest@corelog.test`; tampoco aparece en su historial ni en avisos
- [ ] Un viewer no ve el editor de comentarios

## Tiempo y estimaciones

- [ ] Escribir la estimación `1w 2d 3h 30m` y verla normalizada; `1.5h`, `h`, `10` o `3x` se rechazan
- [ ] Registrar `2h` con estimación restante automática reduce el tiempo restante; nunca baja de cero
- [ ] Los modos fijar, reducir y dejar igual modifican el restante como se indica
- [ ] Un registro de más de 3 días o con inicio a más de un día en el futuro se rechaza
- [ ] Editar y eliminar un registro propio recalcula el tiempo registrado; no se puede editar el de otra persona sin ser lead
- [ ] La hoja de horas (`/timesheet`) agrupa por día y semana y coincide con los registros sembrados de las incidencias 2, 4 y 10

## Clonar, mover y convertir

- [ ] Clonar con y sin adjuntos, vínculos, subtareas y seguidores copia solo lo elegido y crea el vínculo "clona a"
- [ ] Mover a KAN pide asignar los estados y tipos que no existen allí; la clave anterior sigue abriendo la incidencia
- [ ] Mover a un proyecto donde no hay permiso de crear se rechaza (403 o 404 según visibilidad)
- [ ] Convertir tarea en error conserva datos; convertir a subtarea sin indicar padre pide elegirlo (422 `issue.convert_parent_required`)

## Acciones masivas y exportación

- [ ] Seleccionar 3 incidencias y asignarlas; el resultado indica las 3 correctas
- [ ] Una selección con una incidencia sin permiso o con transición no permitida muestra qué falló sin detener las demás
- [ ] Más de 200 incidencias se rechaza con mensaje claro
- [ ] Cambiar prioridad, etiquetas, un campo, hacer una transición, mover y eliminar en masa
- [ ] Exportar a CSV abre bien en una hoja de cálculo con tildes (BOM); una celda con `=1+1` se exporta neutralizada
- [ ] El CSV exportado solo contiene incidencias visibles para quien exporta

## Eliminar y restaurar

- [ ] Eliminar una incidencia (lead) la quita de listados, tablero y búsqueda; pide confirmación
- [ ] Restaurarla (API) la devuelve con su clave
- [ ] Un contributor no puede eliminar (403)

## Roles y permisos

- [ ] Propietario y administrador: todo en SCRUM y SEC
- [ ] Lead (`lead@corelog.test`): crea, edita, elimina, gestiona componentes y versiones
- [ ] Contributor (`member@corelog.test`): crea, edita, comenta, registra tiempo; no elimina ni gestiona componentes
- [ ] Viewer (`viewer@corelog.test`): lee, vota y sigue; ningún botón de edición; un PATCH directo responde 403
- [ ] Invitado (`guest@corelog.test`): solo ve KAN; SCRUM responde 404
- [ ] Sin organización (`newbie@corelog.test`): todo responde 404
- [ ] Una incidencia de SEC no aparece en búsquedas de `member@corelog.test`

## Estados de error y vacíos

- [ ] Incidencia inexistente: página 404 con enlace de regreso
- [ ] Edición concurrente: guardar sobre una versión vieja muestra el conflicto y permite recargar
- [ ] Sin red: el formulario conserva lo escrito y muestra el error
- [ ] Proyecto EMP sin incidencias: estado vacío con acción de crear

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Detalle de incidencia: columnas apiladas en 390, sin desbordamiento horizontal
- [ ] Editor de comentarios y reacciones: utilizables con el teclado y con foco visible
- [ ] Listado con acciones masivas: la barra de acciones no tapa contenido
- [ ] Insignias de prioridad y estado con contraste suficiente en oscuro
- [ ] Diálogos de mover y clonar caben en pantalla de 390

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa con `CORELOG_TEST_DATABASE_URL`
- [ ] Playwright: specs de incidencias y comentarios pasan en el entorno compose
