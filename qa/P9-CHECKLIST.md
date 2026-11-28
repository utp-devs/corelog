# QA Checklist - tiempo e informes

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. SCRUM trae el Sprint 0 cerrado (comprometido 18 puntos, completado 10), el Sprint 1 activo y tiempo registrado en las incidencias 2, 4 y 10.
- Cuentas: `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Registro de tiempo

- [ ] Duraciones válidas: `30m`, `1h 30m`, `2d`, `1w`, `1w 2d 3h 30m`, `1H` (mayúsculas), `1h30m` (sin espacio)
- [ ] Duraciones no válidas: vacío, `0m`, `1.5h`, `-1h`, `10`, `1h 1h`, `3x`; todas muestran el error en el campo
- [ ] Un registro de más de 3 días laborales se rechaza
- [ ] Un inicio a más de 24 horas en el futuro se rechaza; un inicio en el pasado se admite
- [ ] Los modos de tiempo restante (automático, fijar, reducir, dejar igual) dan el resultado esperado y el restante nunca es negativo
- [ ] Una organización con jornada distinta (por ejemplo 6 horas) convierte `1d` a 6 horas
- [ ] La estimación original y el restante se muestran con las mismas unidades
- [ ] Editar y eliminar un registro recalcula el total de la incidencia
- [ ] Un viewer no puede registrar tiempo (403)

## Hoja de horas (`/timesheet`)

- [ ] Muestra los registros de la semana por día con totales; cambiar de semana funciona
- [ ] Los días respetan la zona horaria de la organización (un registro a las 23:30 locales no cae en el día siguiente)
- [ ] Solo lista los registros de la persona; no filtra los de otras
- [ ] Crear un registro desde la hoja lo refleja en la incidencia

## Resumen de tiempo del proyecto

- [ ] Agrupa por persona, por incidencia y por día; los totales de cada vista coinciden
- [ ] Respeta el rango de fechas elegido
- [ ] Un contributor ve solo lo que su rol permite

## Los once informes

Para cada informe: abre sin errores, muestra gráfica y tabla equivalente, respeta el rango (30 días por defecto, máximo 366), el filtro CoreQL y la agrupación, y el estado vacío es comprensible.

- [ ] **Burndown** del Sprint 1: línea ideal y real; sin fechas muestra la explicación
- [ ] **Burnup**: alcance y completado
- [ ] **Velocidad**: muestra el Sprint 0 con 18 comprometidos y 10 completados; el límite de sprints acepta de 1 a 20
- [ ] **Informe del sprint** del Sprint 0: completado, no completado, añadido y retirado, con puntos
- [ ] **Flujo acumulado**: bandas por estado
- [ ] **Tiempo de ciclo**: media y dispersión
- [ ] **Creadas frente a resueltas**: intervalos día, semana y mes
- [ ] **Tiempo de resolución**: por agrupación
- [ ] **Seguimiento de tiempo**: estimado, registrado y restante coinciden con las incidencias 2, 4 y 10
- [ ] **Carga de trabajo**: pendiente por responsable
- [ ] **Versión**: pide elegir la versión; 1.0 y 1.1 muestran su progreso

## Validación de parámetros

- [ ] Fecha inicial posterior a la final: error `report.range_invalid`
- [ ] Rango de más de 366 días: `report.range_too_large`
- [ ] Parámetro no admitido (agrupación o intervalo): `report.param_invalid`
- [ ] Informe de sprint sin sprint o con un sprint de otro proyecto: error y 404
- [ ] Informe de versión sin versión: `report.version_required`
- [ ] Filtro CoreQL no válido: `report.filter_invalid`; filtro que abarca demasiado: aviso `report.filter_too_broad`
- [ ] Un tipo de informe desconocido: `report.kind_unknown` (404)

## Roles y permisos

- [ ] Viewer y contributor consultan informes; los datos solo incluyen incidencias visibles para ellos
- [ ] Invitado sin acceso: 404; sin organización: 404
- [ ] Un token de API `read` consulta informes

## Estados de error y vacíos

- [ ] Proyecto EMP: cada informe muestra un estado vacío con explicación, sin errores
- [ ] Error del servidor: aviso con reintento y los parámetros conservados

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Gráficas legibles en oscuro (líneas y bandas distinguibles, ejes con contraste)
- [ ] En 390 la barra de parámetros se pliega y la tabla se desplaza sin romper la página
- [ ] Las series no dependen solo del color (leyenda y etiquetas)
- [ ] La hoja de horas es utilizable con teclado

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa
- [ ] Playwright: specs de informes y hoja de horas pasan
