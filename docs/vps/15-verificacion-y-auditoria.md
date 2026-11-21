# 15 · Verificación, auditoría y lecciones aprendidas

← [14 Backups y monitoreo](14-backups-monitoreo-y-alertas.md) · [Índice](README.md)

**Objetivo:** comprobar de una vez que el servidor está realmente blindado, saber qué revisar y cada cuánto, y no repetir los errores que ya costaron tiempo.

## 1. Lista de comprobación final

Ejecutar desde el servidor (`root`) y desde un equipo externo. Cada fila remite al documento donde se configura.

### Acceso y usuarios ([02](02-usuarios-permisos-y-ssh.md))

| Comprobación | Esperado |
|---|---|
| `sshd -T \| grep -E '^(passwordauthentication\|authenticationmethods\|permitrootlogin\|allowusers)'` | `no`, `publickey`, `without-password`, lista blanca |
| `ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password usuario@IP` | `Permission denied (publickey)` |
| `getent group sudo` | Vacío |
| `getent group docker` | Solo la cuenta de CI |
| `sudo -l -U <persona>` | `not allowed to run sudo` |
| `awk -F: '$3>=1000 && $3<65000 {print $1}' /etc/passwd` | Solo cuentas conocidas |
| `cat /root/.ssh/authorized_keys` | Solo tu llave |
| `passwd -S <usuario>` | `L` (bloqueada) |

### Red y firewall ([03](03-firewall-ufw.md))

| Comprobación | Esperado |
|---|---|
| `ufw status verbose` | `active`, `deny (incoming)`, solo 22/80/443 |
| `ss -tlnpH` | Públicos: 22, 80, 443; el resto en `127.0.0.1` |
| `nmap -Pn -p- <IP>` (desde fuera) | 22, 80, 443 |

### Sistema ([04](04-endurecimiento-del-sistema.md), [05](05-docker.md))

| Comprobación | Esperado |
|---|---|
| `sysctl net.ipv4.conf.all.rp_filter kernel.kptr_restrict` | `1`, `2` |
| `systemctl is-active unattended-upgrades logrotate.timer fail2ban docker` | Todos `active` |
| `systemctl --failed` | Ninguna |
| `apt-cache policy docker.io \| grep Candidate` | `(none)` |
| `docker info --format '{{.LiveRestoreEnabled}}'` | `true` |

### Nginx, WAF y Anubis ([06](06-imagen-nginx-clang.md)–[09](09-anubis.md))

| Comprobación | Esperado |
|---|---|
| `nginx -V 2>&1 \| head -2` | `nginx/1.31.x`, `built by clang` |
| `docker image inspect nginx-clang:<tag> --format '{{json .Config.Labels}}'` | Revisión y versiones coherentes con el documento 06 |
| `ps -eo user,args \| grep 'nginx: worker'` | Workers como `www-data` (maestro `root`) |
| `systemctl is-active nginx` y `command -v nginx` | `active`, `/usr/local/sbin/nginx` |
| `./check-docs.sh` (en `recursos/nginx-image`) | Sin `OUT OF DATE`: la documentación coincide con el Dockerfile |
| `docker logs nginx 2>&1 \| grep "rules loaded"` | `0/845/0` (o el total de la versión de CRS) |
| `docker logs --tail 5 nginx` y `tail -3 /var/log/nginx/access.log` | La misma petición en ambos |
| `ls -l /var/log/nginx /var/log/modsec_audit.log` | Archivos presentes; el de ModSecurity, de `www-data` |
| `logrotate -d /etc/logrotate.d/nginx-docker` | Considera los 5 logs, sin errores |
| `docker ps --filter name=anubis` | `Up` |

### TLS y DNS ([12](12-dns-y-certificados-tls.md))

| Comprobación | Esperado |
|---|---|
| `certbot certificates` | Un certificado con los 3 nombres, > 14 días |
| `certbot renew --dry-run --no-random-sleep-on-renew` | Éxito |
| `openssl s_client … -servername desconocido.example.com` | `unrecognized name` |

### Reacción ([11](11-fail2ban.md)), respaldo y monitoreo ([14](14-backups-monitoreo-y-alertas.md))

