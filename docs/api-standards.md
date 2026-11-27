# Convenciones y estandares de API REST (CoreLog)

Este documento define las directrices obligatorias de diseño, convenciones y patrones para los endpoints de la API REST de CoreLog (`/api/v1`). Todo nuevo modulo, adaptador HTTP o caso de uso debe respetar estos estandares.

---

## 1. Principios generales

1. **Protocolo**: HTTP/1.1 o HTTP/2 sobre TLS.
2. **Formato**: JSON (`application/json; charset=utf-8`) tanto en peticiones con cuerpo como en respuestas.
3. **Identificadores**: Siempre UUID v4 representados como cadenas canónicas (ej. `550e8400-e29b-41d4-a716-446655440000`).
4. **Fechas y horas**: Formato ISO 8601 / RFC 3339 en UTC (ej. `2026-11-21T18:30:00Z`).
5. **Nomenclatura en JSON**: `snake_case` para todas las claves de los objetos JSON.
6. **URLs**:
   - Prefijo global: `/api/v1` (salvo `/health` que vive en la raíz).
   - Rutas en minúsculas y nombres de recursos en **plural** (ej. `/teams`, `/users`, `/projects`, `/issues`).
   - Identificadores de recurso mediante parámetros de ruta: `/{resource}/{id}` (ej. `/issues/{id}/status`).

---

## 2. Envoltorios de respuesta

Todas las respuestas de la API utilizan un envoltorio estándar unificado gestionado mediante `internal/platform/httpresponse`:

### 2.1 Respuestas exitosas (`200 OK`, `201 Created`)

El resultado exitoso se encapsula en la propiedad `data`:

```json
{
  "data": {
    "id": "11111111-1111-4111-8111-111111111111",
    "title": "Producción API devuelve 500s",
    "status": "open",
    "created_at": "2026-09-10T09:30:00Z"
  }
}
```

Para listas paginadas:

```json
{
  "data": [
    { "id": "...", "name": "Team A" },
    { "id": "...", "name": "Team B" }
  ]
}
```

### 2.2 Respuestas de error (`4xx`, `5xx`)

Los errores se encapsulan en la propiedad `error` con un `message` legible y, opcionalmente, `details`:

```json
{
  "error": {
    "message": "title is required"
  }
}
```

---

## 3. Manejo y mapeo de errores (`apperror`)

El backend desacopla los errores de dominio y aplicación de HTTP mediante `internal/platform/apperror`. La capa HTTP (`httpresponse.Error`) mapea cada `apperror.Kind` a su código HTTP correspondiente:

| `apperror.Kind` | HTTP Status | Significado / Uso |
| --- | --- | --- |
| `KindValidation` | `422 Unprocessable Entity` | Formato JSON inválido, datos requeridos faltantes o fallo de validaciones de reglas de entrada |
| `KindUnauthorized` | `401 Unauthorized` | Token JWT ausente, expirado, malformado o credenciales de acceso inválidas |
| `KindForbidden` | `403 Forbidden` | El usuario está autenticado pero carece de permisos para realizar la acción (rol insuficiente o recurso protegido) |
| `KindNotFound` | `404 Not Found` | El recurso solicitado no existe o no es visible para el actor |
| `KindConflict` | `409 Conflict` | Conflicto de estado (ej. email ya registrado, nombre de equipo duplicado o transición de estado ilegal) |
| `KindInternal` | `500 Internal Server Error` | Fallos no controlados del servidor o de base de datos; nunca debe filtrar trazas ni secretos al cliente |
| `KindRateLimited` | `429 Too Many Requests` | Límite de peticiones superado |

### 3.1 Campos `code` y `details` en los errores

Además de `message`, el cuerpo de error puede incluir:

- `code`: identificador estable y legible por máquina (por ejemplo `issue_not_found`, `team_name_taken`). Los clientes deben basar su lógica en `code`, no en `message`, que puede cambiar.
- `details`: lista de errores por campo, cada uno con `field` (nombre del campo en snake_case) y `message` (descripción en español). Se usa en fallos de validación de varios campos a la vez.

Ambos campos se **omiten** cuando están vacíos. Ejemplo de error de validación:

```json
{
  "error": {
    "message": "datos de entrada no válidos",
    "code": "issue_invalid",
    "details": [
      { "field": "title", "message": "es obligatorio" },
      { "field": "priority", "message": "debe ser low, medium, high o critical" }
    ]
  }
}
```

---

## 4. Paginación y filtrado

Todos los endpoints que devuelven listados de recursos deben soportar paginación consistente:

1. **Parámetros de consulta (Query params)**:
   - `page`: Número de página, entero positivo (mínimo `1`, valor por defecto: `1`).
   - `page_size`: Tamaño de página, entero positivo (mínimo `1`, valor por defecto: `20`, máximo permitido: `100`).
2. **Filtros**:
   - Se pasan como query params en snake_case (ej. `status`, `priority`, `project_id`, `q`).
   - El backend ignora filtros vacíos o los trata con su valor por defecto.

---

## 5. Autenticación y Autorización

1. **Autenticación**:
   - Cabecera obligatoria en rutas protegidas: `Authorization: Bearer <token_jwt>`.
   - Token emitido mediante `POST /api/v1/auth/login`.
   - Claims estándar: `sub` (UUID del usuario), `role` (cadena de rol: `admin`, `agent`, `requester`), expiración (`exp`), emisión (`iat`).
2. **Autorización global por rol**:
   - Middleware `middleware.RequireRole("admin")` restringe endpoints administrativos globales.
3. **Autorización por recurso**:
   - Las reglas de negocio específicas de acceso a un recurso (por ejemplo, asignar issues solo a miembros del mismo equipo o proyecto) se evalúan en el caso de uso y devuelven `KindForbidden` (403) o `KindNotFound` (404 si la existencia del recurso debe ocultarse).

---

## 6. Sincronización del contrato OpenAPI

- La especificación OpenAPI (`openapi.yaml`) en la raíz del repositorio es el contrato vivo de la API.
- **Regla estricta**: Todo commit o PR que añada o modifique un endpoint HTTP debe actualizar simultáneamente `openapi.yaml`.
