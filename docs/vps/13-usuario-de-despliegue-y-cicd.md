# 13 · Usuario de despliegue y CI/CD

← [12 DNS y certificados](12-dns-y-certificados-tls.md) · [Índice](README.md) · Siguiente: [14 Backups y monitoreo](14-backups-monitoreo-y-alertas.md)

**Objetivo:** que GitHub Actions construya, publique y despliegue la aplicación en el VPS sin dar a la automatización más acceso del necesario. El flujo del pipeline en sí está en [`../despliegue.md`](../despliegue.md); aquí se documenta **la parte del servidor y la seguridad**.

## 1. Modelo

```
push a main ─► GitHub Actions
                 ├─ build-and-push: construye imágenes y las sube a GHCR (tag = SHA del commit)
                 └─ deploy (entorno "production"):
                      SSH (llave dedicada, huella fijada) ──► usuario `github` en el VPS
                      docker compose pull && up -d en /opt/<app>
                      verificación desde DENTRO del servidor
```

El usuario `github` **no tiene `sudo`**, no puede abrir túneles y su único poder es ejecutar Docker. Ese poder equivale a `root` (ver el riesgo en la sección 5): se limita lo que se puede, y se asume el resto.

## 2. Preparar el usuario en el servidor

```bash
useradd -m -s /bin/bash github
passwd -l github
usermod -aG docker github                 # único usuario en este grupo
install -d -m 750 -o github -g github /opt/corelog
```

Datos de la aplicación (`/opt/corelog/.env`, solo lectura para su dueño; **nunca** en git):

```bash
umask 077
cat > /opt/corelog/.env <<EOF
DB_USER=corelog
DB_NAME=corelog
DB_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET=$(openssl rand -hex 48)
CORS_ALLOWED_ORIGIN=https://app.example.com
EOF
chown github:github /opt/corelog/.env && chmod 600 /opt/corelog/.env
```

Las contraseñas y el secreto JWT se generan aleatoriamente en el propio servidor y nunca pasan por el repositorio.

## 3. Llave de despliegue

La llave se genera **en un directorio temporal local**, se instala la pública en el servidor, la privada se guarda **solo** como secret de GitHub y se borra del equipo:

```bash
D=$(mktemp -d)
ssh-keygen -t ed25519 -C "github-actions-app" -f "$D/deploy" -N ""

# pública → servidor
ssh root@<IP_DEL_VPS> "install -d -m 700 -o github -g github /home/github/.ssh && \
  cat > /home/github/.ssh/authorized_keys && chown github:github /home/github/.ssh/authorized_keys && \
  chmod 600 /home/github/.ssh/authorized_keys" < "$D/deploy.pub"

# huella del servidor (para que el pipeline detecte un servidor suplantado)
ssh-keyscan -t ed25519 -p 22 <IP_DEL_VPS> > "$D/known_hosts"
```

En `sshd` ya está limitada ([02](02-usuarios-permisos-y-ssh.md)):

```
Match User github
    AllowTcpForwarding no
    X11Forwarding no
    AllowAgentForwarding no
```

## 4. Secrets y variables en GitHub

Con `gh` autenticado y permisos de administración del repositorio:

```bash
R=<org>/<repo>
gh api -X PUT repos/$R/environments/production --silent          # crea el entorno

gh secret set VPS_SSH_KEY     --env production -R $R < "$D/deploy"
gh secret set VPS_KNOWN_HOSTS --env production -R $R < "$D/known_hosts"
gh secret set VPS_HOST        --env production -R $R --body "<IP_DEL_VPS>"
gh secret set VPS_USER        --env production -R $R --body "github"
gh variable set WEB_BUILD_CONFIGURATION -R $R --body "production"

shred -u "$D/deploy" "$D/deploy.pub" 2>/dev/null || rm -f "$D/deploy" "$D/deploy.pub"
rm -rf "$D"
gh secret list --env production -R $R
```

| Nombre | Tipo | Uso |
|---|---|---|
| `VPS_HOST`, `VPS_USER` | Secret (entorno `production`) | Destino SSH |
| `VPS_SSH_KEY` | Secret (entorno `production`) | Llave privada de despliegue |
| `VPS_KNOWN_HOSTS` | Secret (entorno `production`) | Huella del servidor (`StrictHostKeyChecking yes`) |
| `WEB_BUILD_CONFIGURATION` | Variable de repositorio | Configuración de entorno de Angular al compilar (por defecto `production`) |

