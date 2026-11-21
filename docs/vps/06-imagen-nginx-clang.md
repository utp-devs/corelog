# 06 · Imagen Nginx compilada (`nginx-clang`)

← [05 Docker](05-docker.md) · [Índice](README.md) · Siguiente: [07 Configuración de Nginx](07-nginx-configuracion.md)

**Objetivo:** construir una imagen de Nginx **mínima, reproducible y verificable** que incluya los módulos de seguridad (ModSecurity, GeoIP2, headers-more, Brotli) compilados desde las fuentes originales con Clang/LLVM.

- Archivo fuente: [`recursos/nginx-image/Dockerfile`](recursos/nginx-image/Dockerfile) (origen: `tools/nginx-image/` de otro proyecto del mismo autor).
- Etiqueta resultante: `nginx-clang:1.31.6`.
- Se construyó en un equipo de desarrollo (Docker Desktop) y se usó en dos servidores: el de referencia (Debian 12) y el nuevo (Debian 13). **Se transfirió, no se recompiló** (sección 8).

## 1. Por qué compilarla

| Imagen oficial `nginx` | Esta imagen |
|---|---|
| Sin ModSecurity, GeoIP2 ni headers-more | Todos incluidos como módulos dinámicos |
| Basada en una distro completa (shell, apt, utilidades) | Sin shell, sin gestor de paquetes: lo que no está, no se explota |
| OpenSSL, PCRE y zlib de la distro | Versiones recientes, descargadas de sus proyectos y verificadas |
| Compilada con GCC y banderas genéricas | Clang con LTO, `FORTIFY_SOURCE=3`, `RELRO`/`now`, protector de pila |
| ~190 MB | ~23 MB comprimida (~46 MB en disco) |

El coste: hay que reconstruirla para cada parche de seguridad de Nginx, OpenSSL o sus dependencias. Ver la sección 9.

## 2. Arquitectura del build (5 etapas)

```
debian:12 ─► toolchain ─► deps ─► nginx ─┐
   │         (LLVM,CMake,   (OpenSSL,     (nginx + 4 módulos,   │
   │          pkgconf)       PCRE2, zlib,  libmodsecurity        │
   │                         maxminddb,    copiada)              │
   │                         yajl, libxml2,                      ▼
   │                         brotli,                        FROM scratch
   └────────► rootfs ───────────────────────────────────►  (runtime)
             (glibc, libstdc++ de paquetes Debian parcheados,
              status de dpkg, passwd/group mínimos)
```

| Etapa | Contenido |
|---|---|
| `toolchain` | LLVM/Clang y CMake descargados de sus proyectos (no de `apt`); `pkgconf` compilado. De `apt` solo vienen las cabeceras de glibc y libstdc++, porque definen la ABI del sistema y no se pueden sustituir |
| `deps` | OpenSSL, PCRE2, zlib, libmaxminddb, yajl, libxml2, brotli y **libmodsecurity** compilados |
| `nginx` | Nginx + `ModSecurity-nginx`, `headers-more`, `ngx_brotli` y `ngx_http_geoip2` como módulos dinámicos |
| `rootfs` | Sistema de archivos mínimo con solo las bibliotecas que cargan los binarios |
| `runtime` | `FROM scratch`: copia `rootfs` + los binarios compilados |

## 3. Versiones

| Componente | Versión | Cómo se verifica |
|---|---|---|
| Nginx | 1.31.6 | Firma GPG (`.asc`) con las llaves de los mantenedores de nginx.org |
| LLVM/Clang | 23.1.3 | Firma GPG con las llaves de `releases.llvm.org` |
| CMake | 4.4.4 | SHA-256 publicado por Kitware |
| pkgconf | 3.0.8 | — |
| OpenSSL | 3.5.9 | SHA-256 publicado en la release |
| PCRE2 | 10.49 | — |
| zlib | 1.3.2 | — |
| libmaxminddb | 1.14.1 | — |
| Brotli | 1.2.0 | — |
| yajl | 2.1.0 | — |
| libxml2 | 2.15.4 | SHA-256 publicado por GNOME |
| ModSecurity (libmodsecurity) | 3.0.17 | SHA-256 de la release |
| ModSecurity-nginx | 1.0.4 | — |
| headers-more-nginx-module | 0.40 | — |
| ngx_http_geoip2_module | 3.4 | — |
| ngx_brotli | commit `a71f931` | — |

Todas son `ARG` del Dockerfile: para actualizar un componente se cambia el valor (ver sección 9). Los componentes marcados con "—" no publican firma o suma verificable de la forma que usa este build; son un punto a mejorar fijando un hash conocido.

