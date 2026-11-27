# Checklist de QA - Fase [F_] ([Nombre de la Fase])

- **Responsable de QA**: 
- **Fecha de ejecución**: YYYY-MM-DD
- **Versión / Release**: v0.X.0
- **Entorno de pruebas**: PostgreSQL Docker (http://localhost:8080/api/v1)

---

## 1. Verificación técnica y quality gates

- [ ] `go test ./...` ejecuta todas las pruebas unitarias y de integración (código de salida 0).
- [ ] `make test-coverage` supera el umbral mínimo definido para la fase (mínimo 40% en F0).
- [ ] `npm run test:all` en `web/` ejecuta las pruebas de frontend sin errores.
- [ ] `npm run format:check` y `npm run build:all` en `web/` compilan sin errores ni warnings.
- [ ] Migraciones PostgreSQL ejecutadas limpiamente (`up` y `down`).
- [ ] `openapi.yaml` refleja fielmente los endpoints añadidos o modificados.

---

## 2. Casos de prueba funcionales

### Casos de éxito (Happy Path)
- [ ] Caso 1: [Descripción] → Esperado: 200/201 → Obtenido: [ ]
- [ ] Caso 2: [Descripción] → Esperado: 200/201 → Obtenido: [ ]

### Validaciones y errores tipados
- [ ] Payload vacío o formato JSON inválido → Esperado: 422 Unprocessable Entity → Obtenido: [ ]
- [ ] Identificador de formato inválido (no UUID) → Esperado: 422 Unprocessable Entity → Obtenido: [ ]
- [ ] Recurso inexistente → Esperado: 404 Not Found → Obtenido: [ ]
- [ ] Conflicto de estado o regla de negocio (duplicados, transición ilegal) → Esperado: 409 Conflict → Obtenido: [ ]

---

## 3. Matriz de Autorización y Permisos

- [ ] Petición sin cabecera `Authorization` → Esperado: 401 Unauthorized → Obtenido: [ ]
- [ ] Petición con token expirado o firma inválida → Esperado: 401 Unauthorized → Obtenido: [ ]
- [ ] Usuario con rol insuficiente sobre endpoint administrativo → Esperado: 403 Forbidden → Obtenido: [ ]
- [ ] Usuario intentando operar sobre recursos de otro equipo/proyecto no asignado → Esperado: 403 Forbidden o 404 → Obtenido: [ ]

---

## 4. Pruebas visuales y de interfaz (si aplica)

- [ ] Modo claro: componentes renderizados correctamente, legibilidad y contraste adecuado.
- [ ] Modo oscuro: ausencia de fondos o textos con colores hardcodeados rotos.
- [ ] Persistencia de preferencias (tema, filtros) tras recargar la página (`F5`).
- [ ] Estados vacíos (`EmptyState`), errores de red (`ErrorState`) y estados de carga (`Skeleton`).
- [ ] Navegación accesible y URLs coherentes.

---

## 5. Historial de revisiones de PR

| PR | Rama | Qué se probó | Resultado | Firma |
|---|---|---|---|---|
| # | | | Aprobado / Rechazado | |
