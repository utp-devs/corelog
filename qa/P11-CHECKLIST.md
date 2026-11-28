# QA Checklist - notificaciones, automatización e integraciones

## Entorno

- Stack local y datos del escenario `demo` (`SEED_PASSWORD=<contraseña> go run ./cmd/seed --scenario demo --force-reset`, solo en una base de pruebas). El escenario demo trae, en el proyecto CORE: un esquema de notificaciones propio, una regla de automatización desactivada y un webhook de ejemplo desactivado que apunta a `https://example.com/hooks/corelog`. El escenario e2e no los trae.
- Para el correo, usa un servidor SMTP de pruebas (por ejemplo MailHog) con `SMTP_HOST`, o deja `SMTP_HOST` vacío y revisa el registro de la API.
- Para los webhooks, un receptor de pruebas accesible desde la API (con `WEBHOOK_ALLOW_PRIVATE=true` solo en desarrollo).
- Cuentas demo: `admin@corelog.local` (propietario), `agent@corelog.local` (miembro), `requester@corelog.local` (invitado) y las personas `@corelog.test` del listado de la semilla.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Notificaciones en la aplicación

- [ ] Asignar una incidencia avisa al responsable en la campana; el contador sube y baja al leer
- [ ] Marcar un aviso como leído, marcar todos como leídos y abrir el enlace del aviso lleva a la incidencia
- [ ] El contador se actualiza solo (cada 30 segundos) sin recargar
- [ ] Quien hace una acción no recibe aviso de ella; al activar "Mis propias acciones" sí
- [ ] Un aviso de una incidencia a la que se perdió el acceso no filtra el título

## Preferencias personales (`/profile/notifications`)

- [ ] Desactivar "En la aplicación" deja de guardar avisos nuevos en la campana
- [ ] Correo inmediato: llega un correo por aviso
- [ ] Resumen diario: no llegan correos sueltos y a la hora configurada llega uno solo con los avisos
- [ ] Correo desactivado: no llega ninguno
- [ ] Los cambios persisten tras recargar y se aplican a la siguiente notificación

## Esquemas de notificaciones (interfaz y API)

- [ ] Esquema predeterminado: asignación, mención y respuesta avisan en la aplicación; el resto de eventos a nadie
- [ ] `GET /projects/{project}/notification-scheme` del proyecto CORE devuelve el esquema propio y lo marca como tal
- [ ] En CORE, un comentario avisa al informador, al responsable y a los seguidores por la aplicación y por correo
- [ ] Una transición en CORE avisa en la aplicación al responsable y a los seguidores; en otro proyecto no
- [ ] Publicar una versión de CORE avisa a los seguidores y al lead del proyecto
- [ ] `PUT` con un evento desconocido, un destinatario no válido o un canal desconocido responde 422 (`notification.scheme_invalid`)
- [ ] El esquema de la organización sustituye el predeterminado y el del proyecto sustituye al de la organización, evento por evento
- [ ] Solo un lead cambia el esquema del proyecto y solo un administrador el de la organización (403 al resto)

## Cola de correo y resumen (interfaz y API)

- [ ] Con SMTP configurado, los correos pasan de pendiente a enviado; un servidor caído los deja pendientes con reintentos y, tras agotarlos, marcados como fallidos
- [ ] `GET /admin/email-outbox` lo ve un administrador de la plataforma; un miembro recibe 403
- [ ] Reintentar un correo fallido lo vuelve a poner en cola
- [ ] El resumen no repite avisos ya incluidos en un resumen anterior
- [ ] El asunto y el cuerpo están en español, sin datos de proyectos a los que la persona no tiene acceso
- [ ] Los enlaces de los correos usan `APP_PUBLIC_URL`

## Automatización (interfaz y API)

- [ ] La regla de ejemplo de CORE aparece desactivada y válida
- [ ] Activarla (`POST /automation-rules/{id}/enable`) y pasar una incidencia de CORE a "Hecho": el informador recibe el aviso y la incidencia recibe el comentario con los valores inteligentes sustituidos
- [ ] Desactivarla detiene las ejecuciones
- [ ] `GET /automation-rules/{id}/runs` lista ejecuciones correctas, omitidas y fallidas con detalle
- [ ] Crear una regla por cada disparador: creación, edición (con campo concreto), transición (de y a), comentario, inicio y cierre de sprint y programado
- [ ] Una regla programada respeta el intervalo mínimo de 5 minutos y actúa como máximo sobre 50 incidencias
- [ ] Cada condición: consulta, campo igual, campo en lista, rol de quien actúa y tipo de incidencia
- [ ] Cada acción: transición, asignar (usuario, informador, responsable, lead, turnos), fijar campo, comentar, añadir y quitar etiqueta, crear subtarea, webhook y notificación
- [ ] Un marcador `{{desconocido}}` hace que la regla no se pueda guardar (422 `automation.invalid_rule`)
- [ ] Una regla que se dispara a sí misma o a otra regla en cadena se detiene a los 5 niveles y no satura el servidor
- [ ] Más de 100 ejecuciones por hora de una regla se limitan
- [ ] Editar una regla con una versión vieja responde 409 (`automation.version_conflict`)
- [ ] `POST /automation-rules/validate` valida sin guardar
- [ ] Una acción de webhook a una URL privada se rechaza
- [ ] Un viewer no puede crear reglas (403); sin organización, 404