## 4. Banderas de compilación

```
--with-cc=clang
--with-cc-opt="-O2 -flto=thin -fstack-protector-strong -D_FORTIFY_SOURCE=3 -fPIC -Wformat -Werror=format-security -I$DEPS/include"
--with-ld-opt="-fuse-ld=lld -flto=thin -Wl,-z,relro -Wl,-z,now -fPIC -L$DEPS/lib -Wl,-rpath,/opt/modsecurity/lib"
--with-openssl-opt="no-tests no-docs no-ssl3 no-weak-ssl-ciphers"
```

| Bandera | Para qué |
|---|---|
| `-flto=thin` | Optimización entre módulos con menos tiempo de enlace |
| `-fstack-protector-strong` | Detecta desbordamientos de pila |
| `-D_FORTIFY_SOURCE=3` | Comprobaciones en tiempo de ejecución de funciones de memoria/cadenas |
| `-Wformat -Werror=format-security` | Convierte errores de formato en fallo de compilación |
| `-Wl,-z,relro -Wl,-z,now` | Tabla GOT de solo lectura tras el arranque (anti-sobrescritura) |
| `-fPIC` | Necesario para los módulos dinámicos |
| `no-ssl3 no-weak-ssl-ciphers` | Quita protocolos y cifrados débiles desde la compilación |
| Sin `-march=native` | La imagen corre en cualquier CPU x86-64 |

OpenSSL, PCRE2 (con JIT) y zlib se enlazan **estáticamente** dentro de Nginx: no dependen de las bibliotecas del sistema.

### Módulos de Nginx

Integrados: `http_ssl`, `http_v2`, `http_realip`, `http_stub_status`, `http_auth_request` (necesario para Anubis), `gzip_static`, `gunzip`, `sub`, `slice`, `secure_link`, `dav`, `addition`, `flv`, `mp4`, `random_index`, `stream_ssl`, `stream_ssl_preread`, `stream_realip`, con `--with-threads --with-file-aio --with-pcre-jit`.

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

- Las bibliotecas que realmente cargan los binarios: `libc6`, `libcrypt1`, `libgcc-s1`, `libstdc++6`, `gcc-12-base`, copiadas de paquetes Debian 12 **totalmente actualizados** (`apt-get upgrade` previo).
- Un registro por paquete en `/var/lib/dpkg/status.d/` (como hacen las imágenes *distroless*): los escáneres de vulnerabilidades (Trivy, Grype, Scout) siguen viendo y reportando esas bibliotecas aunque no haya `apt`.
- `passwd`/`group` mínimos (`root`, `www-data`, `nobody`) con shell `/sbin/nologin`.
- Directorios de trabajo `/var/lib/nginx/*` con dueño `www-data` (uid 33).
- **Sin** shell, gestor de paquetes, OpenSSL del sistema ni `tzdata`: la hora se hereda montando `/etc/localtime` del host.
- **Sin módulos `gconv` de glibc**, a propósito: `libxml2` maneja UTF-8/16, Latin-1 y ASCII de forma nativa, y quitar `iconv` elimina rutas de denegación de servicio alcanzables con codificaciones exóticas declaradas en cuerpos XML.

Consecuencias operativas:

| Quieres… | Haz esto |
|---|---|
| Entrar al contenedor | No hay shell. Usar `docker exec nginx nginx -t`, `-s reload`, `-V`; o inspeccionar desde el host |
| Ver módulos y versión | `docker exec nginx nginx -V` |
| Validar una configuración nueva | `docker run --rm -v … nginx-clang:1.31.6 nginx -t` |
| Usar CRS, la base GeoIP, `mime.types`, certificados | Montarlos desde el host (no vienen en la imagen) |

## 7. Construcción

Requisitos del equipo de build: Docker con BuildKit, acceso a Internet, y varios GB de disco libre. La compilación de LLVM descargado, OpenSSL y Nginx es la parte lenta.

```bash
cd docs/vps/recursos/nginx-image
docker build -t nginx-clang:1.31.6 .
```

Con registro del proceso (útil para auditar qué se verificó):

```bash
docker build --progress=plain -t nginx-clang:1.31.6 . 2>&1 | tee build.log
```

Cambiar un componente sin editar el archivo:

```bash
docker build --build-arg NGINX_VERSION=1.31.7 --build-arg OPENSSL_VERSION=3.5.10 \
  -t nginx-clang:1.31.7 .
```

### Verificar la imagen

