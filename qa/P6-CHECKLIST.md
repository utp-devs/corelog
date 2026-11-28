# QA Checklist - agile (tableros, backlog, sprints, versiones)

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. SCRUM trae el "Sprint 0" cerrado con historia (comprometido 18 puntos, completado 10), el "Sprint 1" activo, y las versiones 1.0 (publicada) y 1.1 en cada proyecto.
- Cuentas: `owner`, `orgadmin`, `lead` (lead de SCRUM), `member`, `viewer`, `guest`, `newbie` de `@corelog.test`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Tablero

- [ ] El tablero de SCRUM muestra las columnas con sus tarjetas y los contadores correctos
- [ ] Arrastrar una tarjeta cambia el estado y persiste tras recargar
- [ ] Una columna con límite WIP lleno no acepta más tarjetas (409) y la interfaz lo explica
- [ ] El lead cambia el límite WIP en la cabecera; vacío lo quita; 0, negativo o texto se rechazan
- [ ] Las columnas se crean, renombran, reordenan y eliminan desde la configuración (lead); una columna con estados asignados no los pierde
- [ ] Un estado solo puede pertenecer a una columna del tablero
- [ ] Calles por responsable, épica, historia y consulta: las tarjetas se agrupan y las calles se pueden contraer
- [ ] Una calle por consulta con una consulta no válida muestra el error
- [ ] Los tres filtros rápidos sembrados filtran; combinar dos aplica ambos; limpiar restaura
- [ ] El lead crea, edita y elimina un filtro rápido con una consulta CoreQL válida; una consulta no válida se rechaza
- [ ] Crear un segundo tablero y cambiar entre ellos
- [ ] Mover una tarjeta con teclado (botón "Mover a otra columna") funciona
- [ ] Un viewer no puede arrastrar ni ve los controles de configuración

## Backlog

- [ ] El backlog lista el sprint activo, los planificados y las incidencias sin sprint
- [ ] Arrastrar una incidencia dentro del backlog cambia el orden y persiste
- [ ] Arrastrar o usar el menú para mover una incidencia a un sprint y devolverla al backlog
- [ ] Una incidencia terminada no se ofrece para añadir a un sprint nuevo, o se avisa
- [ ] Los puntos totales por sprint se actualizan al mover incidencias
- [ ] Un proyecto Kanban (KAN) no ofrece sprints

## Sprints

- [ ] Crear un sprint con nombre, objetivo y fechas; fecha de fin anterior al inicio se rechaza (422)
- [ ] Iniciar un sprint planificado lo marca activo y guarda lo comprometido
- [ ] Iniciar un segundo sprint con otro activo se rechaza (409) y el mensaje lo explica
- [ ] Completar un sprint con destino backlog, otro sprint planificado y sprint nuevo; en cada caso las no terminadas llegan al destino
- [ ] Completar sin cambios inesperados cuando todo está terminado: no hay diálogo de destino
- [ ] Si la operación falla, no queda nada a medias (probar con un destino inválido)
- [ ] Editar y eliminar un sprint planificado; un sprint activo o cerrado no se elimina
- [ ] El informe del sprint (`/projects/SCRUM/sprints/<id>/report`) del Sprint 0 muestra comprometido, completado y no completado con los puntos sembrados
- [ ] El informe de un sprint activo se marca como parcial

## Versiones

- [ ] La lista de versiones muestra 1.0 publicada y 1.1 sin publicar, con progreso por categoría
- [ ] Crear una versión; un nombre repetido (sin importar mayúsculas) se rechaza
- [ ] Fecha de lanzamiento anterior al inicio se rechaza
- [ ] Publicar 1.1 con incidencias sin terminar ofrece: dejarlas, moverlas a otra versión sin publicar, o crear la siguiente y moverlas
- [ ] Tras publicar, la versión muestra la fecha real y deja de aceptar nuevas incidencias de corrección, o lo indica
- [ ] Las notas de la versión agrupan las incidencias por tipo y el Markdown se puede copiar
- [ ] Archivar una versión la oculta de los selectores
- [ ] Un contributor ve las versiones pero no puede crear, publicar ni archivar (403)

## Componentes

- [ ] Los componentes sembrados muestran su lead
- [ ] Cambiar el lead de un componente y quitarlo
- [ ] Un lead de componente que no es miembro del proyecto se rechaza

## Roles y permisos

- [ ] Propietario y administrador gestionan todo
- [ ] Lead de SCRUM gestiona tablero, sprints y versiones de SCRUM
- [ ] Contributor mueve tarjetas y ordena el backlog; no inicia ni completa sprints, ni cambia WIP
- [ ] Viewer solo lee
- [ ] Invitado con rol viewer en KAN lee el tablero; en SCRUM recibe 404
- [ ] Sin organización: 404

## Estados de error y vacíos

- [ ] Proyecto EMP: tablero y backlog vacíos con mensaje y acción
- [ ] Mover una tarjeta que otra persona acaba de mover: se muestra el conflicto y se refresca el tablero
- [ ] Sprint sin incidencias: el informe y el burndown explican por qué no hay datos

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Tablero: desplazamiento horizontal controlado de columnas en 390; cabeceras legibles
- [ ] Calles contraídas y expandidas no solapan tarjetas
- [ ] Backlog: sin pérdida de acciones en 390
- [ ] Indicador de WIP lleno distinguible sin depender solo del color

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa
- [ ] Playwright: specs de tablero, backlog, sprints y versiones pasan
