# Guía de usuario de CoreLog

Esta guía describe cómo usar CoreLog según lo que puedes hacer en cada proyecto. Las reglas de permisos son las que aplica el servidor; la interfaz oculta o deshabilita lo que no puedes hacer, pero la decisión final siempre la toma la API.

## Roles

Hay un rol global y tres roles por proyecto.

- **Administrador global:** puede hacer todo en todos los proyectos.
- **Lead del proyecto:** gestiona el proyecto, sus miembros, el tablero, el flujo de estados y los sprints.
- **Contributor:** trabaja las incidencias del proyecto: las crea, las edita, las mueve por el tablero y comenta.
- **Viewer:** ve el proyecto y sus incidencias, y puede seguir incidencias. No puede modificar nada.

Quien no es miembro de un proyecto no lo ve: ni en el listado, ni en la búsqueda, ni en el calendario.

## Matriz de permisos

| Acción | Admin | Lead | Contributor | Viewer | Sin acceso |
|---|:-:|:-:|:-:|:-:|:-:|
| Ver proyecto e incidencias | Sí | Sí | Sí | Sí | No |
| Editar datos del proyecto | Sí | Sí | No | No | No |
| Eliminar proyecto | Sí | No | No | No | No |
| Gestionar miembros y roles | Sí | Sí | No | No | No |
| Crear y editar incidencias | Sí | Sí | Sí | No | No |
| Comentar y adjuntar | Sí | Sí | Sí | No | No |
| Seguir una incidencia | Sí | Sí | Sí | Sí | No |
| Mover tarjetas y ordenar el backlog | Sí | Sí | Sí | No | No |
| Gestionar tablero y límites WIP | Sí | Sí | No | No | No |
| Gestionar flujo de estados | Sí | Sí | No | No | No |
| Planificar, iniciar y cerrar sprints | Sí | Sí | No | No | No |

Nota: "Eliminar proyecto" está reservado al administrador en la matriz de permisos, aunque en la versión actual el lead puede borrarlo; esta diferencia está pendiente de decisión.

## Navegación

- **Incidencias:** listado con filtros y búsqueda guardada, y detalle de cada incidencia con comentarios, adjuntos, enlaces, etiquetas, historial y seguimiento.
- **Proyectos:** listado de tus proyectos. Desde cada proyecto accedes a su tablero, backlog, sprints y calendario.
- **Tablero:** columnas por estado. Arrastra una tarjeta a otra columna para cambiar el estado de la incidencia. Una columna con límite WIP lleno no acepta más tarjetas. Si eres lead, puedes fijar el límite de cada columna en su cabecera.
- **Backlog:** todas las incidencias del proyecto en orden de prioridad. Usa las flechas para subir o bajar una incidencia.
- **Sprints:** planifica un sprint con nombre, objetivo y fechas. Solo puede haber un sprint activo por proyecto. Al cerrar un sprint, las incidencias no terminadas pasan al backlog o a otro sprint planificado. El gráfico de burndown muestra el trabajo restante frente a la línea ideal, y necesita fechas de inicio y fin.
- **Calendario:** inicios y fines de sprint, y fechas límite de incidencias del mes. Puedes verlo en cuadrícula o en lista.
- **Mi resumen:** tus incidencias abiertas, las vencidas, tus proyectos, los sprints activos de esos proyectos y las incidencias más urgentes.
- **Búsqueda:** en la cabecera, busca proyectos (por nombre o clave) e incidencias (por título o descripción). Solo aparece lo que puedes ver. Escribe al menos dos caracteres.
- **Notificaciones:** la campana de la cabecera muestra cuántos avisos tienes sin leer. Se actualiza cada 30 segundos. Recibes un aviso cuando te asignan una incidencia.

## Tareas frecuentes

### Asignar una incidencia

Un contributor o lead asigna a alguien de su mismo equipo. La persona asignada recibe una notificación.

### Planificar y cerrar un sprint

1. Como lead, crea el sprint en la pantalla de sprints con nombre, objetivo y fechas.
2. Añade incidencias desde el backlog.
3. Inicia el sprint. Si ya hay otro activo en el proyecto, debes cerrarlo primero.
4. Al cerrarlo, elige qué hacer con lo no terminado: volver al backlog o pasar a un sprint planificado del mismo proyecto.

### Cambiar el límite WIP de una columna

Como lead, escribe el número en el campo de la cabecera de la columna y pulsa Intro o sal del campo. Deja el campo vacío para quitar el límite. El límite debe ser un entero mayor que cero.

### Cambiar el flujo de estados

Desde la API, el lead puede reemplazar las transiciones del flujo por defecto. Cada transición indica los roles de proyecto que pueden ejecutarla. No es posible saltar de un estado a sí mismo ni repetir una transición.

## Errores frecuentes

- **Prohibido:** no tienes el rol que requiere la acción. Pide a un lead que te cambie el rol o que haga la acción.
- **Conflicto (409):** la acción no es válida en el estado actual. Por ejemplo, la columna destino tiene su límite WIP lleno, el sprint no está planificado, o ya hay otro sprint activo.
- **Datos no válidos (422):** revisa las fechas (formato AAAA-MM-DD, y el fin no puede ser anterior al inicio) y los campos obligatorios.
- **No encontrado (404):** el recurso no existe o no pertenece al proyecto indicado.