## Webhooks (interfaz y API)

- [ ] El webhook de ejemplo del escenario demo aparece desactivado; rotar su secreto genera uno utilizable
- [ ] Crear un webhook: la respuesta muestra el secreto una sola vez; listados posteriores no lo muestran
- [ ] URLs no válidas rechazadas: sin esquema, `ftp://`, con usuario y contraseña, `localhost`, `127.0.0.1`, direcciones privadas y `.internal` (422 `webhook.invalid`)
- [ ] Una redirección hacia una dirección privada no se sigue
- [ ] `POST .../test` envía un evento `ping` y la entrega queda registrada
- [ ] Cada evento suscrito llega con las cabeceras `X-CoreLog-Event`, `X-CoreLog-Delivery`, `X-CoreLog-Timestamp` y `X-CoreLog-Signature`
- [ ] La firma verifica con el ejemplo de Node, Go y Python de la guía de usuario; con el secreto equivocado no verifica
- [ ] El filtro CoreQL del webhook limita los eventos a las incidencias que lo cumplen
- [ ] Un receptor que responde 500 genera reintentos con esperas de 1 min, 5 min, 30 min, 2 h y 12 h; tras el sexto intento la entrega queda `dead`
- [ ] Reenviar una entrega fallida o muerta la envía de nuevo
- [ ] Un receptor que tarda más de 10 segundos cuenta como fallo
- [ ] Un webhook desactivado no recibe eventos
- [ ] Solo propietarios y administradores gestionan webhooks (403 a un miembro)

## Tokens de API (interfaz y API)

- [ ] Crear un token con alcance `read`: puede hacer GET y un POST responde 403 `api_token.scope_insufficient`
- [ ] Un token `write` modifica; un token `admin` solo lo puede crear un administrador de la plataforma
- [ ] El valor completo se muestra una vez y los listados solo muestran el prefijo
- [ ] Un token caducado o revocado responde 401
- [ ] Con un token, `/auth/...`, `/me/password`, `/me/sessions` y `/me/api-tokens` responden 403 `api_token.session_required`
- [ ] El límite de peticiones por minuto del token responde 429 al superarse
- [ ] Un token de una cuenta desactivada deja de funcionar
- [ ] No se pueden tener más de 20 tokens activos y la caducidad máxima es de 366 días

## Importación y exportación (interfaz y API)

- [ ] Subir un CSV de Jira con columnas en inglés: la correspondencia sugerida acierta; con columnas en español también
- [ ] Un CSV sin columna de resumen, de más de 10 MiB, de más de 20 000 filas o con más de 100 columnas se rechaza
- [ ] Iniciar la importación: el trabajo pasa por en cola, validando, ejecutando y terminado, y las incidencias aparecen
- [ ] Las filas con error se descargan en `errors.csv` y no detienen las demás
- [ ] Una celda que empieza por `=`, `+`, `-` o `@` queda inofensiva en las exportaciones
- [ ] La copia JSON del proyecto incluye incidencias, comentarios y tiempo, y solo la descarga un lead o administrador
- [ ] Un contributor no puede importar si el esquema no se lo permite (403)

## Avisos en tiempo real y alerta de nuevo inicio de sesión

- [ ] Con la sesión abierta, un aviso generado por otra persona aparece en la campana sin recargar y el contador sube
- [ ] Al cortar la red unos segundos y restablecerla, la aplicación se reconecta sola y aparecen los avisos perdidos, sin duplicados
- [ ] Abrir CoreLog en una segunda pestaña deja una sola conexión activa por sesión
- [ ] Iniciar sesión desde otro navegador genera la alerta de seguridad en la sesión original, con las acciones **Ver mis sesiones** y **No fui yo**
- [ ] **No fui yo** cierra la sesión nueva (esa sesión recibe 401 en su siguiente petición), confirma con un mensaje y lleva a **Perfil > Seguridad**
- [ ] Detrás de nginx con las directivas de `docs/despliegue.md`, la conexión no se corta ni se almacena en búfer durante al menos 5 minutos
- [ ] La alerta también llega por correo si las preferencias lo permiten

## Tokens de API: tokens revocados

- [ ] En **Perfil > Tokens**, un token revocado desaparece de la lista
- [ ] El interruptor **Mostrar tokens revocados** los muestra con su fecha de revocación y al apagarlo vuelven a ocultarse
- [ ] Un token revocado sigue respondiendo 401 aunque esté oculto

## Estados de error y vacíos

- [ ] Sin avisos: la campana muestra un estado vacío
- [ ] Un fallo del servidor de correo no impide guardar el cambio que lo originó

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Campana y lista de avisos: cabe en 390 y los avisos largos se recortan sin romper el diseño
- [ ] Pantalla de preferencias sin desbordamiento
- [ ] El correo del resumen se ve bien en un cliente de correo en claro y oscuro

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa (incluye las pruebas de firma, reintentos, reglas y esquemas)
- [ ] `go test ./cmd/seed/...` pasa: la regla y el webhook sembrados son válidos y están desactivados
