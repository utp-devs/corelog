# QA Checklist - seguridad y administración

## Entorno

- Stack local y datos del escenario e2e. Para probar el bloqueo y los límites, el entorno no debe tener `RATE_LIMIT_DISABLED=true`; usa una cuenta propia para no bloquear las de la suite.
- SMTP de pruebas (por ejemplo MailHog) o `SMTP_HOST` vacío con el registro de la API a la vista.
- Cuentas: `admin@corelog.local` (administrador de la plataforma), `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Verificación en dos pasos

- [ ] Activar: aparece el código QR y la clave manual; un código incorrecto no confirma
- [ ] Confirmar con un código válido muestra 10 códigos de recuperación una sola vez
- [ ] Iniciar sesión pide el código de 6 dígitos tras la contraseña
- [ ] Un código recién usado no se acepta de nuevo y uno del periodo anterior o siguiente se tolera
- [ ] Un código de recuperación entra una sola vez; el mismo segunda vez falla
- [ ] 5 códigos erróneos invalidan el desafío y piden volver a iniciar sesión; un desafío caduca a los 5 minutos
- [ ] Regenerar los códigos pide contraseña y código, e invalida los anteriores
- [ ] Desactivar pide contraseña y código; después se inicia sesión solo con la contraseña
- [ ] El secreto de 2FA no aparece nunca en las respuestas ni en los registros después de la activación
- [ ] La API arranca en `APP_ENV=production` solo con `TWO_FACTOR_ENCRYPTION_KEY`

## Contraseñas

- [ ] Política: menos de 10 caracteres, solo una clase con menos de 14, más de 72 bytes, que contenga el correo y una contraseña común se rechazan con el motivo en español
- [ ] Una contraseña de 14 o más caracteres de una sola clase se acepta
- [ ] Cambiar la contraseña exige la actual; una actual incorrecta se rechaza
- [ ] Cambiar la contraseña cierra las demás sesiones
- [ ] Recuperación: la respuesta de "olvidé mi contraseña" es la misma para un correo existente y uno inexistente
- [ ] El correo llega con un enlace que abre `/reset-password`; sirve una sola vez y caduca
- [ ] Un enlace manipulado o caducado muestra un error claro con la opción de pedir otro
- [ ] Tras restablecer, las sesiones anteriores quedan cerradas
- [ ] Más de 5 solicitudes de recuperación por hora desde una dirección responden 429

## Verificación del correo

- [ ] Tras registrarse llega el correo de verificación y el enlace abre `/verify-email` con confirmación
- [ ] Reenviar la verificación desde el inicio de sesión funciona y respeta el límite de frecuencia
- [ ] Con `REQUIRE_EMAIL_VERIFICATION=true` una cuenta sin verificar no inicia sesión y el mensaje lo explica
- [ ] Un enlace usado o caducado muestra el error

## Bloqueo y límites

- [ ] 5 contraseñas incorrectas seguidas bloquean la cuenta 15 minutos; el mensaje indica el tiempo restante
- [ ] Con la cuenta bloqueada, la contraseña correcta tampoco entra hasta que pase el tiempo
- [ ] El segundo bloqueo consecutivo dura el doble, hasta 24 horas
- [ ] Un inicio de sesión correcto antes del umbral reinicia el contador
- [ ] Un correo inexistente produce la misma respuesta y el mismo bloqueo que uno existente (sin enumerar cuentas)
- [ ] Más de 10 inicios de sesión por minuto desde una dirección responden 429
- [ ] Más de 20 registros por hora desde una dirección responden 429
- [ ] Detrás del proxy, con `TRUSTED_PROXIES` el límite se aplica por dirección de cliente y sin él por la del proxy

## Sesiones

- [ ] El listado muestra la sesión actual y las demás con dispositivo y última actividad
- [ ] Cerrar una sesión ajena la invalida (la siguiente petición de ese navegador recibe 401)
- [ ] "Cerrar todas las demás" conserva la actual
- [ ] Desactivar la cuenta pide contraseña y cierra todas las sesiones; después no se puede iniciar sesión
- [ ] Un token de API no puede gestionar sesiones

## Borrado lógico y alertas de sesión

- [ ] Eliminar un elemento con borrado lógico (organización, proyecto, incidencia, comentario, nota, panel) lo oculta de listados, búsqueda, informes y calendario, y su detalle responde 404
- [ ] Restaurar (donde existe) lo devuelve con su historial; con la depuración en `PURGE_DRY_RUN=true` solo se registra lo que se borraría
- [ ] Un token de API revocado no aparece en la lista salvo con el interruptor **Mostrar tokens revocados**
- [ ] Un inicio de sesión nuevo genera una alerta en tiempo real con **No fui yo**; pulsarla cierra esa sesión y recomienda cambiar la contraseña
- [ ] Limitación conocida: los estados y las membresías todavía se eliminan de forma definitiva

## Cabeceras y protecciones HTTP

- [ ] Con `HSTS_ENABLED=true` la respuesta lleva `Strict-Transport-Security`
- [ ] CORS: un origen no permitido no recibe `Access-Control-Allow-Origin`
- [ ] Un cuerpo JSON de más de `MAX_JSON_BODY_BYTES` se rechaza; un JSON con campos desconocidos también
- [ ] Las respuestas de error no muestran trazas ni rutas internas

## Auditoría

- [ ] `/organization/audit` la ven propietarios y administradores; un miembro recibe 403 y no ve el menú
- [ ] Cambiar el rol de un miembro, crear un proyecto, cambiar un esquema de permisos, activar 2FA y crear un webhook generan entradas con persona, dirección e identificador de petición
- [ ] Los filtros por fecha, persona, tipo y entidad reducen la lista; la paginación no salta ni repite entradas
- [ ] La exportación a CSV abre bien en una hoja de cálculo y neutraliza las celdas con fórmulas
- [ ] Una organización no ve las entradas de otra
- [ ] `/organization/platform-audit` solo la ve un administrador de la plataforma
- [ ] Las entradas más antiguas que `AUDIT_RETENTION_DAYS` se eliminan
- [ ] Ninguna entrada contiene contraseñas, tokens ni secretos

## Administración de la organización

- [ ] Invitaciones: crear, reenviar y cancelar; el enlace de una invitación cancelada o caducada falla
- [ ] Aceptar una invitación con otra cuenta distinta a la del correo se rechaza o lo avisa
- [ ] Cambiar el rol de un miembro y quitarlo; el último propietario no puede dejar la organización ni ser degradado
- [ ] Transferir la propiedad pide confirmación y cambia los roles
- [ ] Un administrador no puede quitar ni degradar a un propietario
- [ ] Configuración de la organización: nombre, descripción, logotipo (archivo no válido o demasiado grande rechazado)
- [ ] Un miembro común no ve el menú de administración y las URL directas responden 403

## Roles y permisos

- [ ] Administrador de la plataforma: auditoría global, cola de correo y tareas (`/admin/jobs`)
- [ ] Propietario y administrador: lo anterior de su organización
- [ ] Miembros, invitados y sin organización: 403 o 404 en las rutas administrativas

## Estados de error y vacíos

- [ ] Auditoría sin entradas para el filtro: estado vacío
- [ ] Error de red durante la activación de 2FA: se puede reintentar sin dejar la cuenta a medias

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] Perfil > Seguridad: sesiones, 2FA y contraseña sin desbordamiento; el código QR legible en oscuro
- [ ] Los códigos de recuperación se pueden copiar y descargar
- [ ] Tabla de auditoría con scroll controlado en 390
- [ ] Pantallas de inicio de sesión, recuperación y verificación con foco visible y mensajes anunciados por lectores de pantalla

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa (bloqueo, política de contraseñas, 2FA, auditoría)
- [ ] Playwright: specs de autenticación, 2FA y auditoría pasan
