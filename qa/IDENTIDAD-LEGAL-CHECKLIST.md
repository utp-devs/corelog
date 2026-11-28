# QA Checklist - identidad y consentimientos legales

Cubre los nombres y apellidos, el teléfono y su verificación, los consentimientos legales, la confirmación de contraseña y el registro por pasos.

## Entorno

- Stack y datos como en `qa/P4-CHECKLIST.md`. Las cuentas sembradas tienen nombre y apellidos separados, y algunas tienen un teléfono ficticio con el patrón `+51 9000 nnnnn` (unas verificadas y otras no). La semilla publica los documentos legales y acepta los consentimientos de las cuentas.
- SMS: en desarrollo el envío es simulado y no llega a ningún teléfono; el código se consulta en el registro de la API.
- Cuentas: `admin@corelog.local`, `owner`, `orgadmin`, `lead`, `member`, `viewer`, `guest`, `newbie` de `@corelog.test`.
- Navegadores: Chromium y, si es posible, Firefox. Anchos: 1440, 820 y 390. Temas: claro y oscuro.

## Registro por pasos (`/register`)

- [ ] El asistente tiene tres pasos: "Tus datos", "Tu cuenta" y "Privacidad", y el indicador muestra el paso actual
- [ ] Paso 1: nombres y apellidos obligatorios; teléfono con país
- [ ] Paso 2: correo, contraseña y confirmación de contraseña
- [ ] Paso 3: documentos legales con casillas
- [ ] No se puede avanzar con errores en el paso actual; los errores se muestran junto al campo y se leen con un lector de pantalla
- [ ] Atrás conserva lo escrito; el indicador permite volver a pasos completados pero no saltar a pasos futuros
- [ ] Intro avanza en los dos primeros pasos y solo crea la cuenta en el último
- [ ] Un correo ya registrado se señala sin revelar más datos de la cuenta
- [ ] La contraseña no se acepta si no cumple la política (ver P12) y el indicador de fortaleza lo explica
- [ ] Crear la cuenta lleva al inicio de sesión o a la verificación del correo según la configuración
- [ ] Los datos del primer paso no se pierden si falla la creación (error del servidor)
- [ ] Tras un registro correcto, los consentimientos aceptados se guardan con su origen (`register`)

## Nombres y apellidos

- [ ] Se guardan por separado y el nombre completo se compone de ambos; la cabecera, los comentarios, los menús y las menciones muestran el nombre visible
- [ ] Nombres con tildes, `ñ`, guiones, apóstrofes y espacios dobles se aceptan y se normalizan
- [ ] Vacío, solo espacios, números o símbolos raros se rechazan con mensaje
- [ ] Más de 120 caracteres en total se rechaza
- [ ] Editar los nombres en **Perfil > General** actualiza el nombre visible en todas las pantallas
- [ ] El nombre visible se puede cambiar sin tocar los nombres y apellidos
- [ ] Las cuentas existentes antes del cambio muestran su nombre dividido de forma razonable

## Confirmación de contraseña

- [ ] En el registro, una confirmación distinta bloquea el avance y lo explica
- [ ] Al cambiar la contraseña en el perfil se pide la actual, la nueva y su confirmación
- [ ] Al restablecer la contraseña desde el enlace del correo se pide la nueva y su confirmación
- [ ] Al aceptar una invitación creando una cuenta se pide la confirmación
- [ ] Pegar en el campo de confirmación está permitido (gestores de contraseñas) y mostrar u ocultar la contraseña funciona con teclado

## Teléfono

