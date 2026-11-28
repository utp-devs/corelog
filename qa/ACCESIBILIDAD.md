# Accesibilidad

Pruebas automáticas de accesibilidad (WCAG 2.1 A y AA más las buenas prácticas de axe) y recorridos solo con teclado. Complementan, no sustituyen, la revisión manual con lector de pantalla descrita al final.

## Qué cubre

### Auditoría con axe (`web/e2e/specs/a11y/screens.spec.ts` y `dialogs.spec.ts`)

El ayudante `web/e2e/lib/axe.ts` (`expectNoAxeViolations`, `axeSummary`) ejecuta las reglas `wcag2a`, `wcag2aa`, `wcag21a`, `wcag21aa` y `best-practice` y, si falla, imprime el identificador de la regla, su impacto y los nodos afectados. Incluye el contraste de color renderizado, así que la comprobación de contraste se hace en el tema claro y en el oscuro (proyectos `desktop` y `desktop-dark`) y en pantalla ancha y de teléfono.

Pantallas:

- Portal: inicio de sesión, registro, recuperar y restablecer contraseña, verificar correo, aceptar invitación, página no encontrada y onboarding.
- Espacio de trabajo: panel, paneles, incidencias, nueva incidencia, vista de incidencia, filtros, proyectos, asistente de proyecto, tablero, backlog, sprints, calendario y informes del proyecto, informe de burndown, hoja de ruta, las 14 secciones de ajustes del proyecto, equipos, equipo, notas, calendario, horas, personas, persona y las 6 secciones del perfil (general, seguridad, notificaciones, apariencia, privacidad y tokens).
- Administración: resumen, ajustes, miembros, catálogos, campos, estados, invitaciones, legal, notificaciones, auditoría, webhooks, auditoría de plataforma, bandeja de correo, tareas y la página 403.

Superposiciones abiertas: diálogo de crear incidencia, paleta de comandos, atajos de teclado, cajón de apuntes, menú de cuenta, confirmación de eliminar proyecto y documento legal del registro.

### Solo teclado (`web/e2e/specs/a11y/keyboard.spec.ts`)

Solo Tab, Shift+Tab, Enter, Espacio, flechas y Escape, sin ratón: inicio de sesión; asistente de registro (avanzar, retroceder, documento legal y casilla de consentimiento); diálogo de crear incidencia (foco atrapado y devuelto al botón que lo abrió); mover una tarjeta del tablero con Alt+flecha (el foco se queda en la tarjeta); selección múltiple en el backlog; tabla de transiciones del editor de flujo; paleta de comandos; diálogo de atajos y cajón de apuntes (foco atrapado y devuelto); enlace "Saltar al contenido"; foco en el contenido principal y aviso en la región en vivo al cambiar de ruta; cajón de navegación en teléfono.

## Cómo ejecutarlo

Con la pila local y el servidor de desarrollo levantados (ver el README de `web/`), desde `web/`:

```sh
npx playwright test e2e/specs/a11y                     # todos los proyectos
sh scripts/e2e-local.sh --project=desktop e2e/specs/a11y
sh scripts/e2e-local.sh --project=desktop-dark --project=mobile --project=tablet e2e/specs/a11y
```

Los recorridos de teclado se ejecutan solo en el proyecto `desktop` (salvo el cajón de navegación, que es de `mobile`). Para auditar una pantalla nueva, añade su ruta a la lista de `screens.spec.ts`.

## Exclusiones

Todas viven en `web/e2e/lib/axe-exclusions.ts`, con el motivo junto a cada una. Solo se admiten internos de terceros; el marcado propio se corrige en el componente.

| Regla | Elemento | Motivo |
| --- | --- | --- |
| `region` | `.cdk-overlay-container`, paneles de PrimeNG | Menús y paneles emergentes se montan en `body` fuera de los landmarks. Son widgets transitorios con su propio rol y nombre. El resto de reglas siguen aplicándose a esos nodos. |

No hay exclusiones de subárbol completo.

## Revisión manual recomendada

- Pasada con lector de pantalla (NVDA con Firefox o Chrome, VoiceOver en Safari) por el inicio de sesión, el tablero, la vista de incidencia y el cajón de notificaciones: orden de lectura, anuncios de la región en vivo y nombres de los controles.
- Zoom al 200 % y 400 % y reflujo en 320 px de ancho.
- Modo de contraste alto de Windows y preferencia de movimiento reducido.
- Arrastrar y soltar con tecnología de apoyo (el tablero y el backlog ofrecen Alt+flecha y el menú "Mover", conviene confirmarlos con el lector).
- Contenido generado por personas (Markdown de descripciones y comentarios), cuyo marcado depende de lo que escriban.
