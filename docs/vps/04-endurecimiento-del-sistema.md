# 04 · Endurecimiento del sistema

← [03 Firewall](03-firewall-ufw.md) · [Índice](README.md) · Siguiente: [05 Docker](05-docker.md)

**Objetivo:** reducir la superficie de ataque del propio sistema operativo: parámetros de kernel y red, actualizaciones automáticas, servicios innecesarios, memoria y rotación de logs.

## 1. Parámetros de kernel y red (`sysctl`)

Archivo: [`recursos/sysctl/99-hardening.conf`](recursos/sysctl/99-hardening.conf) → `/etc/sysctl.d/99-hardening.conf`.

```bash
install -m 644 99-hardening.conf /etc/sysctl.d/99-hardening.conf
sysctl --system
```

| Parámetro | Valor | Efecto |
|---|---|---|
| `net.ipv4.tcp_syncookies` | 1 | Protege de inundaciones SYN |
| `net.ipv4.conf.*.rp_filter` | 1 | Descarta paquetes con IP de origen falsificada |
| `net.ipv4/6.conf.*.accept_redirects`, `send_redirects` | 0 | Ignora redirecciones ICMP (desvío de tráfico) |
| `net.ipv4/6.conf.*.accept_source_route` | 0 | Rechaza enrutamiento de origen |
| `net.ipv4.conf.all.log_martians` | 1 | Registra paquetes con direcciones imposibles |
| `net.ipv4.icmp_echo_ignore_broadcasts`, `icmp_ignore_bogus_error_responses` | 1 | Evita amplificación y ruido ICMP |
| `net.ipv4.tcp_rfc1337` | 1 | Protege de asesinatos de conexiones en `TIME_WAIT` |
| `kernel.kptr_restrict` | 2 | Oculta direcciones del kernel |
| `kernel.dmesg_restrict` | 1 | Solo root lee `dmesg` |
| `kernel.yama.ptrace_scope` | 1 | Un proceso solo puede depurar a sus hijos |
| `kernel.unprivileged_bpf_disabled`, `net.core.bpf_jit_harden` | 1 / 2 | Reduce la superficie de eBPF |
| `fs.protected_hardlinks`, `protected_symlinks` | 1 | Mitiga ataques en `/tmp` |
| `fs.suid_dumpable` | 0 | Sin volcados de memoria de binarios SUID |
| `vm.swappiness` | 10 | Prefiere RAM; el swap queda de colchón |

**No se toca `net.ipv4.ip_forward`:** Docker lo necesita para enrutar el tráfico de los contenedores.

El servidor de referencia añade `fs.protected_fifos=2`, `fs.protected_regular=2` y `net.ipv6.conf.*.accept_ra=0` (este último solo cuando no se usa autoconfiguración IPv6).

## 2. Actualizaciones automáticas de seguridad

```bash
apt-get install -y unattended-upgrades
cat /etc/apt/apt.conf.d/20auto-upgrades
# APT::Periodic::Update-Package-Lists "1";
# APT::Periodic::Unattended-Upgrade "1";
systemctl is-active unattended-upgrades
```

Instala parches de seguridad sin intervención. Dos consideraciones:

- **Kernel:** una actualización de kernel exige reiniciar. Programar el reinicio en una ventana conocida, o activar `Unattended-Upgrade::Automatic-Reboot "true"` con `Automatic-Reboot-Time "04:00"` si el servicio tolera un corte breve. Con `live-restore` y `restart: unless-stopped` los contenedores vuelven solos.
- **Docker:** los paquetes del repositorio oficial se actualizan con `apt` a mano (ver [05](05-docker.md)) para controlar cuándo se reinicia el demonio.

Comprobar tras un reinicio que el kernel en uso es el más nuevo (`uname -r` frente a `dpkg -l 'linux-image-*'`). El servidor de referencia automatiza esa comprobación con un script de verificación de arranque.

## 3. Swap

Con 8 GB de RAM no es necesario, pero un colchón evita que el OOM-killer mate un contenedor ante un pico:

```bash
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
swapon --show
```

## 4. Servicios innecesarios

```bash
systemctl list-unit-files --state=enabled --no-legend | awk '{print $1}'
ss -tlnp
```

Cada servicio escuchando es superficie de ataque. En la imagen de Debian usada aparecía `exim4` (correo local) escuchando en `127.0.0.1:25`; al ser solo loopback no se expone, pero si no se necesita:

```bash
systemctl disable --now exim4
```

`systemd-resolved` escucha en `5355` (LLMNR) en todas las interfaces; ufw lo bloquea, pero se puede desactivar con `LLMNR=no` en `/etc/systemd/resolved.conf` si se quiere eliminar del todo.

## 5. Rotación de logs

`logrotate` debe estar instalado y su temporizador activo (ver [01](01-requisitos-y-primeros-pasos.md)):

```bash
systemctl enable --now logrotate.timer
logrotate -d /etc/logrotate.d/nginx-docker      # simulación; mostrar el resultado completo
```

La configuración de los logs de Nginx está en [07](07-nginx-configuracion.md) y [`recursos/logrotate/nginx-docker`](recursos/logrotate/nginx-docker).

## 6. Permisos de archivos sensibles

| Ruta | Permisos | Contenido |
|---|---|---|
| `/root/.secrets/` | `700` | Token de Cloudflare |
| `/root/.secrets/cloudflare.ini` | `600` | Token |
| `/opt/anubis/.env` | `600` | Llave privada de Anubis |
| `/opt/<app>/.env` | `600`, dueño del usuario de CI | Credenciales de la aplicación |
| `/var/backups/vps/` | `700` | Backups (contienen secretos y llaves TLS) |
| `/home/<usuario>` | `700` | Datos de cada cuenta |

> **Cuidado con `umask`.** Un `umask 077` activo en la sesión (por ejemplo tras crear un archivo secreto) hace que los **siguientes** archivos creados también queden en `600`. Un contenedor sin privilegios no podrá leer su propia política, y el síntoma es un `permission denied` poco evidente. Tras crear archivos de configuración que otro proceso debe leer, revisar `ls -l` y corregir con `chmod 644`.

## 7. Herramientas de auditoría (opcional)

| Herramienta | Uso | Estado |
|---|---|---|
| `lynis audit system` | Índice de endurecimiento y sugerencias | No instalada en el servidor documentado |
| `auditd` | Registro de llamadas al sistema sobre archivos críticos | No instalada |
| `aide` | Detección de cambios en archivos | No instalada |
| `rkhunter` | Búsqueda de rootkits | No instalada |

Se recomienda al menos `lynis` de forma periódica (trimestral) y tras cambios grandes.

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `sysctl net.ipv4.conf.all.rp_filter kernel.kptr_restrict` | `1` y `2` |
| `systemctl is-active unattended-upgrades` | `active` |
| `swapon --show` | `/swapfile 2G` |
| `systemctl is-active logrotate.timer` | `active` |
| `ss -tlnp` | Sin servicios inesperados |

Continuar con [05 · Docker](05-docker.md).
