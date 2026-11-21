# 11 · fail2ban y bloqueo permanente

← [10 Nginx como servicio del host](10-nginx-como-servicio-del-host.md) · [Índice](README.md) · Siguiente: [12 DNS y certificados](12-dns-y-certificados-tls.md)

**Objetivo:** que quien ataca quede bloqueado automáticamente: temporalmente si es un error o un exceso, y **de forma permanente** si accede a recursos privados, usa herramientas de ataque o reincide.

## 1. Cómo encaja

```
Nginx ──escribe──► access.log / error.log / attacks.log / geo_blocked.log
                              │
                       fail2ban lee y cuenta coincidencias (filtros) por IP
                              │ supera maxretry en findtime
                              ▼
                 banaction = ufw ──► regla REJECT en el firewall (todos los puertos)
```

Los **filtros** (`filter.d/*.conf`) definen qué línea de log cuenta como infracción; los **jails** (`jail.local`) deciden qué filtro mirar, dónde, cuántas infracciones toleran y cuánto dura el baneo.

## 2. Instalación

```bash
apt-get install -y fail2ban
systemctl enable --now fail2ban
```

El jail `sshd` con `backend = systemd` necesita `python3-systemd` (se instala como dependencia en Debian 13).

## 3. Configuración

Archivos: [`recursos/fail2ban/jail.local`](recursos/fail2ban/jail.local) → `/etc/fail2ban/jail.local` y [`recursos/fail2ban/filter.d/`](recursos/fail2ban/filter.d/) → `/etc/fail2ban/filter.d/`.

### Valores por defecto

```ini
[DEFAULT]
ignoreip = 127.0.0.1/8 ::1 <TU_IP_PUBLICA>
bantime  = 1h
findtime = 10m
maxretry = 5
banaction = ufw
backend  = auto
```

- **`ignoreip`**: tu IP de administración **siempre** debe estar aquí (y la de cualquier equipo desde el que hagas pruebas de ataque que no quieras ver baneado). Sin esto, una prueba de seguridad propia puede dejarte fuera.
- **`banaction = ufw`**: cada baneo es una regla de ufw, con comentario `by Fail2Ban after N attempts against <jail>`, que bloquea todos los puertos de esa IP.

### Jails del servidor

| Jail | Qué vigila | Umbral | Duración |
|---|---|---|---|
| `sshd` | Intentos fallidos o conexiones sospechosas (`mode = aggressive`) | 3 | 24 h |
| `nginx-404` | Exceso de respuestas 404 desde una IP (sondeo de rutas) en `access.log` | 15 en 10 min | 1 h |
| `nginx-limit-req` | IPs que superan los límites de tasa (`limiting requests` en `error.log`) | 5 | 1 h |
| `nginx-modsec` | Bloqueos del WAF (`ModSecurity: Access denied`) | 5 en 10 min | 1 h |
| **`nginx-attack`** | **Accesos a rutas privadas o escáneres** (`attacks.log`) | **3 en 1 h** | **Permanente** |
| `nginx-geoblock` | IPs de países no permitidos que insisten (`geo_blocked.log`) | 10 en 10 min | 1 día |
| **`recidive`** | **Cualquier IP baneada 3 veces en un día** (mira el log de fail2ban) | 3 en 1 d | **Permanente** |

`bantime = -1` significa baneo **sin fecha de fin**.

### Filtros propios

`nginx-attack.conf` — cada línea de `attacks.log` ya es, por construcción, un intento de ataque (el `map` de [07](07-nginx-configuracion.md) solo escribe ahí lo que coincide):

```ini
[Definition]
failregex = ^<HOST> \[
ignoreregex =
```

`nginx-modsec.conf`:

```ini
[Definition]
failregex = \[client <HOST>\] ModSecurity: Access denied
ignoreregex =
```

`nginx-geoblock.conf`:

```ini
[Definition]
failregex = BLOCKED ip=<HOST> country=
ignoreregex =
```

`nginx-404.conf` — cuenta los 404 de `access.log` (se ignoran `favicon.ico`, `robots.txt`, `apple-touch-icon` y `/.well-known/`):

```ini
[Definition]
failregex = ^<HOST> [] "(?:GET|POST|HEAD|PUT|DELETE|PATCH) [^"]*" 404 
ignoreregex = ^<HOST> [] "[A-Z]+ /(?:favicon.ico|robots.txt|apple-touch-icon[^ ]*|.well-known/[^ ]*)[ ?]
```

> **fail2ban quita la fecha antes de aplicar la expresión.** De la línea `1.2.3.4 [21/Nov/2026:16:30:00 -0500] "GET /x HTTP/2.0" 404 …` queda `1.2.3.4 [] "GET /x HTTP/2.0" 404 …`. Una expresión que espere `[[^]]+]` o la fecha completa **nunca coincide**. `nginx-attack` funciona porque solo exige `^<HOST> [`.

> **No usar el jail `nginx-botsearch` estándar con un formato de log propio.** Su expresión asume el formato por defecto de Nginx (`<HOST> - - [fecha] "GET /… " 404`); con un `log_format` distinto el jail queda activo pero **inerte**. Esto ocurrió en el servidor documentado y se detectó al probar el filtro contra el log real. Regla general: **todo filtro se prueba con `fail2ban-regex` contra el log real antes de darlo por válido.**

### Referencias del servidor anterior

