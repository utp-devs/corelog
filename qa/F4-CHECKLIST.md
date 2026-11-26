# QA Checklist - tableros y backlog

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

## CORE-050 - Vista de tablero por proyecto

- [x] El endpoint autenticado devuelve el tablero predeterminado del proyecto
- [x] El acceso se valida con la política de lectura existente del proyecto (401/403/404)
- [x] Columnas se devuelven en orden y las tarjetas se agrupan por estado y rango
- [x] Cada columna expone su conteo, incluso cuando no tiene tarjetas
- [x] Las tarjetas incluyen tipo, resumen, responsable legible, prioridad y puntos
- [x] La UI permite abrir el tablero desde la tarjeta del proyecto y se adapta a pantallas pequeñas
- [x] La prueba PostgreSQL valida tarjetas, responsable, puntos, columnas vacías y tablero inexistente
- [x] Smoke test con Compose crea un proyecto, obtiene sus cuatro columnas vacías y confirma 401 sin sesión
- [ ] QA manual visual en desktop y móvil
- [ ] QA manual de acceso 403/404 con usuarios/proyectos restringidos
