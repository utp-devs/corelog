# CoreLog web (Angular)

Monorepo de las aplicaciones web de CoreLog. Sustituye a la aplicación React original y usa Angular 21, PrimeNG 21, primeicons y tokens de diseño `--cl-*`.

## Estructura

```
web/
  projects/
    portal/      app "/"            login, registro, invitaciones
    workspace/   app "/workspace/"  proyectos, incidencias, equipos
    admin/       app "/admin/"      consola de la organización
    core/        @corelog/core           configuración, sesión, interceptores, guards, estado, servicios
    ui/          @corelog/ui/<nombre>    componentes compartidos (un entry point por componente)
    features/    @corelog/features/<nombre>  funcionalidades por dominio, con rutas perezosas
  environments/  @corelog/environment   apiUrl y nombre por entorno (local, dev, qa, vps, prod)
  nginx/         imagen de producción (un solo origen, rutas por aplicación)
  scripts/       serve-dist.mjs (servidor local del dist) y dev.mjs (imagen de desarrollo)
  e2e/           Playwright
```

Las aplicaciones son finas: `main.ts` arranca `AppRoot`, `app.config.ts` llama a `provideCorelog`,
`app.routes.ts` declara las rutas. Las tres comparten sesión porque usan el mismo origen y la misma clave
`corelog.auth.token` de `localStorage`.

## Comandos

Desde `web/`:

| Comando                                                    | Qué hace                                                              |
| ---------------------------------------------------------- | --------------------------------------------------------------------- |
| `npm install`                                              | Instala dependencias (usar `npm ci` en Docker)                        |
| `npm run build:libs`                                       | Construye `core`, `ui` y `features` (antes de compilar las apps)      |
| `npm run start:portal` / `start:workspace` / `start:admin` | Servidor de desarrollo con proxy `/api` a `localhost:8080`            |
| `npx ng build <app> --configuration local`                 | Compila una aplicación (`local`, `dev`, `qa`, `vps`, `production`)    |
| `npm run build:all:local`                                  | Compila las tres aplicaciones                                         |
| `npm run test:all`                                         | Pruebas unitarias de core, ui, features y apps (Vitest vía `ng test`) |
| `npm run serve:dist`                                       | Sirve `dist/` como lo haría nginx (requiere haber compilado)          |
| `npm run e2e`                                              | Playwright (requiere `E2E_BASE_URL`)                                  |
| `npm run format:check`                                     | Prettier                                                              |

## Convenciones

- Componentes **standalone**, `ChangeDetectionStrategy.OnPush`, `inject()`, `input()` / `output()`, control de flujo `@if` / `@for`.
- Estado con **signals** (`AsyncState<T>` para lecturas, un store por dominio en `core/lib/state`). RxJS solo en la frontera HTTP.
- Plantillas estrictas (`strictTemplates`) y TypeScript `strict`.
- Estilos con tokens `--cl-*` (`projects/ui/styles/corelog.scss`). Sin colores ni tamaños fijos en componentes.
- Identificadores y comentarios en inglés; textos de interfaz en español.
- Pruebas junto al código (`*.spec.ts`), con `TestBed` y `provideCorelogTesting()`.

## Cómo añadir

- **Un componente compartido**: crear `projects/ui/<nombre>/` con `ng-package.json`, `src/public-api.ts` y el componente. Se importa como `@corelog/ui/<nombre>`.
- **Un dominio de funcionalidades**: crear `projects/features/<nombre>/` con `ng-package.json`, `src/public-api.ts` y un `<NOMBRE>_ROUTES`. Se carga con `loadChildren` en la aplicación.
- **Una aplicación nueva**: copiar `projects/admin/` a `projects/<nombre>/`, añadirla a `angular.json`, a los scripts de `package.json`, a `scripts/serve-dist.mjs`, `scripts/dev.mjs` y al `Dockerfile` (paso de copia a `/site/<nombre>`).
