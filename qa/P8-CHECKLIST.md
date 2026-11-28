# QA Checklist - colaboración (notas y comentarios)

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. Las incidencias 2 y 4 de SCRUM traen comentarios con respuesta, mención, nota interna y reacciones.
- Cuentas: `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test` y `agent@corelog.local`, `admin@corelog.local`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Notas personales (`/notes`)

- [ ] Crear una nota con título y texto; aparece al principio de la lista
- [ ] Crear una nota sin título o sin texto se permite o se rechaza según el formulario, y el mensaje es claro
- [ ] Un título de más de 120 caracteres o un texto de más de 20 000 se rechaza
- [ ] Editar y guardar; la nota conserva el formato
- [ ] Etiquetas: hasta 10 de hasta 30 caracteres; la undécima o una más larga se rechaza; las repetidas se unifican
- [ ] Filtrar por etiqueta y buscar por texto
- [ ] Elegir un color de la paleta y verlo en la tarjeta y en el editor
- [ ] Fijar y desfijar: las fijadas quedan arriba
- [ ] Enlazar una nota a una incidencia con el selector; un viewer puede enlazar solo incidencias que ve
- [ ] Eliminar una nota pide confirmación y no se puede deshacer
- [ ] Atajo `n` abre las notas rápidas
- [ ] Privacidad: `member@corelog.test` no ve las notas de `lead@corelog.test`; la API responde 404 al pedir una nota ajena

## Comentarios y menciones

- [ ] Escribir `@` abre el selector de personas del proyecto; solo ofrece a quien puede leer la incidencia
- [ ] La mención se muestra con el nombre y la persona recibe un aviso con enlace al comentario
- [ ] Mencionar en un comentario interno solo avisa a quien puede leerlo
- [ ] Una mención dentro de un bloque de código no genera aviso
- [ ] La campana cuenta el aviso y desaparece al leerlo
- [ ] Responder a un comentario avisa a su autor; responder a una respuesta se agrupa bajo el mismo comentario raíz
- [ ] El historial de una incidencia con muchos comentarios carga por tramos ("Ver más") sin repetir ni saltar comentarios

## Reacciones

- [ ] Las seis reacciones (pulgar arriba, corazón, celebración, ojos, cohete, verificación) se pueden añadir y quitar
- [ ] No se pueden duplicar: pulsar dos veces la misma la quita
- [ ] Pasar el cursor o enfocar una reacción muestra quién reaccionó
- [ ] Un viewer no puede reaccionar (o sí, según el esquema del proyecto) y la interfaz coincide con la API

## Edición y revisiones

- [ ] El autor edita su comentario; aparece "editado" y la fecha
- [ ] Se puede ver el historial de revisiones del comentario con autor y hora
- [ ] Otra persona no puede editarlo (403) salvo permiso explícito
- [ ] Eliminar deja un marcador y conserva las respuestas

## Comentarios internos

- [ ] Marcar un comentario como interno pide el rol mínimo (contributor o lead)
- [ ] `member@corelog.test` (contributor) y `lead@corelog.test` ven un comentario interno de contributor; `viewer@corelog.test` y `guest@corelog.test` no lo ven, ni en el detalle, ni en el historial, ni en los avisos
- [ ] Un comentario interno de nivel lead no lo ve un contributor
- [ ] Un viewer no puede crear comentarios internos
- [ ] Un comentario interno no aparece en exportaciones, notificaciones por correo ni en gadgets de actividad para quien no puede leerlo

## Seguidores y actividad

- [ ] Seguir una incidencia añade al seguidor; recibe avisos según el esquema de notificaciones
- [ ] El flujo de actividad ordena comentarios, cambios y registros de tiempo por fecha

## Adjuntos

- [ ] Adjuntar un archivo; superar el tamaño máximo o un tipo no permitido muestra el error
- [ ] Descargar un adjunto con una cuenta sin acceso responde 404
- [ ] Eliminar un adjunto propio y, como lead, uno ajeno

## Roles y permisos

- [ ] Contributor y lead comentan; viewer e invitado viewer no ven el editor
- [ ] Un lead no puede leer notas ajenas
- [ ] Sin organización: 404 en todo lo del proyecto

## Estados de error y vacíos

- [ ] Sin notas: estado vacío con acción de crear
- [ ] Guardar un comentario sin red conserva el texto y permite reintentar
- [ ] Un comentario muy largo (más de 20 000 caracteres) se rechaza con mensaje

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Cuadrícula de notas: de varias columnas a una en 390
- [ ] Editor de notas y de comentarios con la barra de formato accesible por teclado
- [ ] Paleta de colores de notas legible en oscuro (texto sobre cada color con contraste suficiente)
- [ ] Reacciones y menciones no desbordan el hilo en 390

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa
- [ ] Playwright: specs de notas y comentarios pasan
