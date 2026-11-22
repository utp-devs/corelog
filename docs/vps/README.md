# Guía de configuración y blindaje de un VPS

Hoja de ruta completa para llevar un VPS Debian desde "recién creado" hasta un servidor **blindado** que publica aplicaciones en contenedores detrás de un Nginx compilado a medida, con WAF, desafío anti-bots, firewall, fail2ban, certificados wildcard, despliegue continuo, backups y monitoreo.

Todo lo documentado aquí se aplicó y se verificó en un servidor real (Debian 13 "trixie") para publicar Corelog. Cuando algo proviene de otro servidor de referencia, o no pudo verificarse en el servidor nuevo, el documento lo indica expresamente.

> **Convenciones.** `<IP_DEL_VPS>` y `<TU_IP_PUBLICA>` son valores a reemplazar; `example.com`, `app.example.com` y `api.example.com` representan el dominio real. Las rutas de `recursos/` son los archivos de configuración listos para copiar. Ningún archivo de esta carpeta contiene secretos.

## Arquitectura objetivo

```
                        Internet
                           │  80 / 443            22 (solo llaves SSH)
                           ▼                          ▼
        ┌──────────────── ufw (deny incoming) ───────────────┐
        │                                                    │
        │   Nginx 1.31 (contenedor, red host, imagen propia) │
        │   ├─ GeoIP (solo país permitido)                   │
        │   ├─ Bloqueo de rutas/escáneres  → attacks.log ──┐ │
        │   ├─ ModSecurity + OWASP CRS (WAF)               │ │
        │   ├─ Anubis (auth_request, solo frontend)        │ │
        │   └─ TLS wildcard (Let's Encrypt, DNS-01)        │ │
        │            │                                     ▼ │
        │            ▼ 127.0.0.1:puerto        fail2ban ──► ufw (baneos)
        │   Contenedores de la aplicación (ui, api, db)      │
        └────────────────────────────────────────────────────┘
        Sistema: usuarios sin sudo · SSH solo llaves · sysctl · actualizaciones automáticas
        Operación: logrotate · backups diarios · monitoreo cada 5 min · CI/CD por SSH
```

## Hoja de ruta

Seguir el orden numérico: cada documento asume los anteriores y termina con una **verificación** que debe pasar antes de continuar.

| # | Documento | Qué logras |
|---|---|---|
| 01 | [Requisitos y primeros pasos](01-requisitos-y-primeros-pasos.md) | Sistema actualizado, hora correcta, paquetes base |
| 02 | [Usuarios, permisos y SSH](02-usuarios-permisos-y-ssh.md) | Solo `root` con privilegios, acceso únicamente por llaves |
| 03 | [Firewall (ufw)](03-firewall-ufw.md) | Todo bloqueado salvo 22/80/443; trampa de Docker y ufw |
| 04 | [Endurecimiento del sistema](04-endurecimiento-del-sistema.md) | sysctl, actualizaciones automáticas, swap, rotación de logs |
| 05 | [Docker desde el repositorio oficial](05-docker.md) | Docker CE fijado al repo oficial, logs acotados |
| 06 | [Imagen Nginx compilada (`nginx-clang`)](06-imagen-nginx-clang.md) | Imagen mínima de ~32 MB: Clang, OpenSSL, ModSecurity, GeoIP2, Brotli; versionada `1.31.9-r2` |
| 07 | [Configuración de Nginx](07-nginx-configuracion.md) | Estructura, vhosts, cabeceras, límites, logs en consola y archivos |
| 08 | [WAF: ModSecurity + OWASP CRS](08-waf-modsecurity-crs.md) | WAF activo, ajuste de falsos positivos |
| 09 | [Anubis (desafío anti-bots)](09-anubis.md) | Protección del frontend contra bots y scrapers |
| 10 | [Nginx como servicio por defecto del host](10-nginx-como-servicio-del-host.md) | `systemctl`/`nginx -t` del host operan el contenedor |
| 11 | [fail2ban y bloqueo permanente](11-fail2ban.md) | Baneo automático, permanente para ataques |
| 12 | [DNS, Cloudflare y certificados TLS](12-dns-y-certificados-tls.md) | Wildcard `*.example.com` con renovación automática |
| 13 | [Usuario de despliegue y CI/CD](13-usuario-de-despliegue-y-cicd.md) | GitHub Actions despliega sin dar acceso de más |
| 14 | [Backups, monitoreo y alertas](14-backups-monitoreo-y-alertas.md) | Copias diarias, alertas por cambio de estado |
| 15 | [Verificación, auditoría y lecciones aprendidas](15-verificacion-y-auditoria.md) | Lista de comprobación final, mantenimiento, problemas conocidos |
| 16 | [Mantenimiento y versionado de la imagen](16-mantenimiento-y-versionado-de-la-imagen.md) | Dockerfile versionado, dependencias al día, compilación rápida, limitaciones del toolchain |

Archivos de configuración: [`recursos/`](recursos/README.md) (con el destino de cada archivo en el servidor).

## Resumen de la configuración final

| Capa | Medida |
|---|---|
| Acceso | SSH solo con llave (`AuthenticationMethods publickey`), root sin contraseña, `AllowUsers`, 3 intentos |
| Privilegios | Solo `root`; ningún usuario con `sudo`; usuario de CI limitado a Docker |
| Red | ufw `deny incoming`; puertos de contenedores solo en `127.0.0.1` |
| Aplicación | WAF (845 reglas CRS), GeoIP, bloqueo de rutas privadas, límite de tasa de login, Anubis; workers de Nginx sin privilegios (`www-data`) |
| Cadena de suministro | Imagen compilada desde fuentes con cada descarga fijada por SHA-256 (y firma GPG donde existe); dependencias auditadas contra upstream con un script; documentación comprobada contra el Dockerfile |
| Reacción | fail2ban: 7 jails, baneo permanente para ataques y reincidentes |
| Transporte | TLS 1.2/1.3, HSTS, certificado wildcard, SNI desconocido rechazado |
| Datos | Backup diario (config + `pg_dump`), 14 días de retención |
| Operación | Monitoreo cada 5 min con alertas por cambio de estado, logrotate, actualizaciones de seguridad automáticas |

## Cómo se verificó

- **Servidor nuevo** (Debian 13): todo lo marcado como "verificado" se probó con tráfico real y simulaciones de ataque desde otro servidor dentro del país permitido.
- **Servidor de referencia** (Debian 13, producción previa, con 19 nombres de sitio): origen de la primera versión de la imagen `nginx-clang`, de la unidad `systemd` que convierte al contenedor en el Nginx del host y de algunas variantes opcionales (`userns-remap`, SSH por WireGuard). Cada vez que se usa, se indica.
- La imagen vigente (`1.31.9-r2`) se **reconstruyó en el servidor nuevo**, y su integración con el sistema (modo servicio del host) está aplicada y verificada allí. Después se **desplegó también en el servidor de referencia**, reemplazando a la `1.31.6-r1`.
