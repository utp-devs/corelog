# QA Checklist - proyectos y permisos

## Entorno

- Rama: `feat/f2-projects`
- Fecha: 2026-11-21

## Migraciones

- [ ] PostgreSQL aplica 000005 y crea proyectos, miembros y issues
- [ ] PostgreSQL down restaura tickets y elimina las tablas nuevas
- [ ] team_members contiene el backfill de users.team_id

## Backend

- [ ] POST /projects como admin devuelve 201
- [ ] POST /projects como usuario no admin devuelve 403
- [ ] POST /projects/{id}/members permite agregar miembros solo a admin o lead
- [ ] PATCH /projects/{id}/members/{user_id} valida lead/contributor/viewer
- [ ] DELETE /projects/{id}/members/{user_id} permite solo admin o lead
- [ ] El policy permite view a miembros y deniega acciones a viewer
- [ ] Clave duplicada devuelve 409
- [ ] Datos invalidos devuelven 422
- [ ] GET /projects devuelve el listado visible
- [ ] PATCH /projects/{id} modifica el proyecto
- [ ] DELETE /projects/{id} elimina el proyecto disponible
- [ ] El flujo actual de tickets sigue verde tras renombrar tickets a issues
- [x] API publica `/issues` sin alias `/tickets`
- [x] Frontend navega a `/issues`, `/issues/new` y `/issues/{id}`
- [x] Textos visibles usan "Incidencias" y "Nueva incidencia"
- [x] Middleware de proyecto aplica rol lead/contributor/viewer a CRUD y miembros

## Frontend

- [ ] Ruta /projects visible segun el rol
- [ ] Crear proyecto valida clave, nombre y tipo
- [ ] Las tarjetas muestran clave, nombre, tipo y visibilidad
- [ ] Seleccionar un proyecto permite listar y administrar miembros
- [ ] Usuario no admin no puede crear proyectos

## Automatizacion

- [x] `go test ./...`
- [ ] `make test-coverage`
- [x] `npm run lint`
- [x] `npm run build`
- [x] `npm test`
- [x] Pruebas de la matriz de permisos por proyecto