```bash
# Versión, compilador y argumentos de configure
docker run --rm --entrypoint /usr/sbin/nginx nginx-clang:1.31.6 -V
#  nginx version: nginx/1.31.6
#  built by clang 23.1.3 (...)
#  built with OpenSSL 3.5.9 ...

# Contenido de la imagen: módulos, bibliotecas, ausencia de shell
c=$(docker create nginx-clang:1.31.6)
docker export "$c" | tar -t | grep -E 'usr/lib/nginx/modules/|opt/modsecurity/lib|usr/sbin/nginx'
docker export "$c" | tar -t | grep -cE '^(bin|usr/bin)/(sh|bash)$'     # 0: sin shell
docker rm "$c"

docker history nginx-clang:1.31.6
docker image ls nginx-clang
```

Resultado verificado: siete módulos `.so`, `libmodsecurity.so.3.0.17`, ningún `sh`.

## 8. Mover la imagen entre servidores (sin recompilar)

La imagen es autocontenida (trae su propia glibc), así que **no depende del sistema operativo del host**: la misma imagen compilada sobre Debian 12 corre en Debian 13.

```bash
# desde un equipo con acceso SSH a ambos servidores
ssh origen 'docker save nginx-clang:1.31.6' | ssh destino 'docker load'
```

También funciona con un archivo (`docker save -o nginx.tar` / `docker load -i nginx.tar`) o con un registro privado (GHCR). Tras cargarla, **verificar por contenido, no por ID**: el ID puede diferir entre servidores según el almacén de imágenes que use cada Docker (`containerd` vs clásico); lo fiable es `nginx -V` y el tamaño en disco.

```bash
docker run --rm --entrypoint /usr/sbin/nginx nginx-clang:1.31.6 -V
```

## 9. Mantener la imagen actualizada

1. Revisar los avisos de seguridad de Nginx, OpenSSL, PCRE2, zlib, libxml2 y ModSecurity.
2. Cambiar los `ARG` correspondientes; si el componente publica firma o suma, actualizar la verificación.
3. Construir con una etiqueta **nueva** (`nginx-clang:1.31.7`), no sobrescribir la anterior.
4. Probar la configuración real con la imagen nueva **sin tocar el servicio**. Cambiar la línea `image:` en `docker-compose.yml` a la etiqueta nueva y validar con los **mismos montajes** que el servicio (`run` crea un contenedor aparte; el que está sirviendo no se toca):
   ```bash
   cd /opt/nginx
   sed -i "s#image: nginx-clang:.*#image: nginx-clang:1.31.7#" docker-compose.yml
   docker compose run --rm --no-deps nginx nginx -t
   ```
5. Si la prueba pasa, recrear el servicio: `docker compose up -d --force-recreate`.
6. Conservar la etiqueta anterior para **volver atrás** cambiando solo la línea `image:`.
7. Escanear: `trivy image nginx-clang:1.31.7`.

## 10. Imagen alternativa de la misma familia (`nginx-template`)

Ese mismo proyecto de origen tiene otra imagen más antigua, `tools/nginx-template/`, **que no es la de este documento**:

| | `nginx-clang` (esta guía) | `nginx-template` |
|---|---|---|
| Base | `scratch` (rootfs Debian mínimo) | Alpine 3.20 |
| Nginx / OpenSSL / ModSecurity | 1.31.6 / 3.5.9 / 3.0.17 | 1.30.3 / 3.6.1 / 3.0.14 |
| Compilador | Clang + LTO | GCC, `-march=native` |
| Reglas CRS | Del host (montadas) | Dentro de la imagen (CRS 4.23 *minimal*) |
| Configuración | Montada desde el host | Plantillas + `envsubst` en el arranque (variables `MODSEC_*`, `GEOIP_*`) |
| Extras | — | Módulo OpenTelemetry, `tini`, `HEALTHCHECK` |
| Tamaño | Mucho menor | Mayor (decenas de paquetes `apk`) |
| Uso recomendado | Servidor con varios sitios, configuración y reglas gestionadas en el host | Contenedor autocontenido y parametrizado por variables de entorno |

Este repositorio usa `nginx-clang` por tamaño, superficie de ataque y porque las reglas y la configuración se actualizan sin reconstruir la imagen.

## 11. Limitaciones conocidas

- Sin shell: la depuración se hace con `docker exec nginx nginx …`, logs y montajes.
- `HEALTHCHECK` no incluido (no hay `wget`/`curl`); supervisar desde fuera ([14](14-backups-monitoreo-y-alertas.md)).
- Los hashes de algunas dependencias no se verifican (sección 3); un compromiso del repositorio de origen no se detectaría.
- Reconstruir es responsabilidad propia: no hay actualización automática. Planificar revisiones periódicas.

Continuar con [07 · Configuración de Nginx](07-nginx-configuracion.md).
