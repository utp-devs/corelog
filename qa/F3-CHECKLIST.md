# QA Checklist - Fase F3, issues ricos

## Entorno

- Rama: `feat/f3-rich-issues`
- Fecha: 2026-11-21

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
- [ ] Migración 0010 crea workflow por defecto y transiciones
- [ ] Transición fuera del workflow devuelve 409
- [ ] Rol no permitido para la transición devuelve 409
- [ ] Las menciones @usuario detectan miembros del proyecto
- [ ] Actividad de issue lista cambios append-only

## Automatizacion

- [x] `go test ./...`
- [x] `go vet ./...`
- [ ] `make test-coverage`
- [x] `npm run lint`
- [x] `npm run build`
- [x] `npm test`