| Comprobación | Esperado |
|---|---|
| `fail2ban-client status` | 7 jails |
| `fail2ban-client get nginx-attack bantime` | `-1` |
| `ls -lh /var/backups/vps` | Backup de las últimas 24 h |
| `cat /var/lib/vps-monitor/last` | Vacío |

## 2. Matriz de pruebas de ataque

Desde un equipo **del país permitido** y **fuera de `ignoreip`** (levantar después cada baneo con `fail2ban-client unban <IP>`):

| Prueba | Esperado | Capa que actúa |
|---|---|---|
| `curl https://app.example.com/` (UA de navegador) | 307 → desafío | Anubis |
| `curl -A sqlmap https://app.example.com/` | 403 | Anubis (política) |
| `curl -A "" https://app.example.com/` | 403 | Anubis (sin UA) |
| `curl "https://api.example.com/?id=1' OR 1=1--"` | 403 | ModSecurity |
| `curl "https://api.example.com/?q=<script>alert(1)</script>"` | 403 | ModSecurity |
| `curl https://api.example.com/.git/config` (×3) | Sin respuesta, luego IP baneada | `block-attacks` + fail2ban |
| `curl -A sqlmap https://api.example.com/api/v1/…` | Sin respuesta | `block-attacks` (User-Agent) |
| 25 × `POST /api/v1/auth/login` seguidos | 401 y luego 429, después baneo | `limit_req` + fail2ban |
| 18 × `GET /ruta-inexistente-N` | 404 y luego baneo | `nginx-404` |
| Petición desde un país no permitido | Sin respuesta | GeoIP |
| TLS a un host sin vhost | `unrecognized name` | `ssl_reject_handshake` |
| `ssh` con contraseña | `Permission denied (publickey)` | `sshd` |
| `curl` de un endpoint protegido sin token | 401 | Aplicación |

Tras la matriz: `fail2ban-client unban <IP_DE_PRUEBA>` y `ufw status | grep -c <IP_DE_PRUEBA>` → `0`.

## 3. Mantenimiento periódico

| Frecuencia | Tarea |
|---|---|
| Diaria (automática) | Backup 03:00, monitoreo cada 5 min, rotación de logs, actualizaciones de seguridad |
| Semanal | Revisar `journalctl -t vps-monitor`, baneos de fail2ban, `attacks.log`, `modsec_audit.log` (falsos positivos), `docker system df` |
| Mensual | `apt list --upgradable`, reiniciar si hay kernel nuevo, actualizar la base GeoLite2, revisar versiones de Docker y de la imagen de Anubis, revisar reglas de ufw acumuladas |
| Trimestral | **Restauración de prueba** de la base de datos, `lynis audit system`, revisar usuarios/grupos/llaves, probar la matriz de ataques |
| Semestral | Rotar la llave de despliegue y el token de Cloudflare; reconstruir `nginx-clang` con parches (o antes si hay una CVE de Nginx/OpenSSL/PCRE2) |
| Al salir una persona del equipo | Quitar su llave de `authorized_keys`, rotar secrets a los que tuvo acceso, auditar como en [02](02-usuarios-permisos-y-ssh.md) |

## 4. Respuesta rápida a incidentes

| Situación | Acción inmediata |
|---|---|
| IP atacante que sigue activa | `fail2ban-client set nginx-attack banip <IP>` (permanente) |
| Falso positivo / IP propia baneada | `fail2ban-client unban <IP>` y añadirla a `ignoreip` |
| Filtración de la llave de CI | Rotarla (13, sección 7), auditar el servidor, revisar `docker ps -a`, imágenes y `authorized_keys` |
| Filtración del token de Cloudflare | Revocarlo y crear otro (12, sección 3); revisar registros DNS |
| Filtración de `.env` | Cambiar `DB_PASSWORD` (en la base y en el `.env`) y `JWT_SECRET` (cierra las sesiones) y recrear contenedores |
| Servidor comprometido | Aislar (firewall del proveedor), conservar evidencia (`journalctl`, `docker logs`), reconstruir desde cero y restaurar **solo datos** de un backup anterior al incidente; rotar **todos** los secretos |
| Sitio caído tras un cambio | Revertir la configuración, `nginx -t && systemctl reload nginx`; si fue la imagen, volver a la etiqueta anterior en `nginx-container` y `systemctl restart nginx` ([16](16-mantenimiento-y-versionado-de-la-imagen.md)); la imagen anterior sigue cargada |

