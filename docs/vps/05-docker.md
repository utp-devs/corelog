# 05 · Docker desde el repositorio oficial

← [04 Endurecimiento](04-endurecimiento-del-sistema.md) · [Índice](README.md) · Siguiente: [06 Imagen Nginx](06-imagen-nginx-clang.md)

**Objetivo:** Docker CE instalado **únicamente** desde `download.docker.com`, sin mezclar paquetes de Debian, con logs acotados y sin exponer puertos por accidente.

## 1. Instalación desde el repositorio oficial

```bash
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/debian $VERSION_CODENAME stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker
```

Se instalan solo los cinco paquetes necesarios. No se instalan `docker-ce-rootless-extras` ni `docker-model-plugin` (el script de conveniencia `get.docker.com` los añade): ampliarían la superficie sin aportar nada a este despliegue.

> Evitar `curl … | sh` y `get.docker.com` en servidores: ejecuta como root un script remoto y decide por ti qué instalar. La instalación explícita de arriba es reproducible y auditable.

## 2. Fijar el origen: nada de paquetes Docker de Debian

Debian publica sus propios `docker.io`, `docker-compose`, `containerd` y `runc`, que entran en conflicto con los oficiales. Para que un `apt upgrade` o una dependencia no los instale jamás:

Archivo `/etc/apt/preferences.d/docker-official`:

```
# Docker must come only from download.docker.com. Block the Debian-packaged variants.
Package: docker.io docker-compose docker-doc podman-docker containerd runc
Pin: release o=Debian
Pin-Priority: -1

Package: docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
Pin: origin download.docker.com
Pin-Priority: 900
```

Verificar:

```bash
apt-cache policy docker-ce | sed -n '1,3p'     # Candidate desde download.docker.com
apt-cache policy docker.io | grep Candidate    # Candidate: (none)
dpkg -l | awk '/^ii/ && /docker|containerd/ {print $2, $3}'
```

Todas las versiones deben terminar en `~debian.<N>~<codename>` (compilación de Docker para Debian) y ninguna debe ser `docker.io`.

## 3. Actualizar Docker

```bash
apt-get update
apt-cache policy docker-ce | sed -n '1,3p'          # comparar Installed y Candidate
apt-get install -y --only-upgrade docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin
docker version --format 'server={{.Server.Version}} client={{.Client.Version}}'
docker ps                                           # los contenedores siguen arriba
```

Con `live-restore` (siguiente sección) un reinicio del demonio no detiene los contenedores. Programarlo en una ventana de bajo tráfico de todos modos.

## 4. Configuración del demonio

Archivo: [`recursos/docker/daemon.json`](recursos/docker/daemon.json) → `/etc/docker/daemon.json`.

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "live-restore": true,
  "no-new-privileges": true
}
```

| Opción | Efecto |
|---|---|
| `log-opts max-size/max-file` | Cada contenedor guarda como máximo 3 × 10 MB de logs; sin esto los logs de Docker llenan el disco |
| `live-restore` | Los contenedores siguen vivos mientras se reinicia o actualiza `dockerd` |
| `no-new-privileges` | Ningún proceso en un contenedor puede ganar privilegios (`setuid`) |

Aplicar con `systemctl restart docker` y **recrear** los contenedores existentes: heredan la configuración de logs al crearse, no al reiniciarse.

```bash
docker inspect <contenedor> --format '{{.HostConfig.LogConfig}}'
```

### Variante endurecida (servidor de referencia)

[`recursos/docker/daemon-hardened.json`](recursos/docker/daemon-hardened.json) añade:

| Opción | Efecto | Costo |
|---|---|---|
| `"icc": false` | Los contenedores de la red `bridge` por defecto no se hablan entre sí | Hay que usar redes definidas por Compose |
| `"userns-remap": "default"` | `root` dentro del contenedor es un uid sin privilegios en el host | Los datos pasan a `/var/lib/docker/<uid>.<gid>/`; los *bind mounts* deben ser legibles por el uid remapeado; un contenedor en red `host` o que lee llaves TLS de root necesita `--userns host` (ver [10](10-nginx-como-servicio-del-host.md)) |

No se activó `userns-remap` en el servidor nuevo para no complicar los *bind mounts* de Nginx; es la mejora recomendada si se dispone de tiempo para probarla.

## 5. El grupo `docker` equivale a `root`

Quien puede hablar con `/var/run/docker.sock` puede montar `/` dentro de un contenedor y leerlo o modificarlo todo. Por eso:

- Ninguna persona va en el grupo `docker`; administran como `root`.
- La única excepción es la cuenta de CI/CD ([13](13-usuario-de-despliegue-y-cicd.md)), con llave dedicada, sin reenvíos y solo para el pipeline.
- Alternativas más estrictas: Docker *rootless* para el usuario de CI, o un despliegue por *pull* (un servicio del servidor que consulta el registro y se actualiza) en lugar de por SSH.

```bash
getent group docker      # solo la cuenta de CI
```

## 6. Publicación de puertos

Ver [03](03-firewall-ufw.md): **siempre** `127.0.0.1:<puerto>:<puerto>`. Nginx es el único que escucha en 80/443.

```yaml
ports:
  - "127.0.0.1:18080:8080"   # correcto
  # - "8080:8080"            # expone el puerto a Internet saltándose ufw
```

## 7. Mantenimiento

```bash
docker system df                 # uso de disco
docker image prune -f            # imágenes colgantes (el pipeline lo ejecuta)
docker image ls                  # revisar etiquetas antiguas
```

- Etiquetar las imágenes propias con el SHA del commit; `latest` solo como alias.
- Para imágenes de terceros usar etiquetas de versión concretas (`postgres:17-alpine`), no `latest`, salvo que se asuma el riesgo de cambios no controlados.
- Escanear imágenes de forma periódica (`docker scout cves <imagen>` o `trivy image <imagen>`).

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `docker version` | Cliente y servidor de la misma versión del repo oficial |
| `apt-cache policy docker.io` | `Candidate: (none)` |
| `docker info --format '{{.LiveRestoreEnabled}} {{.LoggingDriver}}'` | `true json-file` |
| `getent group docker` | Solo la cuenta de CI |
| `ss -tlnp \| grep docker-proxy` | Todo en `127.0.0.1` |

Continuar con [06 · Imagen Nginx compilada](06-imagen-nginx-clang.md).