Los secrets del **entorno** solo los ven los jobs que declaran `environment: production`; y el entorno admite revisores obligatorios (sección 6).

## 5. El riesgo: `docker` equivale a `root`

Quien controle la llave de `github` controla el servidor. Medidas aplicadas y pendientes:

| Medida | Estado |
|---|---|
| Llave exclusiva para CI, nunca reutilizada | Aplicada |
| Sin `sudo`, sin túneles ni reenvío de agente | Aplicada |
| Huella del servidor fijada en el pipeline | Aplicada |
| Secrets limitados al entorno `production` | Aplicada |
| El workflow solo corre en `main` (rama protegida: PR + revisión + checks) | Aplicada |
| Acciones de terceros fijadas por SHA (el repositorio lo exige con un check) | Aplicada |
| Revisores obligatorios en el entorno `production` | **Recomendada**, pendiente |
| Despliegue por *pull* (el servidor consulta el registro) o Docker *rootless* | Alternativa más estricta, no aplicada |
| `command="…"` restringido en `authorized_keys` | No aplicada (el pipeline usa `scp` y varios comandos) |

Si la llave se filtra: rotarla de inmediato (sección 7) y auditar el servidor como en [02](02-usuarios-permisos-y-ssh.md), sección 6.

> **Repositorios públicos.** Los workflows y el modelo de despliegue son visibles. Los secrets no, pero cualquiera puede leer *cómo* se despliega: por eso la protección de rama y los revisores obligatorios importan.

## 6. Puntos del pipeline que dependen del servidor

1. **Verificación desde dentro del servidor.** El filtro GeoIP ([07](07-nginx-configuracion.md)) bloquea a los *runners* de GitHub (fuera del país permitido). El paso "Verify deployment" consulta `http://127.0.0.1:18080/` **por SSH** en lugar de la URL pública.
2. **Huella fijada**, no `ssh-keyscan` en cada ejecución: el pipeline usa `VPS_KNOWN_HOSTS`.
3. **Contraseñas por *stdin*.** El inicio de sesión en GHCR pasa el token por `--password-stdin` y se hace `docker logout` al terminar.
4. **Puertos.** El compose de producción publica `api` y `ui` solo en `127.0.0.1` ([03](03-firewall-ufw.md)).
5. **Migraciones.** El servicio `migrate` corre antes de la API y debe ser un job de una sola vez (sale con código 0); el monitoreo lo excluye de las alertas de "contenedor caído".
6. **Entorno `production` con aprobación manual.** En *Settings → Environments → production → Required reviewers*: un merge a `main` construye, pero el despliegue espera a que alguien lo apruebe.
7. **Protección de rama.** `main` exige PR, aprobación y que pasen los checks (backend, frontend, CodeQL, gitleaks, dependency-review, pinned-actions…).

## 7. Rotar la llave de despliegue

```bash
D=$(mktemp -d)
ssh-keygen -t ed25519 -C "github-actions-app" -f "$D/deploy" -N ""
ssh root@<IP_DEL_VPS> "cat > /home/github/.ssh/authorized_keys" < "$D/deploy.pub"     # reemplaza la anterior
gh secret set VPS_SSH_KEY --env production -R <org>/<repo> < "$D/deploy"
shred -u "$D/deploy" "$D/deploy.pub"; rm -rf "$D"
```

Lanzar un despliegue de prueba (*workflow_dispatch*) para confirmar que entra con la llave nueva. Hacerlo cada 6–12 meses y tras la salida de cualquier persona con acceso a los secrets.

## 8. Revertir un despliegue

Ver [`../despliegue.md`](../despliegue.md#rollback): cada imagen está etiquetada con su SHA; `IMAGE_TAG=<sha> docker compose up -d` vuelve a una versión anterior.

## 9. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `id github` | Solo grupos `github` y `docker` |
| `sudo -l -U github` | `not allowed to run sudo` |
| `ls -l /opt/corelog/.env` | `-rw------- github github` |
| `gh secret list --env production -R <org>/<repo>` | Los cuatro secrets |
| Ejecutar el workflow | `build-and-push` y `deploy` en verde |
| `docker compose -f /opt/corelog/docker-compose.yml ps` | `api`, `ui`, `db` arriba; `migrate` terminado (0) |
| Llave privada en el equipo local | No existe |

Continuar con [14 · Backups, monitoreo y alertas](14-backups-monitoreo-y-alertas.md).
