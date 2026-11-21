# 02 · Usuarios, permisos y SSH

← [01 Primeros pasos](01-requisitos-y-primeros-pasos.md) · [Índice](README.md) · Siguiente: [03 Firewall](03-firewall-ufw.md)

**Objetivo:** que solo `root` tenga privilegios, que cada persona tenga su propia cuenta sin `sudo` y que el único método de acceso sea una llave SSH.

## 1. Modelo de privilegios

| Cuenta | Privilegios | Acceso SSH |
|---|---|---|
| `root` | Total | Solo con llave (`prohibit-password`) |
| Personas (`admin1`, `admin2`…) | Ninguno: grupo propio, sin `sudo`, sin `docker` | Solo con llave |
| `github` (CI/CD) | Solo ejecutar Docker en `/opt/<app>` (ver [13](13-usuario-de-despliegue-y-cicd.md)) | Solo con llave dedicada, sin reenvíos |
| Cuenta del proveedor (`debian`) | Ninguno (se neutraliza) | Bloqueada |

Principio: **un usuario nuevo debe parecer recién creado**: `uid` propio, un único grupo (el suyo), home en `700`, sin ACL, sin SUID, sin cron.

```bash
useradd -m -s /bin/bash admin1
passwd -l admin1                 # sin contraseña utilizable: solo entra con llave
id admin1                        # uid=…(admin1) gid=…(admin1) groups=…(admin1)
ls -ld /home/admin1              # drwx------
```

## 2. Generar las llaves (en el equipo local)

Una llave **por servidor y por usuario**, con nombre descriptivo y sin reutilizar:

```bash
ssh-keygen -t ed25519 -C "vps_root" -f ~/.ssh/vps/vps_root -N ""
# Con passphrase (recomendado en equipos personales): quitar -N "" y escribirla al generar
```

Entrada en `~/.ssh/config` del equipo local:

```
Host vps-root
    HostName <IP_DEL_VPS>
    User root
    IdentityFile ~/.ssh/vps/vps_root
    PreferredAuthentications publickey
    IdentitiesOnly yes
    Port 22
```

Cargar la llave **pública** en el servidor con los permisos exactos que exige `sshd`:

```bash
install -d -m 700 -o admin1 -g admin1 /home/admin1/.ssh
echo "ssh-ed25519 AAAA… comentario" >> /home/admin1/.ssh/authorized_keys
chown admin1:admin1 /home/admin1/.ssh/authorized_keys
chmod 600 /home/admin1/.ssh/authorized_keys
```

La llave privada nunca sale del equipo del dueño ni se pega en chats o tickets.

## 3. Endurecer `sshd`

Archivo: [`recursos/ssh/10-hardening.conf`](recursos/ssh/10-hardening.conf) → `/etc/ssh/sshd_config.d/10-hardening.conf`.

```
AuthenticationMethods publickey
PermitRootLogin prohibit-password
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitEmptyPasswords no
PubkeyAuthentication yes
MaxAuthTries 3
LoginGraceTime 30
MaxStartups 10:30:60
X11Forwarding no
AllowAgentForwarding no
ClientAliveInterval 300
ClientAliveCountMax 2
AllowUsers root admin1 admin2 github

Match User github
    AllowTcpForwarding no
    X11Forwarding no
    AllowAgentForwarding no
```

| Directiva | Por qué |
|---|---|
| `AuthenticationMethods publickey` | Cierra cualquier otra vía, incluso si una directiva se reactivara por error |
| `PasswordAuthentication no` / `KbdInteractiveAuthentication no` | Sin contraseñas por SSH |
| `PermitRootLogin prohibit-password` | `root` solo con llave |
| `MaxAuthTries 3`, `LoginGraceTime 30` | Reduce la ventana de fuerza bruta |
| `MaxStartups 10:30:60` | Limita conexiones no autenticadas simultáneas |
| `AllowUsers` | Lista blanca: cualquier otra cuenta (incluida `debian`) no puede entrar |
| `Match User github` | La cuenta de CI no puede hacer túneles ni reenviar el agente |

> **Orden de lectura.** `sshd` procesa `sshd_config.d/*.conf` en orden alfabético y **el primer valor leído gana**. El archivo del proveedor (`50-cloud-init.conf`) suele fijar `PasswordAuthentication yes`; por eso el propio debe llamarse `10-…` y no `99-…`, o su configuración se ignora silenciosamente.

Aplicar **sin cerrar la sesión actual**:

