# Registro de cambios

Todos los cambios relevantes de CoreLog se documentan aquí. El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/) y el proyecto usa [versionado semántico](https://semver.org/lang/es/).

## [Unreleased] - 1.0

La versión 1.0 convierte CoreLog, un gestor de incidencias de un solo espacio, en una plataforma completa de gestión de trabajo: organizaciones, proyectos, flujos, tableros, búsqueda, informes, automatización e integraciones, con una interfaz nueva en Angular. Para actualizar una instalación anterior, sigue la guía de `docs/despliegue.md` y haz un respaldo antes.

### Added

- **Organizaciones y equipos:** organizaciones como nivel superior, con miembros, invitaciones por enlace, roles (propietario, administrador, miembro, invitado), equipos con rol en los proyectos, catálogos de tipos, prioridades y resoluciones, campos personalizados y estados de la organización. Las instalaciones existentes pasan a una organización predeterminada.
- **Proyectos:** plantillas (Scrum, Kanban, seguimiento de tareas y de errores), clave única por organización, visibilidad abierta o privada, roles y esquemas de permisos por proyecto, archivar y restaurar, favoritos y recientes, versiones y componentes.
- **Incidencias:** jerarquía de épica, historia y subtarea, vínculos, etiquetas, comentarios con menciones y ediciones, registro de tiempo y estimaciones, clonar, mover y convertir, acciones masivas, adjuntos, seguidores y exportación a CSV.
- **Motor de flujos de trabajo:** estados y transiciones por proyecto con editor visual, condiciones, validadores y acciones posteriores.
- **Tableros, backlog, sprints y lanzamientos:** tablero con límites WIP, backlog ordenable, ciclo completo de sprint con informe y burndown, versiones y lanzamientos.
- **Búsqueda con CoreQL**, filtros guardados y suscripciones a filtros.
- **Notas personales**, vinculables a incidencias.
- **Tiempo e informes:** hojas de horas e informes por proyecto.
- **Paneles, hoja de ruta y calendario:** paneles con gadgets, hoja de ruta por proyecto, calendario con eventos recurrentes y suscripción ICS.
- **Notificaciones:** esquemas por organización y por proyecto, preferencias personales (en la aplicación, correo inmediato o resumen diario), cola de correo con reintentos, flujo en tiempo real (`GET /api/v1/notifications/stream`, SSE) y alerta de seguridad por nuevo inicio de sesión con la acción "No fui yo".
- **Automatización:** reglas con disparadores, condiciones y acciones, constructor en la interfaz, protección contra bucles y límite de ejecuciones.
- **Integraciones:** webhooks firmados con reintentos y entregas consultables, tokens de API personales con alcances y caducidad, importación de incidencias desde CSV con asistente, y exportación de proyectos en JSON.
- **Seguridad y cumplimiento:** verificación en dos pasos con códigos de recuperación, sesiones revocables, bloqueo por intentos fallidos, límites de frecuencia, verificación del correo, política de contraseñas, auditoría con exportación y retención configurable, consentimientos legales en el registro con documentos versionados, teléfono con código de país y desactivación de cuenta.
- **Despliegue:** compose de producción con todas las variables de configuración, volúmenes para adjuntos, avatares, logotipos e importaciones, *healthchecks*, respaldo previo a cada despliegue y guía de actualización, proxy inverso y TLS en `docs/despliegue.md`.
- **Calidad:** auditoría de autorización de todas las rutas, pruebas de rendimiento con 20 000 incidencias, suite Playwright en tres tamaños y dos temas, escaneo de accesibilidad y umbral de cobertura del backend.

### Changed

- **Interfaz reescrita en Angular 21** (portal, espacio de trabajo y administración, con bibliotecas `core`, `ui` y `features`), en sustitución de la interfaz anterior en React.
- **Rendimiento:** índices nuevos para listados, tableros, backlog y búsqueda; consultas por lotes sin N+1.
- **Borrado lógico** en 18 tablas (organizaciones, proyectos, incidencias, comentarios, notas, paneles, reglas, webhooks y otras): los elementos eliminados se ocultan y pueden restaurarse en lugar de borrarse al instante. Los tokens de API revocados se ocultan de la lista, con un interruptor para mostrarlos.
- La semilla de datos (`cmd/seed`) se guía por entorno: la contraseña viene de `SEED_PASSWORD` y `--force-reset` se niega a vaciar una base con cuentas reales salvo con `SEED_ALLOW_DATA_LOSS=yes`.
- Las pruebas del backend se ejecutan con `-p 2` y el pipeline publica cobertura mínima del 70 %.

### Security

- Auditoría completa de autorización: cada ruta decide permisos en los casos de uso, los recursos ocultos responden 404 y los visibles sin permiso 403; aislamiento entre organizaciones comprobado por pruebas.
- Cifrado AES-GCM de los secretos de 2FA y de los secretos de firma de los webhooks; los tokens de API y de sesión se guardan solo como huella.
- Protección contra SSRF en webhooks (direcciones privadas rechazadas en producción), cabeceras de seguridad y HSTS opcional, límite de tamaño de cuerpo JSON y plazos del servidor HTTP.
- Alerta inmediata ante un inicio de sesión nuevo y cierre de la sesión desde la propia alerta.
- Los registros no incluyen contraseñas, tokens ni secretos; CORS sin cabeceras cuando no hay orígenes válidos.

### Deprecated

- La variable `CORS_ALLOWED_ORIGIN` (un solo origen) se mantiene por compatibilidad; usa `CORS_ALLOWED_ORIGINS`.
- Los alias `make ui-install`, `ui-dev`, `ui-build`, `ui-lint` y `ui-test` se conservan por costumbre; usa los objetivos `web-*`.

### Removed

- La interfaz en React (carpeta `ui/`), sustituida por el espacio de trabajo Angular en `web/`.
- El script de datos de ejemplo en SQL, sustituido por `cmd/seed`.

### Known limitations

- Los textos legales incluidos (privacidad, tratamiento de datos, términos, cookies, comunicaciones) son borradores: requieren revisión legal antes de usarse con personas reales.
- No hay un proveedor real de SMS: el código de verificación del teléfono no sale a ningún dispositivo.
- Los estados y las membresías todavía se eliminan de forma definitiva (sin borrado lógico ni restauración).
- La depuración de elementos eliminados (`PURGE_ENABLED`, retención de 30 días) está en desarrollo: déjala en modo de prueba (`PURGE_DRY_RUN=true`) hasta validar su registro.
- La API solo expone `GET /health`; no hay un endpoint `/ready` separado.
- Con varias réplicas de la API, las tareas en segundo plano pueden duplicarse; ejecútalas en una sola réplica.
