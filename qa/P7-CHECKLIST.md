# QA Checklist - búsqueda y CoreQL

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. Hay filtros sembrados: "Pendientes de la organización" (compartido con la organización) y "Mis pendientes".
- Cuentas: `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test`, y `admin@corelog.local`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Búsqueda global

- [ ] `/` y `Ctrl+K` abren la búsqueda; Escape la cierra y devuelve el foco
- [ ] Menos de dos caracteres no busca y lo indica
- [ ] Busca proyectos por nombre y clave, e incidencias por título, descripción y clave (`SCRUM-2`)
- [ ] Los resultados se navegan con flechas y se abren con Intro
- [ ] `member@corelog.test` no ve resultados de SEC; `guest@corelog.test` solo los de KAN; `newbie@corelog.test` nada
- [ ] Sin resultados: mensaje claro con sugerencia

## Gramática CoreQL

- [ ] `project = SCRUM` y `status = "En curso"` devuelven lo esperado
- [ ] `AND`, `OR`, `NOT` y paréntesis respetan la precedencia (NOT, luego AND, luego OR)
- [ ] `priority IN (Alta, Crítica)` y `NOT IN`
- [ ] `assignee IS EMPTY`, `IS NOT EMPTY` y `IS NULL`
- [ ] `summary ~ "portal"` contiene y `summary !~ "portal"` no contiene
- [ ] Palabras clave y nombres de campo en mayúsculas o minúsculas funcionan igual
- [ ] Un campo personalizado `cf[severidad] = Alta` filtra; `cf[inexistente]` da error claro
- [ ] Fechas absolutas `created >= 2026-11-01` y relativas `created >= -7d`; una fecha mal escrita da error
- [ ] Los operadores no válidos para el tipo (`summary > 3`) dan error con el mensaje en español
- [ ] `ORDER BY priority ASC, updated DESC` ordena; ordenar por un campo no ordenable da error
- [ ] Una consulta vacía devuelve todo lo visible
- [ ] Una consulta con error de sintaxis muestra la posición del error y qué se esperaba, y no devuelve resultados parciales

## Funciones

- [ ] `assignee = currentUser()` devuelve las incidencias de quien consulta (probar con dos cuentas)
- [ ] `assignee IN membersOf("Platform")` devuelve las del equipo; un equipo inexistente da error
- [ ] `due < startOfDay()`, `updated >= startOfWeek()`, `created >= startOfMonth(-1)` y `endOfDay()`
- [ ] `sprint IN openSprints()` devuelve las del Sprint 1; `closedSprints()` las del Sprint 0; `futureSprints()` ninguna
- [ ] `fixVersion IN unreleasedVersions()` y `releasedVersions()` devuelven las de 1.1 y 1.0
- [ ] `project = currentProject()` dentro de un proyecto

## Navegador de incidencias

- [ ] Editor con resaltado y sugerencias de campos, operadores, valores y funciones
- [ ] Los errores se subrayan en la posición exacta, también con tildes y emojis antes del error
- [ ] Columnas y orden configurables y persistentes al recargar
- [ ] Paginación: cambiar de página y de tamaño mantiene la consulta
- [ ] La URL contiene la consulta y se puede compartir; abrirla con otra cuenta aplica sus permisos
- [ ] Alternar entre el modo básico (filtros) y el modo avanzado (CoreQL) conserva la intención

## Filtros guardados y suscripciones

- [ ] Guardar una consulta como filtro con nombre y descripción; un nombre repetido del mismo dueño se rechaza
- [ ] Renombrar, editar la consulta y eliminar
- [ ] Compartir con una persona (ver y editar), con un equipo, con un proyecto y con la organización
- [ ] Quien recibe el filtro en modo ver no puede editarlo (403); en modo editar sí
- [ ] Un filtro compartido muestra solo lo que el receptor puede ver, no lo del dueño
- [ ] Suscribirse con frecuencia diaria y semanal; cambiar la frecuencia y cancelar
- [ ] La suscripción llega como aviso y correo a la hora indicada (probar con el trabajo programado en un entorno de pruebas)
- [ ] Una persona que pierde acceso al filtro deja de recibir la suscripción

## Roles y permisos

- [ ] Todos los roles buscan solo en lo que ven; el resultado de un mismo filtro cambia según la cuenta
- [ ] Un viewer puede buscar, guardar sus propios filtros y suscribirse
- [ ] Un token de API con alcance `read` puede buscar (`GET /issues/search`)

## Rendimiento y límites

- [ ] Con el escenario `scale` (50 000 incidencias, solo en una base de pruebas) una búsqueda con filtro responde en menos de 1 segundo
- [ ] Una consulta de más de 2000 caracteres se rechaza
- [ ] Una consulta con muchos `OR` anidados no tumba el servidor y responde error o resultado

## Estados de error y vacíos

- [ ] Sin resultados: estado vacío con acción de limpiar filtros
- [ ] Error del servidor: aviso con reintento y la consulta conservada

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Editor CoreQL y lista de sugerencias caben en 390 y se pueden usar con teclado
- [ ] Tabla de resultados con columnas configurables sin desbordar
- [ ] Contraste del resaltado de sintaxis y del subrayado de error en oscuro

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa (incluye las pruebas de gramática y de fuzz)
- [ ] Playwright: specs de búsqueda y filtros pasan