## 5. Lecciones aprendidas (problemas reales y su solución)

| Problema | Causa | Solución |
|---|---|---|
| Edité `nginx.conf`, hice `reload` y nada cambió | *Bind mount* de un **archivo**: `sed -i` crea un archivo nuevo y el contenedor sigue viendo el viejo | `docker compose up -d --force-recreate` tras editar archivos montados ([07](07-nginx-configuracion.md)) |
| Un script remoto se cortó a la mitad sin error | `docker compose exec` consumió el resto del script como `stdin` | `docker compose exec -T … </dev/null` en scripts |
| `curl https://127.0.0.1/` falla en el *handshake* TLS (alerta `unrecognized name`) | El `default_server` rechaza el handshake sin SNI (comportamiento buscado) | Probar con `--resolve host:443:127.0.0.1` |
| El log de auditoría de ModSecurity estaba vacío | Los *workers* corren como `nobody` y el archivo era de `root` | `chown nobody:nogroup` y `chmod 640/750` ([08](08-waf-modsecurity-crs.md)) |
| `logrotate` nunca rotaba nada | No viene instalado en la imagen de Debian de la nube; la prueba falló en silencio | Instalarlo, activar el temporizador y probar con `logrotate -f` ([04](04-endurecimiento-del-sistema.md)) |
| Tras rotar, Nginx seguía escribiendo en el archivo viejo | Faltaba `nginx -s reopen` / el log de ModSecurity necesita `copytruncate` | `postrotate` y bloque `copytruncate` propios ([07](07-nginx-configuracion.md)) |
| Anubis se reiniciaba sin parar con `permission denied` | `umask 077` dejó `politica.yaml` en `600` y el contenedor no es root | `chmod 644` ([09](09-anubis.md)) |
| El jail `nginx-botsearch` estaba activo pero **no hacía nada** | Su expresión espera el formato de log estándar; el mío es propio | Filtro propio `nginx-404`, probado con `fail2ban-regex` contra el log real ([11](11-fail2ban.md)) |
| Un filtro de fail2ban "no coincidía" | fail2ban **elimina la fecha** antes de aplicar la expresión (quedan `[]`) | Escribir la expresión sobre la línea sin fecha |
| `fail2ban-regex -` daba 0 coincidencias | `-` se toma como texto literal | Pasar la línea como argumento o un archivo |
| Mi propia prueba me baneó la IP del servidor de pruebas | Cumplí el umbral del jail | `ignoreip` para administración y `fail2ban-client unban` tras las pruebas |
| Puerto publicado por Docker accesible desde Internet | Docker se salta ufw | Publicar siempre `127.0.0.1:puerto` ([03](03-firewall-ufw.md)) |
| `sshd` ignoraba mi configuración | `sshd_config.d` se lee en orden alfabético y gana el primer valor; el archivo del proveedor (`50-…`) iba antes | Nombrar el propio `10-…` ([02](02-usuarios-permisos-y-ssh.md)) |
| Un colaborador con `sudo` temporal dejó cambios no pedidos | Privilegios totales sin auditoría posterior | Auditoría obligatoria al revocar ([02](02-usuarios-permisos-y-ssh.md)) |
| La verificación del despliegue fallaba siempre | GeoIP bloquea a los *runners* de GitHub (fuera del país) | Verificar desde dentro del servidor por SSH ([13](13-usuario-de-despliegue-y-cicd.md)) |
| `pinned-actions` fallaba en el PR | El repositorio exige acciones fijadas por SHA, no por etiqueta | Fijar `uses: org/accion@<sha> # vX.Y.Z` ([13](13-usuario-de-despliegue-y-cicd.md)) |
| `certbot renew --dry-run` parecía colgado | Espera aleatoria de hasta 8 min en modo no interactivo | `--no-random-sleep-on-renew` ([12](12-dns-y-certificados-tls.md)) |
| El monitoreo mostraba `● ●` como unidades fallidas | `systemctl --failed` imprime un símbolo `●` que contaba como nombre | `--plain --no-legend --no-pager` ([14](14-backups-monitoreo-y-alertas.md)) |
| `cloud-init` aparecía como fallido sin haber tocado nada | `bootcmd` del aprovisionamiento del proveedor en el primer arranque | Entenderlo con `cloud-init status --long` y `systemctl reset-failed` ([01](01-requisitos-y-primeros-pasos.md)) |
| La red de la oficina "bloqueaba" el dominio nuevo | Filtro web que no categoriza dominios recién creados | Probar desde otra red; pedir la recategorización ([12](12-dns-y-certificados-tls.md)) |
| `docker` instalado con `get.docker.com` trajo paquetes extra | El script instala `rootless-extras` y `model-plugin` | Instalación explícita desde el repositorio y *pin* de apt ([05](05-docker.md)) |
| Clang compila pero **todo enlace falla** (`crtbeginS.o`, `-lgcc_s`) | El LLVM descargado no trae el *runtime* de GCC; falta `libgcc-N-dev` | Instalar `libgcc-N-dev` y `libstdc++-N-dev` en el toolchain; reproducible con `repro-clang.sh` ([16](16-mantenimiento-y-versionado-de-la-imagen.md)) |
| `tar --zstd` falla con `Window size larger than maximum` al abrir LLVM | Los `.tar.zst` de LLVM usan una ventana de 1 GB | `zstd -d --long=31 | tar -x` ([16](16-mantenimiento-y-versionado-de-la-imagen.md)) |
| Un `! comando` dentro de `set -e` no detenía el build | `set -e` ignora los comandos negados con `!` | `if comando; then exit 1; fi` ([16](16-mantenimiento-y-versionado-de-la-imagen.md)) |
| `docker build -q --progress=plain` no hace nada | Las dos opciones son incompatibles (error inmediato, tiempo medido 0 s) | Usar solo una; comprobar siempre que un benchmark **realmente corrió** |
| El script copiado por SSH quedó vacío | `cat > f && … &` en segundo plano pierde `stdin` (los procesos en segundo plano leen de `/dev/null`) | Copiar primero, de forma síncrona, y lanzar después |
| `pkill -f nombre` mató mi propia sesión SSH | El patrón coincide con la línea de comando del propio `ssh`/shell | Usar el PID o un patrón que no aparezca en el comando que lo ejecuta |
| Cuatro procesos `nginx` como `uuidd` junto a los míos | Es el contenedor oficial de la aplicación web (uid 101); el host muestra el nombre de la cuenta que tiene ese uid | Identificarlos por su proceso maestro y línea de comando ([10](10-nginx-como-servicio-del-host.md)) |
| Un benchmark de Meson fallaba en 0,2 s | Opciones que no existen en ese proyecto (`lzma`, `programs`, `tests`) y falta de `git` | Listar las opciones reales (`meson configure`) y validar que la medición ejecuta de verdad |

