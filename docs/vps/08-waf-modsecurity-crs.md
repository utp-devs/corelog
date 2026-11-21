# 08 · WAF: ModSecurity + OWASP CRS

← [07 Configuración de Nginx](07-nginx-configuracion.md) · [Índice](README.md) · Siguiente: [09 Anubis](09-anubis.md)

**Objetivo:** un *Web Application Firewall* activo que inspeccione cada petición con las reglas del OWASP Core Rule Set, bloquee ataques (inyección SQL, XSS, traversal…) y deje un registro auditable.

## 1. Piezas

| Pieza | Dónde vive | Quién la aporta |
|---|---|---|
| `libmodsecurity` 3.0.17 y el conector `ngx_http_modsecurity_module.so` | **Dentro de la imagen** | Compilados en [06](06-imagen-nginx-clang.md) |
| `modsecurity.conf` | `/etc/modsecurity/` en el host | [`recursos/modsecurity/modsecurity.conf`](recursos/modsecurity/modsecurity.conf) |
| Reglas OWASP CRS | `/usr/share/modsecurity-crs/` en el host | Descarga del proyecto CRS (sección 2) |
| Registros | `/var/log/modsec_audit.log`, `/var/log/modsecurity/` | Escritos por Nginx |

La imagen trae el motor, **no** las reglas: se montan desde el host y se actualizan sin reconstruir la imagen.

## 2. Instalar las reglas CRS

Usar siempre una **versión publicada** (no una rama de desarrollo):

```bash
CRS=v4.30.0     # ver la última en https://github.com/coreruleset/coreruleset/releases
cd /tmp && curl -fsSL "https://github.com/coreruleset/coreruleset/archive/refs/tags/$CRS.tar.gz" -o crs.tgz
tar xzf crs.tgz
mkdir -p /usr/share/modsecurity-crs
cp -a coreruleset-${CRS#v}/rules /usr/share/modsecurity-crs/rules
cp coreruleset-${CRS#v}/crs-setup.conf.example /usr/share/modsecurity-crs/crs-setup.conf
ls /usr/share/modsecurity-crs/rules | wc -l       # ~49 archivos .conf/.data
```

## 3. `modsecurity.conf`

```
SecRuleEngine On
SecAuditEngine RelevantOnly
SecAuditLogRelevantStatus "^(?:5|4(?!04))"
SecAuditLogParts ABCEFHZ
SecAuditLogType Concurrent
SecAuditLog "/var/log/modsec_audit.log"
SecAuditLogStorageDir "/var/log/modsecurity"
SecRequestBodyAccess On
SecRequestBodyLimit 13107200
SecRequestBodyNoFilesLimit 131072
SecRequestBodyLimitAction Reject
SecPcreMatchLimit 100000
SecPcreMatchLimitRecursion 100000
SecResponseBodyAccess Off
SecUploadDir /tmp
SecUploadKeepFiles Off
SecWebAppId "corelog-vps"
SecArgumentSeparator "&"
SecTmpDir /tmp
SecDataDir /tmp
Include /usr/share/modsecurity-crs/crs-setup.conf
Include /usr/share/modsecurity-crs/rules/*.conf
```

| Directiva | Por qué |
|---|---|
| `SecRuleEngine On` | **Bloquea**. `DetectionOnly` solo registra (útil al desplegar un sitio nuevo) |
| `SecAuditEngine RelevantOnly` + `SecAuditLogRelevantStatus` | Audita solo respuestas 5xx y 4xx (excepto 404): el log no se llena de tráfico normal |
| `SecAuditLogParts ABCEFHZ` | Cabeceras, cuerpo de la petición, reglas disparadas… sin el cuerpo de la respuesta |
| `SecAuditLogType Concurrent` | Un archivo por evento en `/var/log/modsecurity/AAAAMMDD/` + índice en `modsec_audit.log` |
| `SecRequestBodyLimit` 12.5 MB, `…NoFilesLimit` 128 KB | Tope del cuerpo inspeccionado; debe ser ≥ `client_max_body_size` de Nginx para subidas |
| `SecResponseBodyAccess Off` | No inspecciona respuestas: menos CPU y memoria |
| `SecPcreMatchLimit*` | Evita que una expresión regular patológica consuma la CPU (ReDoS) |

> **Permisos.** Los *workers* de Nginx corren como `www-data` (`user www-data;` en `nginx.conf`): `modsec_audit.log` y `/var/log/modsecurity/` deben ser de `www-data` (`640` y `750`). Si pertenecen a `root`, el audit log queda vacío en silencio. Sin la directiva `user`, los workers son `nobody` y el dueño debe ser `nobody:nogroup`. Ver [07](07-nginx-configuracion.md).

## 4. Cómo decide el CRS (puntuación de anomalía)

Cada regla que coincide **suma puntos** a la petición (crítica 5, error 4, aviso 3, nota 2). Al final de la fase de petición, si el total supera el umbral (`inbound_anomaly_score_threshold`, por defecto 5), se bloquea con 403:

```
ModSecurity: Access denied with code 403 (phase 2). Matched "Operator `Ge' with parameter `5'
against variable `TX:BLOCKING_INBOUND_ANOMALY_SCORE' (Value: `20')
```

Una sola regla crítica (por ejemplo, inyección SQL) basta; varias coincidencias menores también pueden llegar al umbral.

### Nivel de paranoia

En `crs-setup.conf`, `tx.blocking_paranoia_level` (1–4). Se usa **1** (el valor por defecto): pocos falsos positivos. Subir a 2 amplía la detección a costa de más falsos positivos; solo con pruebas y un periodo en `DetectionOnly`. El servidor de referencia usa nivel 2 en su imagen alternativa.

## 5. Probar que bloquea

Desde un equipo del país permitido (la imagen del servidor responde 403 antes de llegar a la aplicación):

```bash
UA="Mozilla/5.0 Chrome/120"
curl -s -o /dev/null -A "$UA" -w "%{http_code}\n" "https://api.example.com/?id=1%27%20OR%201=1--"      # 403 (SQLi)
curl -s -o /dev/null -A "$UA" -w "%{http_code}\n" "https://api.example.com/?q=%3Cscript%3Ealert(1)%3C/script%3E"  # 403 (XSS)
curl -s -o /dev/null -A "$UA" -w "%{http_code}\n" "https://api.example.com/api/v1/health"               # pasa el WAF
```

Ver el evento:

```bash
grep "ModSecurity" /var/log/nginx/error.log | tail -2          # también en `docker logs nginx`
tail -1 /var/log/modsec_audit.log                              # índice del evento
ls /var/log/modsecurity/$(date +%Y%m%d)/                       # detalle completo
```

Verificado en el servidor: 845 reglas cargadas (`rules loaded inline/local/remote: 0/845/0` al arrancar) y bloqueo de SQLi y XSS con 403.

## 6. Falsos positivos: cómo afinar

Tras desplegar una aplicación real habrá peticiones legítimas bloqueadas (formularios con texto, JSON con caracteres especiales, subidas…). Procedimiento:

1. **Reproducir** y localizar el evento en el audit log: la parte `H` lista las reglas (`[id "942100"]`).
2. **Decidir el alcance mínimo**: una ruta concreta, un parámetro concreto; nunca desactivar el WAF entero.
3. **Escribir la exclusión** en un archivo propio, con identificadores `1000000+` (reservados al usuario), e incluirlo **después** de las reglas CRS:

```
# /etc/modsecurity/custom_exclusions.conf
# Quitar la regla 942100 solo para el parámetro "q" de la búsqueda de usuarios
SecRule REQUEST_URI "@beginsWith /api/v1/users/search" \
    "id:1000001,phase:1,pass,nolog,\
    ctl:ruleRemoveTargetById=942100;ARGS:q"

# Apagar el motor para un endpoint concreto y de confianza (último recurso)
SecRule REQUEST_URI "@streq /api/v1/set-policy" \
    "id:1000002,phase:1,pass,nolog,ctl:ruleEngine=Off"
```

```
# al final de modsecurity.conf
Include /etc/modsecurity/custom_exclusions.conf
```

> Estos ejemplos son **plantillas**: en el servidor documentado aún no hizo falta ninguna exclusión (no hay tráfico real que las exija). Validar siempre con `nginx -t` y repetir la prueba de ataque de la sección 5.

4. Validar y recargar: `nginx -t && systemctl reload nginx` (la unidad ya valida antes de recargar).
5. Repetir la petición y confirmar que pasa **y que el ataque de la sección 5 sigue bloqueado**.
6. Dejar un comentario con el motivo y la fecha de cada exclusión.

> `/etc/modsecurity` está montado `:ro` y se lee al arrancar/recargar. Una exclusión nueva requiere `reload`.

## 7. Despliegue gradual de un sitio nuevo

1. `SecRuleEngine DetectionOnly` y `reload`.
2. Usar la aplicación con normalidad una o dos semanas y revisar las reglas que se disparan.
3. Excluir lo legítimo (sección 6).
4. Pasar a `SecRuleEngine On`.

## 8. Actualizar el CRS

```bash
# nueva versión a un directorio aparte, comparar y reemplazar
mv /usr/share/modsecurity-crs /usr/share/modsecurity-crs.old
# …repetir la sección 2 con la versión nueva…
nginx -t && systemctl reload nginx
# probar los ataques de la sección 5; si algo falla: volver a .old y recargar
```

Revisar las notas de la versión: los identificadores de regla cambian y pueden invalidar exclusiones propias.

## 9. Integración con fail2ban

Cada bloqueo del WAF deja una línea `ModSecurity: Access denied` en `error.log`. El jail `nginx-modsec` banea una hora a quien acumula 5 en 10 minutos, y `recidive` lo hace permanente si reincide ([11](11-fail2ban.md)).

## 10. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `docker logs nginx 2>&1 \| grep "rules loaded"` | `…0/845/0` (o el número de reglas de la versión) |
| SQLi y XSS de prueba | `403` |
| Petición normal | Pasa (200/401/404 de la aplicación, no 403) |
| `wc -l /var/log/modsec_audit.log` tras un bloqueo | Crece |
| `ls -l /var/log/modsec_audit.log` | Dueño `www-data` |

Continuar con [09 · Anubis](09-anubis.md).