- [ ] El selector de país muestra el prefijo; escribir el número con espacios o guiones lo normaliza
- [ ] Un número inválido para el país, demasiado corto o con letras se rechaza
- [ ] El teléfono es opcional en el registro
- [ ] Verificar: pulsar Verificar envía un código de 6 dígitos y abre el diálogo
- [ ] Un código correcto marca el teléfono como verificado; uno incorrecto muestra el error y descuenta intentos
- [ ] Tras 5 intentos fallidos o 10 minutos el código deja de valer y hay que pedir otro
- [ ] Más de 3 envíos en una hora se rechazan con mensaje que indica cuándo reintentar
- [ ] Más de 10 solicitudes por hora desde una dirección responden 429
- [ ] Cambiar el número quita la verificación; borrarlo lo elimina
- [ ] Un código de otra cuenta no verifica la propia
- [ ] El número completo no aparece en los registros de la API; en la interfaz se muestra enmascarado fuera de tu perfil
- [ ] Las cuentas sembradas verificadas muestran la marca de verificado y las demás el botón Verificar

## Consentimientos y documentos legales

- [ ] Política de privacidad, tratamiento de datos y términos de uso son obligatorios; comunicaciones y cookies son opcionales
- [ ] El registro sin aceptar un documento obligatorio responde 422 `legal.consent_required` y señala cuáles faltan
- [ ] Aceptar cada documento enlaza con su texto completo en una ventana y permite volver al registro sin perder datos
- [ ] Aceptar una invitación exige aceptar los documentos obligatorios vigentes
- [ ] **Perfil > Privacidad** lista cada documento con versión y fecha de aceptación; los opcionales se pueden retirar y volver a aceptar
- [ ] Un documento obligatorio no se puede retirar
- [ ] Publicar una versión nueva (administrador de la plataforma, `/organization/legal`) hace que las cuentas deban volver a aceptarla
- [ ] Con `LEGAL_ENFORCEMENT=warn` el uso continúa y se avisa; con `block` las demás llamadas responden 403 `legal.consent_pending` hasta aceptar, y la pantalla de consentimiento no se puede cerrar
- [ ] El consentimiento de comunicaciones no se puede marcar como obligatorio (422 `legal.communications_optional`)
- [ ] Un administrador ve quién aceptó qué versión y cuándo; un miembro común no accede a esas pantallas
- [ ] Cada aceptación guarda su origen (registro, invitación, reaceptación al iniciar sesión, perfil)
- [ ] Las cuentas sembradas ya tienen aceptados los documentos y no ven la pantalla de consentimiento

## Datos personales

- [ ] **Perfil > General** edita nombre, nombre visible, teléfono y avatar; el avatar acepta solo imágenes y rechaza archivos grandes
- [ ] Desactivar la cuenta pide la contraseña, cierra las sesiones y oculta a la persona de los selectores nuevos sin borrar su historial
- [ ] Una cuenta desactivada no puede iniciar sesión ni usar sus tokens de API

## Roles y permisos

- [ ] Cada persona edita solo su propio perfil; un PATCH a otro usuario responde 403 o 404
- [ ] Un administrador de la organización no ve el teléfono completo ni los consentimientos de otra persona salvo en la pantalla legal de administración
- [ ] Sin sesión: las pantallas de perfil redirigen al inicio de sesión

## Estados de error y vacíos

- [ ] Servidor caído durante el registro: mensaje claro y datos conservados
- [ ] Teléfono sin verificar: el aviso no bloquea el uso
- [ ] Sin documentos publicados: el registro lo explica y no se queda bloqueado en una pantalla vacía

## Diseño responsivo y tema

Repetir en claro y oscuro, en 1440, 820 y 390 píxeles.

- [ ] El asistente de registro cabe en 390; el teclado móvil no tapa los botones
- [ ] El selector de país y el campo de teléfono caben en una línea en 390
- [ ] El diálogo de verificación y la pantalla de consentimiento tienen foco inicial y se cierran con Escape cuando se permite
- [ ] Mensajes de error con contraste suficiente en ambos temas

## Automatización

- [ ] `go test ./cmd/... ./internal/...` pasa (nombres, teléfono, consentimientos)
- [ ] `go test ./cmd/seed/...` pasa: nombres divididos, teléfonos ficticios con el patrón documentado
- [ ] Playwright: specs de registro, consentimientos e invitaciones pasan