```bash
sshd -t && systemctl reload ssh      # validar antes de recargar
sshd -T | grep -E '^(passwordauthentication|authenticationmethods|permitrootlogin|allowusers)'
```

### Variante opcional: criptografía restringida

[`recursos/ssh/crypto-optional.conf`](recursos/ssh/crypto-optional.conf) fija algoritmos modernos de intercambio, cifrado y MAC (viene del servidor de referencia). Probar primero con los clientes reales: un OpenSSH muy antiguo podría quedar sin acceso. **No se aplicó en el servidor nuevo.**

## 4. La cuenta del proveedor

Los proveedores con *cloud-init* crean un usuario (`debian`) con `sudo` sin contraseña. Aunque no tenga llaves, "solo `root` con privilegios" exige neutralizarlo:

```bash
gpasswd -d debian sudo
sed -i 's/^debian ALL=(ALL) NOPASSWD:ALL/# disabled: only root has privileges\n# debian ALL=(ALL) NOPASSWD:ALL/' /etc/sudoers.d/90-cloud-init-users
visudo -c
passwd -l debian
```

Y dejarlo fuera de `AllowUsers`.

## 5. Contraseñas bloqueadas y consola de rescate

```bash
for u in admin1 admin2 github debian; do passwd -l "$u"; done
passwd -S root      # "P" = tiene contraseña local; ver nota
```

`root` conserva su contraseña **solo** para la consola de rescate del proveedor: por SSH no se puede usar. Bloquearla (`passwd -l root`) elimina esa vía de recuperación; decidirlo con conocimiento de que el proveedor ofrece (o no) un modo de rescate alterno.

## 6. Privilegios temporales (cuando una persona necesita instalar algo)

Si alguien debe administrar durante un tiempo limitado, no se le agrega al grupo `sudo`: se concede una regla que **caduca sola** con un temporizador de systemd.

```bash
# Conceder (4 horas)
cat > /etc/sudoers.d/90-admin1-temporal <<'EOF'
admin1 ALL=(ALL:ALL) NOPASSWD: ALL
EOF
visudo -cf /etc/sudoers.d/90-admin1-temporal
systemd-run --unit=revoke-admin1-sudo --on-active=4h \
  /bin/sh -c 'rm -f /etc/sudoers.d/90-admin1-temporal'

# Revocar antes de tiempo
rm -f /etc/sudoers.d/90-admin1-temporal && systemctl stop revoke-admin1-sudo.timer
```

`NOPASSWD` es necesario porque las cuentas no tienen contraseña. Un `sudo -i` abierto antes de la revocación **sigue siendo root** hasta cerrarse.

### Auditoría obligatoria al terminar

Con privilegios totales alguien puede crear usuarios, añadir grupos o instalar software. Revisar siempre:

```bash
journalctl _COMM=sudo --no-pager -o short-iso | grep COMMAND     # qué ejecutó
grep -E '^(Start-Date|Commandline|Requested-By)' /var/log/apt/history.log | tail -30
getent group sudo docker adm                                      # membresías privilegiadas
awk -F: '$3>=1000 && $3<65000 {print $1}' /etc/passwd            # usuarios nuevos
cat /root/.ssh/authorized_keys                                    # llaves ajenas en root
ls /etc/sudoers.d; ls /etc/cron.d /var/spool/cron/crontabs
ss -tlnp                                                          # puertos nuevos
```

**Caso real que motiva esta lista:** durante una ventana de sudo temporal, un colaborador instaló Docker con un script, se agregó al grupo `docker` (equivale a `root`), creó un usuario adicional, también en `docker`, y construyó una imagen de prueba. Nada era malicioso, pero ninguno de esos cambios estaba pedido: se detectó y revirtió **solo** gracias a esta auditoría.

Deshacer: quitar los grupos (`gpasswd -d usuario docker`), borrar usuarios ajenos (`userdel -r`), purgar paquetes extra (`apt-get purge`), eliminar imágenes (`docker rmi`) y revisar que los archivos del colaborador no conserven grupos privilegiados (`find / -xdev -user admin1 \( -group docker -o -group sudo \)`; corregir con `chgrp -R admin1 /home/admin1`).

## 7. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `getent group sudo` | Vacío |
| `getent group docker` | Solo la cuenta de CI |
| `sudo -l -U admin1` | `not allowed to run sudo` |
| `ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password usuario@IP` | `Permission denied (publickey)` |
| Conexión nueva con llave de cada cuenta | Entra |
| `sshd -T \| grep authenticationmethods` | `publickey` |

Continuar con [03 · Firewall](03-firewall-ufw.md).
