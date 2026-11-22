# QA Checklist - Fase F4, tableros y backlog

## Entorno

- Rama: `feat/roadmap-clean`
- Fecha: 2026-11-22

## CORE-049 - Tablero predeterminado por proyecto

- [x] La migración 000014 crea `boards` y `board_columns` con claves, restricciones e índices
- [x] Los proyectos existentes reciben exactamente un tablero predeterminado
- [x] Proyectos Kanban y Scrum nuevos reciben un tablero del mismo tipo
- [x] El tablero inicial mapea en orden abierto, en progreso, resuelto y cerrado
- [x] Un proyecto no puede tener dos tableros predeterminados
- [x] Si falla el alta del líder, no quedan proyecto, tablero ni columnas parciales
- [x] Eliminar un proyecto elimina sus tableros y columnas por cascada
- [x] La migración down elimina las tablas de tableros sin afectar proyectos ni issues

## Automatizacion

- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `CORELOG_TEST_DATABASE_URL` ejecuta la prueba PostgreSQL de provisionamiento de tableros
