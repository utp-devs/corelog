# 03 · Firewall (ufw)

← [02 Usuarios y SSH](02-usuarios-permisos-y-ssh.md) · [Índice](README.md) · Siguiente: [04 Endurecimiento del sistema](04-endurecimiento-del-sistema.md)

**Objetivo:** que el servidor solo responda en SSH, HTTP y HTTPS, y que ningún contenedor se exponga por accidente.

## 1. Configuración base

Script: [`recursos/scripts/ufw-setup.sh`](recursos/scripts/ufw-setup.sh).

```bash
ufw default deny incoming
ufw default allow outgoing

ufw allow 22/tcp  comment 'SSH'
ufw allow 80/tcp  comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'

ufw --force enable
ufw status verbose
```

> **Abrir SSH antes de habilitar.** `ufw enable` con la política `deny incoming` y sin la regla del 22 corta la sesión actual y obliga a usar la consola de rescate. Verificar siempre con una **conexión nueva** antes de cerrar la sesión de trabajo.

Resultado esperado:

```
Status: active
Default: deny (incoming), allow (outgoing), deny (routed)

22/tcp   ALLOW IN  Anywhere   # SSH
80/tcp   ALLOW IN  Anywhere   # HTTP
443/tcp  ALLOW IN  Anywhere   # HTTPS
(+ las mismas reglas en IPv6)
```

El puerto 80 solo sirve para redirigir a HTTPS y para el reto ACME si se usa `http-01`; con el certificado wildcard por DNS (documento [12](12-dns-y-certificados-tls.md)) sigue siendo necesario para la redirección.

## 2. La trampa: Docker se salta ufw

Docker inserta sus propias reglas en `iptables` **antes** de las de ufw. Un contenedor publicado con `-p 8080:80` queda abierto a Internet aunque `ufw status` no muestre ese puerto.

Reglas para evitarlo:

1. **Publicar siempre en loopback**: `127.0.0.1:18080:8080`, nunca `8080:8080`.
2. El único componente que escucha en interfaces públicas es Nginx (red `host`, puertos 80/443).
3. Las bases de datos **no publican ningún puerto**; solo se alcanzan por la red interna de Compose.

Verificación (debe listar solo 22, 80 y 443 en `0.0.0.0`, y el resto en `127.0.0.1`):

```bash
ss -tlnpH | awk '{print $4, $6}' | sort -u
```

Ejemplo del servidor documentado:

```
0.0.0.0:22         sshd
0.0.0.0:80         nginx
0.0.0.0:443        nginx
127.0.0.1:18080    docker-proxy   (API)
127.0.0.1:18081    docker-proxy   (frontend)
127.0.0.1:10020    docker-proxy   (Anubis)
127.0.0.1:25       exim4          (correo local, ver documento 04)
```

## 3. Integración con fail2ban

fail2ban usa `banaction = ufw`: cada baneo se convierte en una regla `REJECT` de ufw con el comentario `by Fail2Ban after N attempts against <jail>`. Ver [11](11-fail2ban.md). Los baneos permanentes se acumulan: revisar la lista de vez en cuando con `ufw status numbered | grep -c Fail2Ban`.

## 4. Opcionales

| Medida | Cuándo | Cómo |
|---|---|---|
| Limitar intentos SSH | Si se prefiere ufw a fail2ban | `ufw limit 22/tcp` |
| SSH solo por VPN (WireGuard) | Servidores críticos | `ufw allow in on wg0 to any port 2222 proto tcp` y no abrir el 22 público. **Es la configuración del servidor de referencia; no se aplicó en el nuevo** |
| Aceptar solo IPs de Cloudflare en 80/443 | Si se activa el proxy de Cloudflare | Permitir únicamente los rangos publicados por Cloudflare y configurar `real_ip` en Nginx (documento [12](12-dns-y-certificados-tls.md)) |
| Firewall del proveedor | Siempre que exista | Reglas equivalentes en el panel: una segunda barrera ante un error de ufw |
| Registro | Diagnóstico | `ufw logging low` (por defecto) |

## 5. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `ufw status verbose` | `active`, `deny (incoming)` |
| `ss -tlnpH` | Solo 22/80/443 públicos |
| `nmap -Pn -p- <IP_DEL_VPS>` (desde otro equipo) | Solo 22, 80 y 443 abiertos |
| Conexión SSH nueva | Entra |

Continuar con [04 · Endurecimiento del sistema](04-endurecimiento-del-sistema.md).