[`filter.d/api-attacks-reference.conf`](recursos/fail2ban/filter.d/api-attacks-reference.conf) y [`modsecurity-reference.conf`](recursos/fail2ban/filter.d/modsecurity-reference.conf) son las variantes que usa el servidor de referencia (expresiones sobre `error.log` para `/.env`, `phpunit`, `eval-stdin.php`, `owa/auth`…). Allí además hay jails `nginx-403` (más de 2 respuestas 403 a rutas sensibles en 5 min → 12 h), `nginx-4xx` (más de 20 errores 4xx en 3 min → 30 min) y `global-attacks` (3 ataques en 5 min → 48 h). **No están activos en el servidor nuevo**, que obtiene el mismo efecto con el `map` de Nginx y un único jail permanente.

## 4. Por qué baneo permanente

Una IP que pide `/.env` o `/.git/config` o usa `sqlmap` **no se equivoca**: en una aplicación que no tiene esas rutas, ningún usuario legítimo las pide. Esperar una hora y dejarla volver solo le permite seguir sondeando. Por eso el jail `nginx-attack` y `recidive` son permanentes.

Riesgos y mitigaciones:

| Riesgo | Mitigación |
|---|---|
| Un usuario legítimo pide una ruta del mapa por error (p. ej. un enlace roto a `/backup.sql`) | Umbral de **3** infracciones en una hora, no 1 |
| Tu IP de administración queda baneada tras una prueba | `ignoreip`; o `fail2ban-client unban <IP>` |
| IP compartida (CGNAT, oficina) | Una sola persona con malas intenciones banea a todos; vigilar los baneos y levantarlos si hay quejas |
| Las reglas de ufw se acumulan | Revisar periódicamente: `ufw status numbered \| grep -c Fail2Ban`; las IPs de atacantes cambian con el tiempo |

## 5. Comandos de uso diario

```bash
fail2ban-client status                       # jails activos
fail2ban-client status nginx-attack          # baneadas, total de fallos
fail2ban-client get nginx-attack bantime     # -1 = permanente
fail2ban-client set nginx-attack banip 203.0.113.9      # banear a mano
fail2ban-client unban 203.0.113.9            # levantar un baneo (en todos los jails)
fail2ban-client -t                           # validar la configuración antes de reiniciar
systemctl restart fail2ban                   # tras editar jail.local o filtros
```

Probar un filtro contra una línea de log **sin** activarlo:

```bash
fail2ban-regex '1.2.3.4 [21/Nov/2026:15:50:17 -0500] "GET /.env HTTP/1.1" 444 host="x" ua="y"' \
  /etc/fail2ban/filter.d/nginx-attack.conf
# Lines: 1 lines, 0 ignored, 1 matched, 0 missed
```

> `fail2ban-regex` espera la **línea como argumento** o un **archivo de log**. Pasarla por `stdin` con `-` se interpreta como el texto literal `-` y da `0 matched`, lo que parece un fallo del filtro y no lo es.

## 6. Prueba de extremo a extremo

Desde un equipo **que no esté en `ignoreip`** y dentro del país permitido:

```bash
UA="Mozilla/5.0 Chrome/120"
for p in /.git/config /wp-login.php /backup.sql; do
  curl -s -o /dev/null -m 8 -A "$UA" -w "GET $p -> %{http_code}\n" https://api.example.com$p
done
# 000 000 000 (conexión cerrada sin respuesta)
```

En el servidor:

```bash
tail -3 /var/log/nginx/attacks.log                         # las tres peticiones, status 444
fail2ban-client status nginx-attack                        # Currently banned: 1
ufw status | grep Fail2Ban                                 # REJECT <IP> # by Fail2Ban after 3 attempts…
```

Y desde el equipo de prueba, la siguiente conexión es rechazada (`connection refused`). **Después, levantar el baneo de prueba:**

```bash
fail2ban-client unban <IP_DEL_EQUIPO_DE_PRUEBA>
ufw status | grep -c <IP_DEL_EQUIPO_DE_PRUEBA>             # 0
```

Verificado también `nginx-404`: 18 peticiones a rutas inexistentes desde un equipo del país permitido dieron 404 y el jail baneó la IP (regla `nginx-404` en ufw); tras levantar el baneo la API volvió a responder.

Verificado en el servidor documentado: las tres peticiones se cortaron, el jail baneó la IP de forma permanente (`bantime -1`), la regla apareció en ufw y la IP dejó de recibir respuesta. Un `User-Agent` `sqlmap` también se cortó y quedó registrado.

## 7. Ajustes

- **Más estricto**: bajar `maxretry` de `nginx-attack` a 1–2 si la aplicación no tiene rutas ambiguas.
- **Más permisivo**: subir `findtime`, o cambiar `bantime = -1` por `7d`.
- **Nuevas rutas a bloquear**: se agregan al `map` de Nginx ([07](07-nginx-configuracion.md)); fail2ban las cuenta automáticamente porque lee `attacks.log`.
- **Notificaciones**: la acción `%(action_mwl)s` envía correo con registros; para webhooks, usar una acción propia que llame a `curl`.

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `fail2ban-client -t` | `OK: configuration test is successful` |
| `fail2ban-client status` | Los 7 jails listados |
| `fail2ban-client get nginx-attack bantime` | `-1` |
| `fail2ban-client get sshd ignoreip` | Incluye tu IP |
| Prueba de la sección 6 | Baneo y regla en ufw; luego levantado |

Continuar con [12 · DNS y certificados](12-dns-y-certificados-tls.md).