## 6. Estado de verificación de cada documento

| Documento | Dónde se verificó |
|---|---|
| 01–05, 08, 09, 11–14 | Servidor nuevo (Debian 13) con tráfico real y ataques simulados |
| 06 (imagen) | **Construida y probada en el servidor nuevo** (`nginx-clang:1.31.9-r2`): 16 comprobaciones de integridad, 2 firmas GPG, `smoke-test.sh` 15/15, enlace estático verificado, mediciones de tiempo reales |
| 07 (configuración) | Servidor nuevo, ya con el esquema del sistema (`/etc/nginx`, workers `www-data`) |
| 10, modo A (Compose) | Servidor nuevo, hasta la migración |
| 10, modo B (systemd + envoltorio) | **Servidor nuevo (migrado, corte de 1,5 s) y servidor de referencia**, ambos en producción |
| 16 (mantenimiento) | Scripts ejecutados en el servidor nuevo: `check-updates.sh`, `check-docs.sh` (incluida una prueba negativa de desfase), `build.sh`/`smoke-test.sh`, `repro-clang.sh` y las mediciones de velocidad |
| Variantes opcionales (`userns-remap`, SSH por WireGuard, criptografía SSH restringida, jails `nginx-403/4xx/global-attacks`) | Solo en el servidor de referencia |

Lo que **no** se ha podido comprobar y debe hacerse a mano: el flujo completo en un **navegador real** (desafío de Anubis, registro e inicio de sesión), la **restauración** de un backup en un entorno limpio y el **reinicio completo del servidor** con Nginx como servicio systemd (está habilitado, pero no se reinició la máquina).

← Volver al [índice](README.md)
