# QA Checklist - Fase F1, interfaz corporativa

## Entorno

- Rama: `feat/f1-ui-redesign`
- Fecha: 2026-11-21
- Build objetivo: frontend CoreLog

## Automatizacion

- [x] `npm run lint`
- [x] `npm run build`
- [x] `npm test`
- [x] 4 archivos de pruebas frontend pasan
- [x] 10 pruebas frontend pasan

## Tema

- [ ] Activar modo oscuro desde la topbar
- [ ] Recargar y comprobar que el tema oscuro persiste
- [ ] Activar modo claro y comprobar que vuelve a persistir
- [ ] Verificar que fondos, bordes y textos mantienen contraste en ambas variantes

## Navegacion

- [ ] Sidebar colapsable en escritorio
- [ ] Layout responsive en tablet y movil
- [ ] Item seleccionado coincide con la ruta actual
- [ ] Menu de usuario permite ir al perfil y cerrar sesion
- [ ] Campana y buscador permanecen deshabilitados hasta F6/F7

## Pantallas

- [ ] Login corporativo y validacion en español
- [ ] Registro corporativo y validacion en español
- [ ] Perfil con datos personales y cambio de contrasena
- [ ] Listado de tickets con filtros, estados vacios y paginacion
- [ ] Detalle de ticket con encabezado, metadatos y acciones
- [ ] Equipos con tabla, alta, renombrado y eliminacion
- [ ] Miembros con busqueda, alta y baja
- [ ] Ruta 404 con accion de regreso

## Errores y estados

- [ ] 401 redirige al login
- [ ] 403 muestra el comportamiento esperado por rol
- [ ] 404 muestra la pantalla nueva
- [ ] 422 presenta validaciones en español
- [ ] 409 presenta el conflicto sin romper la pagina

## Revisión visual

- [ ] Sin colores hexadecimales fuera de `tokens.ts` y `tokens.css`
- [ ] Sin estilos inline reutilizables
- [ ] Contraste suficiente en botones, textos secundarios y estados
- [ ] Sin textos visibles restantes en inglés
