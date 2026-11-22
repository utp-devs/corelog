# QA Checklist - Fase F3, issues ricos

## Entorno

- Rama: `feat/roadmap-clean`
- Fecha: 2026-11-22

## Migraciones

- [ ] 000006 agrega issue_type, parent_id, due_date, story_points, rank y closed_at
- [ ] Los issues existentes reciben issue_type task y rank m
- [ ] El down revierte solo los metadatos de F3

## API

- [ ] POST /issues acepta issue_type y metadatos iniciales
- [ ] PATCH /issues/{id}/metadata valida issue_type
- [ ] story_points rechaza valores negativos
- [ ] parent_id acepta UUID valido
- [ ] due_date acepta RFC3339
- [ ] Transiciones de estado mantienen closed_at

## Frontend

- [ ] Crear incidencia permite elegir tipo
- [ ] Detalle permite elegir tipo, fecha limite y puntos
- [ ] Guardar metadatos actualiza la incidencia
- [ ] No quedan rutas /tickets visibles
- [ ] Crear comentario valida longitud y registra autor
- [ ] Eliminar comentario propio oculta el registro
- [ ] Crear y eliminar etiquetas desde la configuración de proyecto
- [ ] Crear y eliminar componentes desde la configuración de proyecto
- [ ] Crear vínculo blocks/relates/duplicates entre dos incidencias
- [ ] No permitir vincular una incidencia consigo misma
- [ ] Eliminar vínculo elimina solo la relación
- [ ] Mover incidencia arriba cambia su rank y persiste tras recargar
- [ ] Mover incidencia abajo cambia su rank y persiste tras recargar
- [ ] Buscar texto coincide en título o descripción
- [ ] Combinar estado, prioridad, tipo y fechas devuelve una lista consistente
- [ ] Editar título, descripción, prioridad y metadatos desde el detalle
- [ ] Un colaborador del proyecto puede editar; un lector y un no miembro reciben 403
- [ ] Estado, asignacion, metadatos, ranking, comentarios y vinculos validan rol del proyecto
- [ ] Un miembro solo lista y consulta issues de sus proyectos; admin puede consultar todos
- [ ] Migración 0010 crea workflow por defecto y transiciones
- [ ] Transición fuera del workflow devuelve 409
- [ ] Rol no permitido para la transición devuelve 409
- [ ] Las menciones @usuario detectan miembros del proyecto
- [ ] Actividad de issue lista cambios append-only
- [ ] Un colaborador puede cargar un PDF, imagen o texto de hasta 10 MiB
- [ ] Rechazar archivos vacios, mayores a 10 MiB y tipos no permitidos (incluido SVG/HTML)
- [ ] El nombre presentado no permite rutas y se conserva como nombre descargable
- [ ] Solo miembros con permiso de lectura listan y descargan; no miembros reciben 403
- [ ] Solo colaboradores/lideres o admin cargan; viewer recibe 403
- [ ] Un ID de adjunto de otro issue no permite descargarlo (404)
- [ ] Los bytes permanecen disponibles tras reiniciar el contenedor API (volumen Docker)
- [ ] Si falla la insercion de metadatos, se elimina el archivo guardado
- [ ] El detalle presenta cuatro pestanas: Detalle, Comentarios, Actividad y Archivos
- [ ] Cada lista de colaboracion muestra carga, error y estado vacio
- [ ] Cambiar de incidencia no muestra comentarios, actividad ni archivos de la anterior
- [ ] Un miembro del proyecto puede seguir y dejar de seguir; la operacion es idempotente y el contador coincide
- [ ] El estado de seguimiento de un issue ajeno responde 403/404 segun visibilidad
- [ ] Comentar agrega al autor a watchers dentro de la transaccion del comentario
- [ ] Asignar agrega al responsable como watcher dentro de la transaccion de asignacion
- [ ] La migracion 000012 crea watchers y su down elimina solo esa tabla
- [ ] Guardar un filtro conserva los criterios activos y aparece al volver a cargar el listado
- [ ] Cargar un filtro restaura estado, prioridad, tipo, texto y fechas
- [ ] Los filtros guardados son privados por usuario y no se pueden borrar entre usuarios
- [ ] No se permiten nombres vacios, duplicados por usuario ni criterios invalidos
- [ ] La migracion 000013 crea `saved_filters` y su down elimina solo esa tabla

## Errores tipados y autorizacion

- [ ] Sin bearer token, los endpoints privados de issues, adjuntos, watchers y filtros guardados responden 401
- [ ] Un usuario sin acceso al proyecto recibe 403 al consultar issues, comentarios, adjuntos o watchers
- [ ] Un viewer no puede modificar issues, comentar, asignar, gestionar vínculos ni subir adjuntos (403)
- [ ] IDs inexistentes o recursos de otro propietario responden 404 sin revelar datos ajenos
- [ ] Una transición invalida del workflow y un nombre de filtro duplicado responden 409
- [ ] Payloads invalidos de issue, comentario, adjunto y filtro guardado responden 422
- [ ] Los errores usan el envoltorio `{ "error": { "message": "..." } }`

## Automatizacion

- [x] `go test ./...`
- [x] `go vet ./...`
- [ ] `make test-coverage`
- [x] `npm run lint`
- [x] `npm run build`
- [x] `npm test`
- [x] `postman spec lint --issueType=syntax --no-report-events openapi.yaml`
