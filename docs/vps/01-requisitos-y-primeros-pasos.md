# 01 · Requisitos y primeros pasos

← [Índice](README.md) · Siguiente: [02 Usuarios, permisos y SSH](02-usuarios-permisos-y-ssh.md)

## 1. Requisitos

| Elemento | Mínimo recomendado |
|---|---|
| VPS | Debian 12 o 13, 2 vCPU, 4 GB RAM, 40 GB de disco, IPv4 pública |
| Dominio | Gestionado en Cloudflare (se usa su API para el certificado wildcard) |
| Equipo local | OpenSSH, `git`, acceso a la consola web del proveedor (rescate) |
| Cuenta de GitHub | Con permisos de administración del repositorio (secrets y entornos) |

Antes de empezar, anotar: IP del VPS, usuario inicial que entrega el proveedor y el método de acceso (llave o contraseña).

> **Regla de oro.** Nunca cerrar la sesión de `root` que está abierta hasta haber comprobado, en una **sesión nueva**, que el acceso configurado funciona. Un `sshd_config` o un `ufw` mal aplicado sin sesión de respaldo obliga a usar la consola de rescate.

## 2. Primer acceso

```bash
ssh root@<IP_DEL_VPS>          # o el usuario que indique el proveedor
cat /etc/os-release | head -3  # confirmar la versión de Debian
```

Si el proveedor aprovisiona con *cloud-init*, puede existir un usuario por defecto (por ejemplo `debian`) con `sudo` sin contraseña. Se neutraliza en el documento [02](02-usuarios-permisos-y-ssh.md).

## 3. Actualizar el sistema

```bash
apt-get update
apt-get -y full-upgrade
systemctl reboot            # si se instaló un kernel nuevo
```

Tras el reinicio, comprobar que el kernel en uso es el más reciente:

```bash
uname -r
dpkg -l 'linux-image-*' | awk '/^ii/ {print $2, $3}'
```

## 4. Zona horaria y hora

La zona horaria determina la hora de los logs, de `cron` y de los backups. Configurarla desde el inicio evita confusiones al correlacionar eventos.

```bash
timedatectl set-timezone America/Lima
timedatectl            # "System clock synchronized: yes" debe aparecer
```

Los contenedores heredan la hora montando `/etc/localtime` en solo lectura (ver [07](07-nginx-configuracion.md)).

## 5. Nombre de host

```bash
hostnamectl set-hostname <nombre>      # opcional: el proveedor ya asigna uno
```

## 6. Paquetes base

```bash
apt-get install -y ca-certificates curl gnupg ufw fail2ban logrotate \
  certbot python3-certbot-dns-cloudflare unattended-upgrades htop
```

> **Atención con `logrotate`.** Las imágenes mínimas de Debian en la nube **no lo traen instalado**. Sin él, ninguna configuración de `/etc/logrotate.d/` se ejecuta y los logs crecen sin límite; además, `logrotate -d` falla en silencio si se oculta el error. Instalarlo explícitamente y confirmar `systemctl is-active logrotate.timer`.

## 7. Revisar unidades fallidas del arranque

En algunos proveedores el primer arranque deja unidades de *cloud-init* en error por un `bootcmd` del aprovisionamiento. Es ajeno a la configuración propia, pero hay que conocerlo para no confundirlo con un fallo posterior:

```bash
systemctl --failed
cloud-init status --long          # muestra la causa
systemctl reset-failed            # limpia el estado una vez entendido
```

El script de monitoreo del documento [14](14-backups-monitoreo-y-alertas.md) alerta cuando aparece una unidad fallida, por lo que conviene partir de un estado limpio.

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `uname -r` | Kernel más reciente instalado |
| `timedatectl` | Zona correcta y reloj sincronizado |
| `systemctl is-active logrotate.timer` | `active` |
| `systemctl --failed` | Sin unidades (o entendidas) |
| `apt list --upgradable 2>/dev/null \| wc -l` | `1` (solo la cabecera) |

Continuar con [02 · Usuarios, permisos y SSH](02-usuarios-permisos-y-ssh.md).
