# 12 · DNS, Cloudflare y certificados TLS

← [11 fail2ban](11-fail2ban.md) · [Índice](README.md) · Siguiente: [13 Usuario de despliegue y CI/CD](13-usuario-de-despliegue-y-cicd.md)

**Objetivo:** apuntar el dominio y todos sus subdominios al VPS con registros *wildcard*, y obtener un **único certificado wildcard** de Let's Encrypt con renovación automática, para poder crear subdominios nuevos sin emitir nada más.

## 1. Cloudflare CLI (`cf`)

CLI oficial de Cloudflare (en beta al escribir esto) que cubre DNS, zonas y tokens.

```bash
npm install --global cf@latest
cf --version
cf auth login          # abre el navegador: la autenticación la hace la persona, no un script
cf zones list          # confirma que aparece el dominio
```

El descubrimiento de comandos se hace describiendo la tarea (`cf cli search "create a DNS record"`); no incluir dominios ni identificadores en esas búsquedas.

## 2. Registros DNS

Para `app.example.com` y `api.app.example.com` (dos niveles) hacen falta **dos** comodines: un `*` solo cubre **un** nivel.

| Registro | Tipo | Contenido | Cubre |
|---|---|---|---|
| `example.com` | A | `<IP_DEL_VPS>` | El dominio raíz |
| `*.example.com` | A | `<IP_DEL_VPS>` | `app.example.com`, `otra.example.com`… |
| `*.app.example.com` | A | `<IP_DEL_VPS>` | `api.app.example.com`, `x.app.example.com`… |

```bash
for n in "example.com" "*.example.com" "*.app.example.com"; do
  cf dns records create --zone example.com --body \
    "{\"type\":\"A\",\"name\":\"$n\",\"content\":\"<IP_DEL_VPS>\",\"ttl\":300,\"proxied\":false,\"comment\":\"VPS\"}"
done
cf dns records list --zone example.com
```

Si hay IPv6 global, añadir registros `AAAA` equivalentes **solo** si Nginx y el firewall están probados con IPv6 (Nginx ya escucha en `[::]`).

### ¿Proxy de Cloudflare (nube naranja) o solo DNS (gris)?

