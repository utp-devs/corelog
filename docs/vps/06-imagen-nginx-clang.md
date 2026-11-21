# 06 · Imagen Nginx compilada (`nginx-clang`)

← [05 Docker](05-docker.md) · [Índice](README.md) · Siguiente: [07 Configuración de Nginx](07-nginx-configuracion.md)

**Objetivo:** construir una imagen de Nginx **mínima, reproducible y verificable** que incluya los módulos de seguridad (ModSecurity, GeoIP2, headers-more, Brotli) compilados desde las fuentes originales con Clang/LLVM.

- Dockerfile vigente: [`recursos/nginx-image/Dockerfile`](recursos/nginx-image/Dockerfile). Todas las revisiones publicadas están archivadas en [`releases/`](recursos/nginx-image/releases/) y descritas en el [`CHANGELOG`](recursos/nginx-image/CHANGELOG.md).
- Etiqueta vigente: **`nginx-clang:1.31.9-r2`** (`<versión de nginx>-r<revisión>`; el esquema de versionado está en [16](16-mantenimiento-y-versionado-de-la-imagen.md)).
- Origen: la primera versión (`1.31.6-r1`) viene del proyecto `warlay` (`tools/nginx-image/`) y corría en el servidor de referencia. La revisión actual se **reconstruyó en el servidor nuevo** (Debian 13), con las dependencias actualizadas y un diseño de build más rápido.

## 1. Por qué compilarla

| Imagen oficial `nginx` | Esta imagen |
|---|---|
| Sin ModSecurity, GeoIP2 ni headers-more | Todos incluidos como módulos dinámicos |
| Basada en una distro completa (shell, apt, utilidades) | Sin shell, sin gestor de paquetes: lo que no está, no se explota |
| OpenSSL, PCRE y zlib de la distro | Versiones recientes, descargadas de sus proyectos, verificadas y enlazadas estáticamente |
| Compilada con GCC y banderas genéricas | Clang con LTO, `FORTIFY_SOURCE=3`, `RELRO`/`now`, protector de pila |
| Todos los módulos de nginx | Sin los que un servidor solo-proxy no usa (DAV, FLV, MP4, autoindex, SSI, FastCGI…) |
| ~190 MB | **~32 MB** |

El coste: hay que reconstruirla para cada parche de seguridad de Nginx, OpenSSL, zlib o sus dependencias. Ver [16](16-mantenimiento-y-versionado-de-la-imagen.md).

## 2. Arquitectura del build

El Dockerfile es un **grafo de etapas** que BuildKit ejecuta en paralelo cuando no dependen entre sí:

```
                        ┌─► openssl ─────────┐
                        ├─► zlib ────────────┤
                        ├─► pcre2 ───────────┼──────────────┐
 debian:13 ─► toolchain ├─► maxminddb ───────┤              ▼
 (LLVM, CMake,          ├─► yajl ────────────┼─► modsecurity ─► nginx ─► /out
  Ninja, pkgconf)       ├─► libxml2 ─────────┘                   ▲        │
                        ├─► brotli (ngx_brotli) ─────────────────┤        ▼
                        └─► srcs (nginx + módulos) ──────────────┘     runtime
 debian:13 ─► rootfs (glibc, libstdc++, status de dpkg) ──────────────► (scratch)
```

