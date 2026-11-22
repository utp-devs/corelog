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
- [ ] Las menciones @usuario detectan miembros del proyecto
- [ ] Actividad de issue lista cambios append-only

## Automatizacion

- [x] `go test ./...`
- [x] `go vet ./...`
- [ ] `make test-coverage`
- [x] `npm run lint`
- [x] `npm run build`
- [x] `npm test`
