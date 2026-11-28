# QA Checklist - flujos de trabajo

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. Cuentas: `owner`, `orgadmin`, `lead` (lead de SCRUM), `member`, `viewer`, `guest`, `newbie` de `@corelog.test`.
- Proyectos: SCRUM (Scrum), KAN (Kanban), SEC (privado), EMP (seguimiento de tareas).
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Catálogo de estados de la organización

- [ ] El administrador crea un estado con nombre y categoría (por hacer, en curso, hecho), lo renombra y lo reordena
- [ ] Un nombre repetido se rechaza
- [ ] Cambiar la categoría de un estado en uso actualiza dónde cuenta en tableros e informes
- [ ] Eliminar un estado en uso en algún flujo se rechaza con un mensaje que indica dónde se usa
- [ ] Un miembro común no accede a la pantalla (`/organization/statuses`)

## Editor de flujo

- [ ] El lead abre `/projects/SCRUM/workflow`; contributor y viewer no ven el editor o lo ven en solo lectura
- [ ] **Plantillas:** aplicar cada una de las cuatro plantillas muestra una vista previa y, al confirmar, reemplaza el flujo
- [ ] Añadir un estado nuevo y reordenar estados
- [ ] Añadir una transición con varios estados de origen y uno de destino
- [ ] Una transición hacia su mismo estado o repetida se rechaza
- [ ] Sin estado inicial, o con un estado inalcanzable, el editor muestra el error por campo y no guarda
- [ ] **Restablecer** devuelve el flujo de la plantilla tras confirmar
- [ ] Guardar con cambios pendientes muestra confirmación al salir sin guardar

## Condiciones

- [ ] "Usuario con rol": solo ese rol ve y ejecuta la transición (probar con lead y con contributor)
- [ ] "Es responsable" y "Es informador": solo esa persona la ve
- [ ] "Permiso": ve la transición quien tiene el permiso del esquema del proyecto
- [ ] "Campo igual a": la transición solo está disponible cuando el campo tiene el valor
- [ ] Una transición no disponible no aparece en el selector ni en el tablero y la API responde 403 o 422 si se fuerza

## Validadores

- [ ] Campo obligatorio: la transición se rechaza con el campo señalado
- [ ] Comentario obligatorio: el diálogo pide un comentario y lo guarda
- [ ] Resolución obligatoria: el diálogo pide la resolución
- [ ] Subtareas terminadas: se rechaza mientras haya subtareas abiertas y el mensaje lo dice
- [ ] Responsable asignado: se rechaza sin responsable
- [ ] Todos los errores de varios validadores se muestran juntos

## Funciones posteriores

- [ ] Asignar al informador, a quien ejecuta, al lead o dejar sin asignar tras la transición
- [ ] Fijar y quitar la resolución: pasar a un estado de categoría "hecho" asigna resolución; reabrir la quita
- [ ] Fijar un campo y añadir un comentario automático (el comentario aparece con el autor correcto)
- [ ] Una función posterior que falla no deja la incidencia a medias (la transición se revierte)

## Migración de incidencias al quitar un estado

- [ ] Quitar un estado con incidencias abre el diálogo de correspondencia; sin asignar destino no se guarda
- [ ] Tras confirmar, las incidencias están en el estado destino y el historial lo registra
- [ ] Quitar un estado sin incidencias no pide correspondencia
- [ ] Una transición en curso por otra persona durante la migración no pierde cambios

## Tablero y selector de estado

- [ ] Arrastrar una tarjeta solo ofrece columnas con transición válida
- [ ] Soltar en una columna que exige datos abre el diálogo y cancelar deja la tarjeta donde estaba
- [ ] Un viewer solo cambia el estado de una incidencia de la que es responsable, si el flujo lo permite
- [ ] El selector de estado del detalle muestra el nombre de la transición y el estado destino

## Roles y permisos

- [ ] Propietario y administrador editan el flujo en cualquier proyecto
- [ ] Lead de SCRUM edita el flujo de SCRUM y no el de KAN
- [ ] Contributor y viewer reciben 403 al guardar el flujo
- [ ] Invitado y sin organización: 404

## Estados de error

- [ ] Dos personas editan el flujo a la vez: la segunda recibe el conflicto y recarga
- [ ] Error de red al guardar: se conservan los cambios en el editor

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Editor: lista de estados y tabla de transiciones sin desbordamiento horizontal oculto
- [ ] Diálogo de plantillas y de correspondencia caben en 390
- [ ] Categorías de estado distinguibles por texto y no solo por color

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa
- [ ] Playwright: specs de flujos y tablero pasan
