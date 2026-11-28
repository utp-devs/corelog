# QA Checklist - paneles, hoja de ruta y calendario

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. Hay un panel compartido "Resumen del equipo" con seis gadgets, eventos de calendario (una serie semanal con una ocurrencia cancelada, un evento de todo el día y uno de proyecto con invitado) y dos épicas de SCRUM con fechas.
- Cuentas: `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test`, y `admin@corelog.local`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Mi resumen y paneles

- [ ] `/dashboard` muestra mi panel; un usuario nuevo recibe uno predeterminado
- [ ] Crear un panel con nombre y distribución; un nombre vacío se rechaza
- [ ] Añadir cada uno de los 13 gadgets: asignadas a mí, creadas por mí, resultados de un filtro, estadísticas, circular, dos dimensiones, creadas frente a resueltas, burndown del sprint, flujo de actividad, proyectos recientes, favoritos, texto y próximos eventos
- [ ] Cada gadget tiene su estado vacío, de carga y de error
- [ ] Un panel admite hasta 20 gadgets; el 21 se rechaza con mensaje
- [ ] Configurar un gadget con un filtro guardado y con una consulta CoreQL; una consulta no válida se rechaza
- [ ] El gadget de texto admite Markdown y no ejecuta HTML ni scripts
- [ ] Mover gadgets entre columnas y guardar; el orden persiste
- [ ] Duplicar un panel crea una copia editable e independiente
- [ ] Eliminar un panel pide confirmación; el panel predeterminado no se puede eliminar o se reasigna
- [ ] **Compartir:** con una persona, un equipo, un proyecto y la organización, con permiso ver o editar
- [ ] Quien tiene permiso de ver no ve controles de edición y un PUT responde 403
- [ ] Un gadget muestra solo incidencias visibles para quien mira (comparar el panel compartido con `owner` y con `viewer`)
- [ ] `guest@corelog.test` no ve el panel compartido con la organización salvo acceso explícito, o lo ve sin datos de proyectos sin acceso

## Hoja de ruta (`/projects/SCRUM/roadmap`)

- [ ] Muestra las dos épicas con barras según sus fechas de inicio y límite
- [ ] Agrupar por épica, versión y sprint cambia las filas
- [ ] Filtrar por todas, abiertas y terminadas
- [ ] Una épica sin fechas propias deduce el rango de sus hijas y se marca como derivada; sin hijas con fechas, "sin planificar"
- [ ] Las dependencias entre épicas aparecen como flechas y salen de vínculos de bloqueo
- [ ] El lead reordena las épicas y el orden persiste; un contributor no puede (403)
- [ ] Zoom y desplazamiento horizontal; "Hoy" marcado
- [ ] Un proyecto sin épicas muestra el estado vacío (EMP)
- [ ] Un proyecto privado: `member@corelog.test` recibe 404 en la hoja de ruta de SEC

## Calendario (`/calendar`)

- [ ] Vistas mes, semana, día y agenda; navegar entre periodos y volver a hoy
- [ ] Se muestran los sprints, versiones y fechas límite de incidencias visibles, y no los de proyectos sin acceso
- [ ] Crear un evento con título, fechas y hora, lugar, color y descripción; título vacío o fin anterior al inicio se rechaza
- [ ] Evento de todo el día: se muestra en la franja superior y el fin es exclusivo
- [ ] Zonas horarias: un evento en `Europe/Madrid` se muestra a la hora local correcta y sobrevive al cambio de hora
- [ ] Visibilidad privada, de proyecto y de organización: comprobar con otra cuenta quién lo ve
- [ ] Invitados: añadir, quitar y responder (acepto, rechazo, tal vez); solo el invitado cambia su respuesta (`calendar.not_attendee` para los demás)
- [ ] Más de 100 invitados se rechaza

### Recurrencia

- [ ] Diaria, semanal con días, mensual por fecha y por posición (segundo lunes, último viernes) y anual
- [ ] Intervalo, número de repeticiones (hasta 5000) y fecha final; combinar repeticiones y fecha final se rechaza
- [ ] Una regla no admitida (`FREQ=HOURLY`) se rechaza con `calendar.invalid_recurrence`
- [ ] La ocurrencia cancelada de la serie sembrada no aparece
- [ ] Editar solo una ocurrencia crea una excepción; editar la serie cambia todas
- [ ] Eliminar solo una ocurrencia, y toda la serie
- [ ] Una ventana con más de 1000 ocurrencias se recorta sin errores

### Suscripción ICS

- [ ] Crear el enlace privado: se muestra la URL una sola vez
- [ ] Abrir la URL (sin sesión) descarga un archivo `.ics` válido (probar importándolo en una aplicación de calendario)
- [ ] El archivo incluye los eventos visibles de 30 días atrás y 90 adelante, y las series recurrentes con `RRULE` y `EXDATE`
- [ ] Rotar el enlace invalida el anterior (404)
- [ ] Exportar `.ics` desde el calendario descarga el periodo visible
- [ ] Un evento privado de otra persona no está en mi archivo

## Roles y permisos

- [ ] Propietario y administrador ven todo lo de la organización
- [ ] Un viewer ve paneles y calendario; edita solo lo que posee o comparte con permiso de editar
- [ ] Sin organización (`newbie@corelog.test`): sin datos de otras organizaciones

## Estados de error y vacíos

- [ ] Sin eventos en el periodo: mensaje vacío con acción de crear
- [ ] Error del servidor en un gadget no rompe el resto del panel

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Paneles: de varias columnas a una en 390
- [ ] Hoja de ruta: desplazamiento horizontal con cabecera fija legible
- [ ] Calendario: la vista de mes en 390 pasa a una presentación legible (agenda o puntos)
- [ ] Eventos y barras con contraste suficiente en oscuro; no dependen solo del color
- [ ] Diálogos de evento y de compartir caben en 390

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa (incluye las pruebas de recurrencia e ICS)
- [ ] Playwright: specs de paneles, hoja de ruta y calendario pasan
