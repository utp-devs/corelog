# Changelog de la imagen `nginx-clang`

Cada imagen se etiqueta `nginx-clang:<versión de nginx>-r<revisión>`. La **revisión** (`r1`, `r2`, …) sube cuando cambia algo de la imagen con la misma versión de nginx (dependencias, banderas, Dockerfile) y vuelve a `r1` con cada versión nueva de nginx. El `Dockerfile` de esta carpeta es siempre el vigente. **Cada** revisión publicada queda archivada, sin modificar, en [`releases/`](releases/) (la vigente es idéntica al `Dockerfile`), para poder reconstruir cualquier etiqueta.

Cómo se mantiene al día: [16 · Mantenimiento y versionado de la imagen](../../16-mantenimiento-y-versionado-de-la-imagen.md).

Cada entrada se compara con la inmediatamente anterior. Las más recientes van primero.

---

## 1.31.9-r2 · 2026-11-21 · vigente (en producción en el servidor nuevo)

Archivo: [`Dockerfile`](Dockerfile) y [`releases/Dockerfile.1.31.9-r2`](releases/Dockerfile.1.31.9-r2). Cambios respecto de `1.31.9-r1`.

**Seguridad**
- **zlib 1.3.2 → 1.4.** Corrige un desbordamiento en `gzwrite()` no bloqueante (CVE-2026-85091) y dos escrituras fuera de rango en `inflateGetDictionary()` e `inflateSync()`. Es un salto de versión menor con cambios incompatibles que no afectan a nginx (CMake/Windows, tipo `z_off_t`, `deflateReset()`); se comprobó con la prueba funcional (`gzip`, `gunzip`).
- LLVM/Clang 23.1.3 → 23.1.5. La 23.1.6 existe pero ya no publica firma GPG, que este build verifica; se usa la más reciente verificable.
- pkgconf 3.0.8 → 3.0.11; ngx_brotli `a71f931` → `53e0fb5` (solo actualiza su submódulo de Brotli, que este build ya sustituye por 1.2.1).

**Compilación más rápida**
- Etapas independientes (OpenSSL, zlib, PCRE2, libmaxminddb, yajl, libxml2, Brotli) que BuildKit ejecuta **en paralelo**.
- OpenSSL, PCRE2 y zlib se compilan aparte y se **enlazan estáticamente** en nginx: subir nginx ya no recompila OpenSSL. El build falla si el binario necesita `libssl`, `libcrypto`, `libz` o `libpcre2`; hoy solo necesita `libc.so.6` y `libcrypt.so.1`.
- LLVM se descarga como `.tar.zst` (1,2 GB frente a 2,0 GB; la ventana de 1 GB exige `zstd --long=31`) y la descarga queda en una caché de BuildKit.
- CMake genera para Ninja (Ninja 1.13.2, nuevo, con SHA-256 fijado).
- Mediciones (4 CPU, capa de toolchain en caché): nginx nuevo **17 s**; OpenSSL nuevo 50 s; ModSecurity nuevo 82 s; todas las bibliotecas más nginx 114 s; sin ninguna caché de capas 212 s. El paso de LLVM pasó de 118 s a 25 s.

**Operación**
- Etiquetas OCI y `nginx-clang.*` con la versión de cada componente (`docker image inspect`).
- Imagen de **31,7 MB** (antes 33,1 MB).
- Nuevos: `build.sh` (construye, verifica, archiva), `smoke-test.sh` (15 comprobaciones funcionales), `check-updates.sh` (versiones frente a upstream), `check-docs.sh` (documentación frente al Dockerfile) y `repro-clang.sh` (limitación del toolchain, documento 16).
- Desplegada con el esquema "Nginx como servicio del host" ([10](../../10-nginx-como-servicio-del-host.md)): unidad systemd, envoltorio `/usr/local/sbin/nginx`, workers `www-data`. Corte medido: 1,5 s.
- **Desplegada también en el servidor de referencia** (Debian 13, 19 nombres de sitio) con `upgrade-nginx-image.sh`: los 19 sitios, los puertos en escucha, el SNI desconocido y la ruta de ataque respondieron igual que antes; corte medido **2,4 s**; la configuración real validó con la imagen nueva sin cambios, porque no usaba ninguna directiva de los módulos eliminados; `certbot renew --dry-run` pasó para los 12 certificados. La `1.31.6-r1` queda cargada allí para retroceder.

**Probado y no adoptado:** OpenSSL 3.6.5, 4.0.3 y 4.1.0 compilan con nginx 1.31.9 y pasan las 15 comprobaciones (50–57 s de reconstrucción). Se mantiene la rama LTS 3.5 por política ([16](../../16-mantenimiento-y-versionado-de-la-imagen.md)); cambiar es una línea (`OPENSSL_VERSION`).

---

## 1.31.9-r1 · 2026-11-21

Archivo: [`releases/Dockerfile.1.31.9-r1`](releases/Dockerfile.1.31.9-r1). Cambios respecto de `1.31.6-r1`. Primera reconstrucción en el servidor nuevo; ya reemplazada.

**Seguridad**
- **nginx 1.31.6 → 1.31.9.** La 1.31.9 (18-nov-2026) corrige varias vulnerabilidades: desbordamiento de búfer en un worker (CVE-2026-125283), desbordamiento de la ventana de control de flujo de HTTP/2 (CVE-2026-123007), *use-after-free* en HTTP/2 y HTTP/3 con pocas `worker_connections` (CVE-2026-123633, CVE-2026-121810), en `resolver` (CVE-2026-123634), en el proxy de correo (CVE-2026-113425) y en `stream`.
- Cada fuente descargada queda **fijada por SHA-256**, además de las firmas GPG (LLVM, nginx) y las sumas oficiales (OpenSSL, CMake, ModSecurity, libxml2) que ya se verificaban.
- Ya no se compilan los módulos que un servidor solo-proxy no usa: DAV, FLV, MP4, `random_index`, `addition`, `autoindex`, SSI, FastCGI, SCGI y uwsgi.

**Versiones y base**
- Base de la imagen y del sistema de archivos: Debian 12 → **Debian 13** (glibc 2.41, GCC 14).
- CMake 4.4.4 → 4.5.1; Brotli 1.2.0 → 1.2.1; headers-more 0.40 → 0.42.
- Mismo compilador (LLVM 23.1.3) y mismo diseño de etapas en serie. 33,1 MB.

---

## 1.31.6-r1 · octubre de 2026

Archivo: [`releases/Dockerfile.1.31.6-r1`](releases/Dockerfile.1.31.6-r1), tal como estaba en el servidor de referencia (proyecto `warlay`), sin modificar.

- nginx 1.31.6 sobre Debian 12, LLVM 23.1.3, OpenSSL 3.5.9, ModSecurity 3.0.17. 46,3 MB.
- Compilación en serie, con OpenSSL, PCRE2 y zlib dentro del propio build de nginx.