| | Solo DNS (`proxied: false`) — usado | Con proxy |
|---|---|---|
| IP del servidor | Pública | Oculta tras Cloudflare |
| Certificado | El del servidor (Let's Encrypt) | Cloudflare termina TLS; el servidor puede seguir con su certificado |
| IP real del visitante | `$remote_addr` directo | Hay que configurar `real_ip` con los rangos de Cloudflare, o GeoIP y fail2ban verán IPs de Cloudflare |
| Subdominios de 2.º nivel | Cubiertos por el certificado propio | El certificado universal gratuito de Cloudflare no cubre `*.app.example.com` |

Se partió de solo DNS por simplicidad. Pasar a proxy es una decisión a tomar con `real_ip` y un ufw que acepte únicamente los rangos de Cloudflare.

## 3. Token de API con permisos mínimos

Certbot necesita crear y borrar registros `TXT` (`_acme-challenge`). **No** usar la clave global: crear un token limitado a **una zona** con dos permisos.

```bash
cf user tokens permission-groups list | grep -B1 -A1 -E '"name": "(DNS Write|Zone Read)"'   # obtener los IDs

ZID=<ID_DE_LA_ZONA>      # cf zones list --name example.com
cf user tokens create --body "{
  \"name\": \"certbot-vps\",
  \"policies\": [{
    \"effect\": \"allow\",
    \"resources\": {\"com.cloudflare.api.account.zone.$ZID\": \"*\"},
    \"permission_groups\": [{\"id\": \"<ID_DNS_WRITE>\"}, {\"id\": \"<ID_ZONE_READ>\"}]
  }]
}"
```

La respuesta contiene el token **una sola vez**. Enviarlo directamente al servidor sin dejarlo en el historial ni mostrarlo:

```bash
install -d -m 700 /root/.secrets
umask 077
cat > /root/.secrets/cloudflare.ini <<'EOF'
dns_cloudflare_api_token = <TOKEN_DE_CLOUDFLARE>
EOF
chmod 600 /root/.secrets/cloudflare.ini
```

Plantilla: [`recursos/certbot/cloudflare.ini.example`](recursos/certbot/cloudflare.ini.example). El token no caduca por defecto: anotar una fecha de rotación y, si se puede, fijar `expires_on` al crearlo. Si se compromete, se revoca desde el panel o con `cf` sin afectar a nada más.

## 4. Certificado wildcard

```bash
apt-get install -y certbot python3-certbot-dns-cloudflare

certbot certonly --dns-cloudflare \
  --dns-cloudflare-credentials /root/.secrets/cloudflare.ini \
  --dns-cloudflare-propagation-seconds 30 \
  --non-interactive --agree-tos --register-unsafely-without-email \
  --cert-name example.com \
  -d example.com -d "*.example.com" -d "*.app.example.com"

certbot certificates
```

Resultado: un único certificado `example.com` con los tres nombres, en `/etc/letsencrypt/live/example.com/`. Todos los vhosts apuntan a él ([07](07-nginx-configuracion.md)):

```nginx
ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
```

Un **subdominio nuevo** del mismo nivel no necesita DNS ni certificado: basta un `server` en Nginx. Un tercer nivel distinto (`x.otro.example.com`) exige añadir `*.otro.example.com` al certificado (`certbot certonly … --expand` con la lista completa).

> **Sin correo.** `--register-unsafely-without-email` evita pedirlo, pero entonces Let's Encrypt **no avisa** si la renovación falla. Compensarlo con el monitoreo de caducidad ([14](14-backups-monitoreo-y-alertas.md): alerta a menos de 14 días).

## 5. Renovación y recarga de Nginx

`certbot` instala un temporizador de systemd (`certbot.timer`) que renueva a los 60 días. Hace falta que Nginx **recargue** el certificado nuevo:

```bash
install -d /etc/letsencrypt/renewal-hooks/deploy
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh <<'EOF'
#!/bin/sh
systemctl reload nginx
EOF
chmod 755 /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
```

Plantilla: [`recursos/certbot/reload-nginx.sh`](recursos/certbot/reload-nginx.sh).

### Probar la renovación

```bash
certbot renew --dry-run --no-random-sleep-on-renew --cert-name example.com
# Congratulations, all simulated renewals succeeded
```

> **`--no-random-sleep-on-renew`.** En modo no interactivo certbot espera un tiempo **aleatorio de hasta 8 minutos** antes de renovar. Sin esta opción la simulación parece colgada (y un `timeout` la mata con código 124). No es un fallo.

## 6. Parámetros TLS de Nginx

[`snippets/ssl-params.conf`](recursos/nginx/snippets/ssl-params.conf): TLS 1.2 y 1.3; suites ECDHE con AES-GCM y ChaCha20; HSTS un año con `includeSubDomains`; sin *session tickets*. No se configura `ssl_stapling`: Let's Encrypt dejó de publicar OCSP, y Nginx solo registraría `"ssl_stapling" ignored, no OCSP responder URL`.

Comprobar desde fuera:

```bash
echo | openssl s_client -connect api.app.example.com:443 -servername api.app.example.com 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates -ext subjectAltName
# issuer Let's Encrypt, SAN: example.com, *.example.com, *.app.example.com
```

Un host sin vhost debe ser rechazado en el handshake ([07](07-nginx-configuracion.md)):

```bash
echo | openssl s_client -connect <IP_DEL_VPS>:443 -servername desconocido.example.com 2>&1 | grep -i alert
# tlsv1 unrecognized name
```

## 7. Problemas frecuentes

| Síntoma | Causa | Solución |
|---|---|---|
| `curl` desde la red de la oficina muestra "Fortiguard … Blocked Page" o `SEC_E_UNTRUSTED_ROOT` | El filtro web de la red clasifica los **dominios nuevos** como no categorizados y los intercepta | Probar desde otra red/servidor; pedir al administrador de red que recategorice el dominio |
| `DNS_PROBE` / no resuelve | Propagación o registro mal escrito | `nslookup nombre 1.1.1.1`; revisar `cf dns records list` |
| Certbot falla con `Unable to determine zone_id` | Token sin `Zone Read` o de otra zona | Recrear el token con los dos permisos |
| El certificado no cubre `api.app.example.com` | Falta `*.app.example.com` | Añadirlo con `--expand` |
| El sitio muestra el certificado viejo tras renovar | Falta el gancho de recarga | Crear `reload-nginx.sh` |

## 8. Verificación

| Comprobación | Resultado esperado |
|---|---|
| `cf dns records list --zone example.com` | Los 3 registros |
| `certbot certificates` | Un certificado, 3 nombres, ~89 días de validez al emitirse |
| `certbot renew --dry-run --no-random-sleep-on-renew` | Éxito |
| `openssl s_client` a un subdominio | Cadena válida de Let's Encrypt |
| SNI desconocido | `unrecognized name` |

Continuar con [13 · Usuario de despliegue y CI/CD](13-usuario-de-despliegue-y-cicd.md).