| Etapa | Contenido |
|---|---|
| `toolchain` | LLVM/Clang, CMake y Ninja descargados de sus proyectos (no de `apt`); `pkgconf` compilado. De `apt` solo vienen las cabeceras de glibc y las bibliotecas de desarrollo de GCC (`libgcc-14-dev`, `libstdc++-14-dev`): Clang enlaza contra ellas por defecto. **No quitarlas**: ver [16, sección 6](16-mantenimiento-y-versionado-de-la-imagen.md#6-limitación-conocida-del-toolchain-clang-sin-libgcc-dev) |
| `openssl`, `zlib`, `pcre2`, `maxminddb`, `yajl`, `libxml2` | Cada biblioteca en su propia etapa, instaladas solo en `/opt/nginx-deps` |
| `brotli` | `ngx_brotli` con Brotli recompilado desde la versión fijada |
| `srcs` | Fuentes de nginx y de los módulos de terceros, descargadas y verificadas |
| `modsecurity` | `libmodsecurity` como biblioteca compartida con sus dependencias estáticas |
| `nginx` | Reúne todo, compila nginx y sus módulos, **comprueba el enlace estático** |
| `rootfs` | Sistema de archivos mínimo con solo las bibliotecas que cargan los binarios |
| `runtime` | `FROM scratch`: copia `rootfs` + los binarios compilados |

Ventajas medidas (4 CPU, ver [16, sección 5](16-mantenimiento-y-versionado-de-la-imagen.md#5-compilar-más-rápido-qué-funcionó-y-qué-no)): subir **solo nginx** tarda **17 s** (antes ~80 s, porque OpenSSL se recompilaba dentro de nginx), subir OpenSSL 50 s, ModSecurity 82 s, y todas las bibliotecas juntas 114 s.

## 3. Versiones

El **Dockerfile es la única fuente de verdad**: todas las versiones son `ARG` en su cabecera. Esta tabla se comprueba contra él con [`check-docs.sh`](recursos/nginx-image/check-docs.sh) (falla si se desfasa) y se compara con las versiones publicadas con [`check-updates.sh`](recursos/nginx-image/check-updates.sh).

| Componente | Versión | Cómo se verifica |
|---|---|---|
| Nginx | 1.31.9 | Firma GPG (`.asc`) con las llaves de los mantenedores de nginx.org, y SHA-256 |
| LLVM/Clang | 23.1.5 | Firma GPG con las llaves de `releases.llvm.org` (archivo `.tar.zst`) |
| CMake | 4.5.1 | SHA-256 publicado por Kitware |
| Ninja | 1.13.2 | SHA-256 fijado |
| pkgconf | 3.0.11 | SHA-256 fijado |
| OpenSSL | 3.5.9 | SHA-256 publicado en la release (rama LTS 3.5) |
| PCRE2 | 10.49 | SHA-256 fijado |
| zlib | 1.4 | SHA-256 fijado |
| libmaxminddb | 1.14.1 | SHA-256 fijado |
| Brotli | 1.2.1 | SHA-256 fijado |
| yajl | 2.1.0 | SHA-256 fijado |
| libxml2 | 2.15.4 | SHA-256 publicado por GNOME |
| ModSecurity (libmodsecurity) | 3.0.17 | SHA-256 de la release |
| ModSecurity-nginx | 1.0.4 | SHA-256 fijado |
| headers-more-nginx-module | 0.42 | SHA-256 fijado |
| ngx_http_geoip2_module | 3.4 | SHA-256 fijado |
| ngx_brotli | 53e0fb5 (commit) | SHA-256 fijado |

**Dos niveles de verificación.** Firma GPG o suma publicada por el propio proyecto (autenticidad) donde existe; SHA-256 fijado en el Dockerfile para el resto (inmutabilidad: si el archivo cambia upstream, el build falla). Un hash fijado **no** prueba que el archivo fuera legítimo al fijarlo; prueba que no cambió después.

Las etiquetas OCI y `nginx-clang.*` de la imagen llevan la versión de cada componente principal:

```bash
docker image inspect nginx-clang:1.31.9-r2 --format '{{range $k,$v := .Config.Labels}}{{$k}}={{$v}}{{println}}{{end}}'
```

## 4. Banderas de compilación

```
--with-cc=clang
--with-cc-opt="-O2 -flto=thin -fstack-protector-strong -D_FORTIFY_SOURCE=3 -fPIC -Wformat -Werror=format-security -I$DEPS/include"
--with-ld-opt="-fuse-ld=lld -flto=thin -Wl,-z,relro -Wl,-z,now -fPIC -L$DEPS/lib -Wl,-rpath,/opt/modsecurity/lib"
--with-pcre-jit
```

| Bandera | Para qué |
|---|---|
| `-flto=thin` | Optimización entre módulos con menos tiempo de enlace |
| `-fstack-protector-strong` | Detecta desbordamientos de pila |
| `-D_FORTIFY_SOURCE=3` | Comprobaciones en tiempo de ejecución de funciones de memoria/cadenas |
| `-Wformat -Werror=format-security` | Convierte errores de formato en fallo de compilación |
| `-Wl,-z,relro -Wl,-z,now` | Tabla GOT de solo lectura tras el arranque (anti-sobrescritura) |
| `-fPIC` | Necesario para los módulos dinámicos |
| `-I/-L $DEPS` | Nginx encuentra OpenSSL, PCRE2 y zlib **ya compilados** (estáticos) en `/opt/nginx-deps` |
| Sin `-march=native` | La imagen corre en cualquier CPU x86-64 |

OpenSSL se compila con `no-shared no-threads no-tests no-docs no-ssl3 no-weak-ssl-ciphers` (las mismas opciones que usaría nginx con su OpenSSL integrado), PCRE2 con JIT, y los tres quedan **enlazados estáticamente**. El build **falla** si el binario necesitara alguna biblioteca compartida distinta de glibc:

```
Shared library: [libc.so.6]
Shared library: [libcrypt.so.1]
```

### Módulos de Nginx

Integrados: `http_ssl`, `http_v2`, `http_realip`, `http_stub_status`, `http_auth_request` (necesario para Anubis), `gzip_static`, `gunzip`, `sub`, `slice`, `secure_link`, `stream_ssl`, `stream_ssl_preread`, `stream_realip`, con `--with-threads --with-file-aio --with-pcre-jit`.

**No compilados**, a propósito: `http_dav`, `http_flv`, `http_mp4`, `http_random_index`, `http_addition`, `autoindex`, `ssi`, `fastcgi`, `scgi` y `uwsgi`. Un servidor solo-proxy no los usa y cada uno es superficie de ataque. **Si una aplicación los necesita, hay que añadirlos al `configure` y subir la revisión.**

Dinámicos (en `/usr/lib/nginx/modules/`):

| Archivo `.so` | Función |
|---|---|
| `ngx_http_modsecurity_module.so` | WAF (ModSecurity v3) |
| `ngx_http_geoip2_module.so` | País por IP (base GeoLite2) |
| `ngx_stream_geoip2_module.so` | Lo mismo para tráfico TCP/UDP |
| `ngx_http_headers_more_filter_module.so` | Borrar/escribir cabeceras (p. ej. ocultar `Server`) |
| `ngx_http_brotli_filter_module.so`, `ngx_http_brotli_static_module.so` | Compresión Brotli |
| `ngx_stream_module.so` | Proxy TCP/UDP |

Rutas fijadas en el binario: `--conf-path=/etc/nginx/nginx.conf`, `--http-log-path=/var/log/nginx/access.log`, `--error-log-path=stderr`, `--pid-path=/run/nginx.pid`, `--modules-path=/usr/lib/nginx/modules`, temporales en `/var/lib/nginx/*`.

`--error-log-path=stderr` implica que, **sin** una directiva `error_log` explícita, los errores salen por consola. En [07](07-nginx-configuracion.md) se escriben a archivo **y** a consola.

## 5. libmodsecurity

Se compila como biblioteca compartida con sus dependencias enlazadas de forma estática (PCRE2, yajl, libxml2), de modo que el entorno de ejecución solo necesita `libstdc++`:

```
./configure --prefix=/opt/modsecurity --with-pcre2 --with-yajl --with-libxml \
  --without-lua --without-curl --without-geoip --without-maxmind --without-lmdb --without-ssdeep
```

- Quedan fuera Lua, curl, ssdeep y LMDB: funciones que las reglas CRS no necesitan y que son superficie extra.
- `-Wl,-rpath,/opt/modsecurity/lib` en Nginx hace que encuentre la biblioteca sin tocar `LD_LIBRARY_PATH`.
- `unicode.mapping` se copia a `/opt/modsecurity/`.

## 6. Sistema de archivos mínimo (etapa `rootfs`)

La imagen final es `FROM scratch`. Se construye un `rootfs` con:

- Las bibliotecas que realmente cargan los binarios: `libc6`, `libcrypt1`, `libgcc-s1`, `libstdc++6`, `gcc-14-base`, copiadas de paquetes Debian 13 **totalmente actualizados** (`apt-get upgrade` previo).
- Un registro por paquete en `/var/lib/dpkg/status.d/` (como hacen las imágenes *distroless*): los escáneres de vulnerabilidades (Trivy, Grype, Scout) siguen viendo y reportando esas bibliotecas aunque no haya `apt`.
- `passwd`/`group` mínimos (`root`, `www-data`, `nobody`) con shell `/sbin/nologin`.
- Directorios de trabajo `/var/lib/nginx/*` con dueño `www-data` (uid 33).
- **Sin** shell, gestor de paquetes, OpenSSL del sistema ni `tzdata`: la hora se hereda montando `/etc/localtime` del host.
- **Sin módulos `gconv` de glibc**, a propósito: `libxml2` maneja UTF-8/16, Latin-1 y ASCII de forma nativa, y quitar `iconv` elimina rutas de denegación de servicio alcanzables con codificaciones exóticas declaradas en cuerpos XML.

Consecuencias operativas:

| Quieres… | Haz esto |
|---|---|
| Entrar al contenedor | No hay shell. Usar `nginx -t`, `nginx -s reload`, `nginx -V` (envoltorio del host, [10](10-nginx-como-servicio-del-host.md)) o `docker exec nginx nginx …` |
| Ver módulos y versión | `nginx -V` |
| Validar una configuración nueva | `nginx-container test` (con la imagen del servicio) o `NGINX_IMAGE=nginx-clang:NUEVA nginx-container test` |
| Usar CRS, la base GeoIP, `mime.types`, certificados | Montarlos desde el host (no vienen en la imagen) |
| Workers sin privilegios | `user www-data;` en `nginx.conf` (uid 33, igual al de la imagen y al del host Debian) |

## 7. Construcción

Requisitos del equipo de build: Docker con BuildKit, acceso a Internet y varios GB libres. La primera vez descarga LLVM (1,2 GB); después queda en una caché de BuildKit.

```bash
cd docs/vps/recursos/nginx-image
./build.sh                                  # construye, etiqueta, verifica, prueba y archiva
./build.sh --build-arg OPENSSL_VERSION=3.6.5 --build-arg IMAGE_REVISION=r2-test    # variante de prueba
```

`build.sh` deduce la etiqueta del propio Dockerfile (`nginx-clang:<NGINX_VERSION>-<IMAGE_REVISION>`), guarda el registro en `build-<tag>.log`, **verifica** la imagen, ejecuta la **prueba funcional** y archiva el Dockerfile en `releases/`.

Equivalente manual: `docker build -t nginx-clang:1.31.9-r2 .`

### Verificar la imagen

```bash
docker run --rm --entrypoint /usr/sbin/nginx nginx-clang:1.31.9-r2 -V
#  nginx version: nginx/1.31.9
#  built by clang 23.1.5 (...)
#  built with OpenSSL 3.5.9 ...

c=$(docker create nginx-clang:1.31.9-r2)
docker export "$c" | tar -t | grep -E 'usr/lib/nginx/modules/|opt/modsecurity/lib|usr/sbin/nginx'
docker export "$c" | tar -t | grep -cE '^(bin|usr/bin)/(sh|bash)$'     # 0: sin shell
docker rm "$c"
```

Resultado verificado: siete módulos `.so`, `libmodsecurity.so.3.0.17`, ningún `sh`, 31,7 MB.

### Prueba funcional (`smoke-test.sh`)

[`recursos/nginx-image/smoke-test.sh`](recursos/nginx-image/smoke-test.sh) arranca un nginx **de prueba** en puertos altos de loopback (18998 y 18999), con su propia configuración mínima, de modo que puede correr junto al servicio de producción sin tocarlo:

```bash
./smoke-test.sh nginx-clang:1.31.9-r2 /etc/nginx/geoip/GeoLite2-Country.mmdb
```

| Comprobación | Qué ejercita |
|---|---|
| Arranque y respuesta HTTP | Configuración mínima y módulos cargados |
| `Server` reescrito | headers-more |
| Respuesta y cuerpo `gzip` | zlib (compresión) |
| Respuesta `br` | Brotli |
| `.gz` guardado en disco servido descomprimido | `gunzip` + `gzip_static` (zlib, descompresión) |
| Petición con regla propia → 403 / petición normal → 200 | ModSecurity |
| TLS 1.3 + HTTP/2, TLS 1.2 aceptado, TLS 1.0 rechazado | OpenSSL |
| Workers como `www-data` | Privilegios |
| Cabecera de país presente | Base GeoIP2 |
| Sin `emerg`/`alert`/`crit` en el log; sin shell | Estabilidad y superficie |

Resultado de la revisión vigente: **15 de 15**.

## 8. Mover la imagen entre servidores (sin recompilar)

La imagen es autocontenida (trae su propia glibc), así que **no depende del sistema operativo del host**: la compilada sobre Debian 12 corrió en Debian 13, y viceversa.

```bash
ssh origen 'docker save nginx-clang:1.31.9-r2' | ssh destino 'docker load'
```

También con un archivo (`docker save -o nginx.tar` / `docker load -i nginx.tar`) o un registro privado (GHCR). Tras cargarla, **verificar por contenido, no por ID**: el ID puede diferir según el almacén de imágenes de cada Docker; lo fiable es `nginx -V`, las etiquetas y el tamaño.

## 9. Mantener la imagen actualizada

Procedimiento completo, política de versiones y lista de comprobación en **[16 · Mantenimiento y versionado de la imagen](16-mantenimiento-y-versionado-de-la-imagen.md)**. Resumen:

1. `./check-updates.sh` compara cada componente con upstream.
2. Cambiar los `ARG` (y su SHA-256), subir `IMAGE_REVISION`.
3. `./build.sh` construye, prueba y archiva.
4. Actualizar `CHANGELOG.md` y la tabla de la sección 3; `./check-docs.sh` confirma que coinciden.
5. Desplegar con `NGINX_IMAGE=… nginx-container test` y `systemctl restart nginx` ([10](10-nginx-como-servicio-del-host.md)).

## 10. Imagen alternativa de la misma familia (`nginx-template`)

El proyecto de origen tiene otra imagen más antigua, `tools/nginx-template/`, **que no es la de este documento**:

| | `nginx-clang` (esta guía) | `nginx-template` |
|---|---|---|
| Base | `scratch` (rootfs Debian mínimo) | Alpine 3.20 |
| Nginx / OpenSSL / ModSecurity | 1.31.9 / 3.5.9 / 3.0.17 | 1.30.3 / 3.6.1 / 3.0.14 |
| Compilador | Clang + LTO | GCC, `-march=native` |
| Reglas CRS | Del host (montadas) | Dentro de la imagen (CRS 4.23 *minimal*) |
| Configuración | Montada desde el host | Plantillas + `envsubst` en el arranque (variables `MODSEC_*`, `GEOIP_*`) |
| Extras | — | Módulo OpenTelemetry, `tini`, `HEALTHCHECK` |
| Tamaño | Mucho menor | Mayor (decenas de paquetes `apk`) |
| Uso recomendado | Servidor con varios sitios, configuración y reglas gestionadas en el host | Contenedor autocontenido y parametrizado por variables de entorno |

Este repositorio usa `nginx-clang` por tamaño, superficie de ataque y porque las reglas y la configuración se actualizan sin reconstruir la imagen.

## 11. Limitaciones conocidas

- Sin shell: la depuración se hace con `nginx -t`/`-V`, logs y montajes.
- Sin `HEALTHCHECK` (no hay `wget`/`curl`); se supervisa desde fuera ([14](14-backups-monitoreo-y-alertas.md)).
- Para los componentes sin firma publicada, el SHA-256 fijado detecta cambios posteriores pero no prueba la autenticidad original (sección 3).
- LLVM 23.1.6 y posteriores no publican la firma GPG que este build verifica: se usa la más reciente que sí la tiene (ver [16](16-mantenimiento-y-versionado-de-la-imagen.md)).
- Reconstruir es responsabilidad propia: no hay actualización automática; planificar revisiones periódicas.
- Los módulos no compilados (sección 4) hay que añadirlos a mano si una aplicación los necesita.

Continuar con [07 · Configuración de Nginx](07-nginx-configuracion.md).
