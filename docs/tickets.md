# Módulo de tickets
Permite registrar incidencias y darles seguimiento hasta su cierre.

## Estados
open → in_progress → resolved → closed

resolved → in_progress

## Prioridades
low, medium, high, critical

## Funcionalidades
- Crear un ticket
- Listar tickets con filtros por estado y prioridad
- Buscar tickets por título en el listado
- Consultar el detalle de un ticket
- Asignar un ticket a un usuario de su mismo equipo
- Cambiar el estado de un ticket

## Endpoints principales
- POST /api/v1/tickets
- GET /api/v1/tickets
- GET /api/v1/tickets/{id}
- PATCH /api/v1/tickets/{id}/status
- PATCH /api/v1/tickets/{id}/assign

## Cómo ejecutarlo
docker compose up -d --build
Luego abrir http://localhost:5173, registrar una cuenta e ir a Entradas.