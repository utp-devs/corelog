# 16 · Mantenimiento y versionado de la imagen

← [15 Verificación y auditoría](15-verificacion-y-auditoria.md) · [Índice](README.md)

**Objetivo:** que la imagen `nginx-clang` y esta documentación no se queden atrás: cómo se versiona cada revisión, cómo se detectan versiones nuevas de las dependencias, cómo se actualiza y se despliega sin sorpresas, cómo se compila más rápido y qué limitaciones del compilador conviene conocer.

Archivos: [`recursos/nginx-image/`](recursos/nginx-image/): `Dockerfile` (vigente), `releases/` (todas las revisiones), `CHANGELOG.md`, `build.sh`, `smoke-test.sh`, `check-updates.sh`, `check-docs.sh`, `repro-clang.sh`.

## 1. Esquema de versionado

Cada imagen se etiqueta **`nginx-clang:<versión de nginx>-r<revisión>`**:

| Parte | Significado |
|---|---|
| `<versión de nginx>` | La de nginx incluida (`1.31.9`) |
| `r<N>` | **Revisión de la imagen**: sube cuando cambia algo (dependencias, banderas, módulos, Dockerfile) con la **misma** versión de nginx. Vuelve a `r1` con cada versión nueva de nginx |

Ejemplos reales: `1.31.6-r1` (original) → `1.31.9-r1` → `1.31.9-r2` (vigente).

Reglas:

- **Una revisión publicada no se modifica.** Si hay que cambiar algo, se sube `IMAGE_REVISION`. `check-docs.sh` falla si `releases/Dockerfile.<tag>` difiere del vigente.
- **Cada revisión queda archivada** sin cambios en [`releases/Dockerfile.<tag>`](recursos/nginx-image/releases/): `build.sh` la copia al construir. Así cualquier etiqueta publicada se puede reconstruir.
- El `Dockerfile` de la raíz es siempre el de la última revisión.
- Las revisiones de prueba usan sufijo (`r2-test`, `r2-ossl3.6.5`) y no se archivan ni se despliegan.
- La imagen lleva etiquetas OCI y `nginx-clang.revision`, `.nginx`, `.openssl`, `.zlib`, `.pcre2`, `.modsecurity`, `.llvm`: se puede consultar la versión de cada componente de un contenedor en marcha con `docker image inspect`.
- Las etiquetas viejas se conservan en el servidor **al menos una revisión** para poder volver atrás (sección 7).

### Cada Dockerfile declara su revisión

Las versiones están en un único bloque `ARG` al inicio del Dockerfile (incluida `IMAGE_REVISION`), junto con el SHA-256 de cada descarga. Es la **única fuente de verdad**: `check-updates.sh`, `check-docs.sh`, `build.sh` y las etiquetas de la imagen leen de ahí.

## 2. Mantener Dockerfile y documentación sincronizados

| Herramienta | Qué hace | Cuándo |
|---|---|---|
| [`check-updates.sh`](recursos/nginx-image/check-updates.sh) | Compara cada versión fijada con la última publicada (nginx, LLVM, CMake, Ninja, OpenSSL, PCRE2, zlib, libmaxminddb, Brotli, yajl, libxml2, ModSecurity, módulos, pkgconf, ngx_brotli) y clasifica `ok` / `PATCH` / `MINOR` / `MAJOR` | Al menos una vez al mes, y siempre que haya un aviso de seguridad |
| [`build.sh`](recursos/nginx-image/build.sh) | Construye, etiqueta según el Dockerfile, verifica, ejecuta la prueba funcional y archiva en `releases/` | En cada cambio |
| [`smoke-test.sh`](recursos/nginx-image/smoke-test.sh) | 15 comprobaciones funcionales sobre un nginx de prueba en puertos altos | Lo ejecuta `build.sh`; también a mano antes de desplegar |
| [`check-docs.sh`](recursos/nginx-image/check-docs.sh) | **Falla** si la tabla de versiones del documento 06 no coincide con el Dockerfile, si falta la entrada del `CHANGELOG`, si `nginx-container` no apunta a la etiqueta vigente o si el archivo de `releases/` difiere | Antes de abrir el PR; ideal en CI |

```bash
cd docs/vps/recursos/nginx-image
./check-updates.sh                 # ¿qué hay de nuevo?
./check-docs.sh                    # ¿la documentación coincide con el Dockerfile?
```

Salida de `check-updates.sh` en el momento de redactar esto (todo al día; LLVM 23.1.6 existe pero no se puede verificar):

```
COMPONENT    PINNED   LATEST   STATUS  NOTE
nginx        1.31.9   1.31.9   ok      mainline (stable line: 1.30.7)
LLVM/Clang   23.1.5   23.1.5   ok      newest with a GPG .sig; 23.1.6 exists but has no .sig, so this build cannot verify it
OpenSSL      3.5.9    3.5.9    ok      newest patch of the 3.5 line; newest per line: … 3.5.9 3.6.5 4.0.3 4.1.0
zlib         1.4      1.4      ok
...
```

## 3. Política de versiones

| Componente | Política |
|---|---|
| **nginx** | Mainline (`1.31.x`) por defecto: recibe antes las correcciones. Si se prefiere estabilidad, la rama estable (`1.30.x`) cambia el ritmo de actualizaciones, no la seguridad. Subir **en cuanto haya un aviso de seguridad** |
| **OpenSSL** | Última versión de la rama **LTS** (hoy 3.5.x). Ver "Por qué OpenSSL 3.5 y no una rama más nueva" |
| **zlib, PCRE2, libxml2, Brotli, libmaxminddb** | Última versión estable. Una subida de versión **mayor o menor** se prueba con `smoke-test.sh` antes de adoptarla (zlib 1.4 incluye cambios incompatibles y correcciones de seguridad; se adoptó tras la prueba) |
| **ModSecurity, ModSecurity-nginx, headers-more, geoip2, ngx_brotli** | Última release; ngx_brotli se fija por *commit* (no publica versiones) |
| **LLVM/Clang** | Última versión **con firma GPG** publicada. Si una versión nueva no la trae, no se adopta hasta poder verificarla (ver sección 6) |
| **CMake, Ninja, pkgconf** | Última estable (herramientas de build; no están en la imagen final) |
| **Base Debian** | La versión estable vigente; la imagen final solo copia glibc, libstdc++ y libgcc de paquetes totalmente actualizados al construir |

### Por qué OpenSSL 3.5 y no una rama más nueva

`check-updates.sh` muestra la última versión de cada rama de OpenSSL. En la fecha de este documento:

| Rama | Última | Fecha | Compila con nginx 1.31.9 | `smoke-test.sh` | Reconstrucción |
|---|---|---|---|---|---|
| **3.5 (LTS)** | **3.5.9** | 2026-09-29 | Sí | 15/15 | (valor por defecto) |
| 3.6 | 3.6.5 | 2026-09-29 | Sí | 15/15 | 52 s |
| 4.0 | 4.0.3 | 2026-09-29 | Sí | 15/15 | 52 s |
| 4.1 | 4.1.0 | 2026-10-23 | Sí | 15/15 | 57 s |

Lectura de los datos:

- **No falta ningún parche.** 3.5.9 es la última de su rama y se publicó el mismo día que las últimas de 3.6 y 4.0: las correcciones de seguridad de ese día están en todas.
- Las ramas más nuevas **sí funcionan** con esta imagen (se compilaron y pasaron la prueba funcional completa, incluidos TLS 1.3 con HTTP/2 y los rechazos de TLS 1.0). Probarlo es barato: `./build.sh --build-arg OPENSSL_VERSION=4.1.0 --build-arg IMAGE_REVISION=r2-test` (~1 minuto).
- Se prefiere la **rama LTS** porque recibe correcciones durante más tiempo y cambia menos entre actualizaciones; una rama nueva (versión mayor) puede retirar opciones y comportamientos heredados que la prueba funcional no cubre (cifrados, certificados antiguos, clientes viejos). Conviene revisar las notas de la versión y las capacidades de los clientes reales antes de pasar a producción.
- La decisión es del operador: cambiar es una línea (`ARG OPENSSL_VERSION`), subir `IMAGE_REVISION` y ~1 minuto de compilación. Esta tabla debe revisarse cada vez que salga una rama nueva o la LTS cambie de fecha de soporte (consultar la política de soporte de OpenSSL).

## 4. Checklist de actualización

Seguir en este orden. **Cada paso que toca la imagen toca también la documentación.**

1. **Detectar.** `./check-updates.sh`. Leer las notas de versión de lo que cambia (seguridad, cambios incompatibles). Los `MAJOR` no se aplican a ciegas.
2. **Rama.** `git checkout -b docs/nginx-image-<tag>` desde `main`.
3. **Dockerfile.**
   - Cambiar el `ARG` de cada componente.
   - Si el componente no publica firma/suma verificable, **recalcular y actualizar su `*_SHA256`** (`curl -fsSL URL | sha256sum`).
   - Subir `IMAGE_REVISION` (`r2` → `r3`), o ponerlo en `r1` si cambia la versión de nginx.
4. **Construir y probar.** `./build.sh`: construye, verifica, ejecuta `smoke-test.sh` (debe dar 15/15) y archiva `releases/Dockerfile.<tag>`.
5. **Actualizar la documentación.**
   - [`CHANGELOG.md`](recursos/nginx-image/CHANGELOG.md): nueva entrada **encima** con fecha, etiqueta y qué cambió respecto de la anterior (separar seguridad, versiones, compilación, operación).
   - La tabla de versiones y la etiqueta del documento [06](06-imagen-nginx-clang.md).
   - `recursos/scripts/nginx-container`: valor por defecto de `IMAGE`.
   - Si cambian banderas, módulos o etapas: documento 06; si cambia lo operativo: [10](10-nginx-como-servicio-del-host.md).
6. **Comprobar el desfase.** `./check-docs.sh` debe terminar sin `OUT OF DATE`.
7. **PR.** Commit y descripción en inglés, plantilla del repositorio, sin atribuciones. Incluir en la descripción la salida de `check-updates.sh`, el resultado de `smoke-test.sh` y los tiempos.
8. **Desplegar** en cada servidor (sección 7) y **medir**: respuestas idénticas a la línea base y `nginx -V` con la versión nueva. Anotar en el `CHANGELOG` en qué servidores quedó desplegada.

## 5. Compilar más rápido: qué funcionó y qué no

Mediciones en el servidor de producción (4 CPU, 8 GB, imagen de toolchain en caché), comparando el diseño anterior (etapas en serie, OpenSSL dentro de nginx) con el actual.

### Lo que sí aceleró

| Escenario | Antes (r1) | Ahora (r2) | Mejora |
|---|---|---|---|
| Subir **solo nginx** | ~80 s (nginx 74 s + fuentes) | **17 s** | ~4,7 veces |
| Subir **OpenSSL** | ~165 s (invalida todas las capas siguientes de la etapa en serie) | 50 s | ~3,3 veces |
| Subir **ModSecurity** | ~140 s | 82 s | ~1,7 veces |
| Reconstruir **todas las bibliotecas** + nginx | ~165 s (en serie) | **114 s** (en paralelo) | ~1,4 veces |
| Sin ninguna capa en caché (LLVM descargado) | — | 212 s | — |
| Paso de **LLVM** | 118 s (`.tar.xz`, 2,0 GB) | 25 s con descarga en caché (`.tar.zst`, 1,2 GB); 52 s de descarga la primera vez | ~4,7 veces |

(Los valores "antes" son **estimaciones**: suma de los tiempos de etapa del registro del build anterior, que era en serie, contando las capas que un cambio de versión invalida. Los "ahora" son tiempos reales de pared medidos con `--no-cache-filter` para forzar la reconstrucción de esas etapas.)

Qué lo explica, en orden de importancia:

1. **OpenSSL fuera de nginx, enlazado estáticamente.** Antes cada versión nueva de nginx recompilaba OpenSSL. Ahora OpenSSL, PCRE2 y zlib viven en etapas propias que la caché de capas reutiliza.
2. **Etapas en paralelo.** `openssl`, `zlib`, `pcre2`, `maxminddb`, `yajl`, `libxml2` y `brotli` no dependen entre sí; BuildKit las ejecuta a la vez. El camino más largo pasa a ser `modsecurity` (82 s) en lugar de la suma.
3. **LLVM en `.tar.zst`.** Un archivo 40 % más pequeño y una descompresión mucho más rápida. Requiere `zstd --long=31` (ver sección 6).
4. **Caché de BuildKit para la descarga de LLVM** (`RUN --mount=type=cache`): cambiar un `ARG` ya no vuelve a bajar 1,2 GB.

### Lo que casi no aportó: Ninja y Meson

Se midieron configurar + compilar libxml2 (la mayor de las dependencias con CMake), 4 CPU, promedio de 3 ejecuciones:

| Herramienta | Tiempo | Notas |
|---|---|---|
| CMake + Make (`-j4`) | 3,2 s | Lo que se usaba |
| CMake + **Ninja** | 3,4 s | Diferencia dentro del ruido de medición |
| **Meson** + Ninja | 3,5 s | Exige instalar `python3`, `meson` y `git` desde `apt` (~8 s y decenas de MB extra) solo para esta medición |

**Conclusión:** las bibliotecas con CMake compilan en segundos; el generador no es el cuello de botella. El Dockerfile usa Ninja (`-G Ninja`) porque no cuesta nada y es el estándar, **pero no es lo que acelera el build**. Meson no se adopta: no lo usa ninguna otra pieza (nginx, OpenSSL y ModSecurity tienen sus propios sistemas de compilación) y añadiría Python a la cadena de herramientas.

Las piezas realmente lentas (nginx con LTO ~15 s, OpenSSL ~50 s, libmodsecurity ~80 s) no se acortan cambiando de generador: usan `make`/autotools propios.

### Otras opciones, sin medir

| Opción | Efecto esperado | Coste |
|---|---|---|
| Publicar la etapa `toolchain` como imagen (p. ej. en GHCR) y usarla como base | Un equipo o un CI limpio se ahorra bajar LLVM y compilar pkgconf (~140 s) | Mantener otra imagen y su versionado |
| Caché de BuildKit en el registro (`--cache-to/--cache-from type=registry`) en CI | Un *runner* efímero reutiliza capas | Configurar el pipeline |
| `ccache` con caché de BuildKit | Ayuda cuando se invalida la capa pero casi nada cambió | Complejidad; el diseño por etapas ya evita la mayoría de recompilaciones |
| Quitar `-flto=thin` | Enlace más rápido | Pérdida de optimización en tiempo de ejecución; **no recomendado** |

## 6. Limitación conocida del toolchain: Clang sin `libgcc-dev`

**Síntoma.** Con el LLVM descargado de su proyecto (no el de `apt`) en un Debian con solo `libc6-dev`, cualquier enlace falla aunque Clang compile bien los objetos:

```
ld.lld: error: cannot open crtbeginS.o: No such file or directory
ld.lld: error: unable to find library -lgcc
ld.lld: error: unable to find library -lgcc_s
ld.lld: error: cannot open crtendS.o: No such file or directory
clang: error: linker command failed with exit code 1
```

Y en C++:

```
/t.cpp:1:10: fatal error: 'iostream' file not found
```

**Causa.** El LLVM de la release solo trae Clang, `lld` y sus bibliotecas; **no** incluye el *runtime* de GCC que Clang usa por defecto en Linux. Clang busca una instalación de GCC en `/usr/lib/gcc/x86_64-linux-gnu/<N>/` y de ahí toma:

- `crtbeginS.o` y `crtendS.o` (objetos de arranque y cierre),
- `libgcc.a` y **`libgcc_s.so`** (el enlace simbólico de desarrollo para `-lgcc_s`),
- `libstdc++.so` y las cabeceras de C++.

En Debian esos archivos **no** están en `libc6-dev` ni en los paquetes de ejecución (`libgcc-s1`, `libstdc++6`, que sí traen `libgcc_s.so.1`, pero **no** el `.so` de enlace sin número de versión): están en **`libgcc-<N>-dev`** y **`libstdc++-<N>-dev`**. Por eso "falta un `.so`": el `libgcc_s.so` de desarrollo.

**Reproducirlo y ver la solución.** [`recursos/nginx-image/repro-clang.sh`](recursos/nginx-image/repro-clang.sh) arranca un contenedor con la etapa `toolchain`, desinstala esos dos paquetes, muestra los errores, los vuelve a instalar y comprueba que C y C++ enlazan (tarda unos 6 s con la imagen en caché):

```
== state B: remove the two development packages
linker symlink libgcc_s.so (needed by -lgcc_s)      : cannot access '/usr/lib/gcc/x86_64-linux-gnu/14/libgcc_s.so'
crtbeginS.o (startup object)                        : cannot access '/usr/lib/gcc/x86_64-linux-gnu/14/crtbeginS.o'
C   with lld      : ld.lld: error: cannot open crtbeginS.o: No such file or directory
== state C: install them again (the fix)
C   with lld : COMPILED_OK
C++ with lld : hello from libstdc++
files that come from the dev packages:
crtbegin.o crtbeginS.o crtbeginT.o crtend.o crtendS.o libgcc.a libgcc_s.so libstdc++.a libstdc++.so libstdc++exp.a libstdc++fs.a
```

**Solución adoptada.** En la etapa `toolchain`, de `apt` se instalan solo `libc6-dev`, `libgcc-<N>-dev` y `libstdc++-<N>-dev` (N = versión de GCC de la distro: 12 en Debian 12, 14 en Debian 13, parametrizada con `ARG GCC_MAJOR`). Son el único uso de `apt` en el toolchain porque definen la ABI del sistema y **no se pueden sustituir** por el LLVM descargado. Al cambiar de versión de Debian hay que cambiar `GCC_MAJOR` (y `gcc-<N>-base` en el `rootfs`).

**Alternativa sin GCC** (no adoptada): usar el *runtime* propio de LLVM (`compiler-rt`, `libunwind`, `libc++`) con `--rtlib=compiler-rt --unwindlib=libunwind --stdlib=libc++`. Evita depender de los paquetes de GCC, pero obliga a compilar `libc++` y enlazar libmodsecurity (C++) contra ella, y a que todo el resto sea consistente. Más trabajo y más riesgo de incompatibilidad ABI con las bibliotecas del sistema.

### Otras particularidades del toolchain descubiertas al actualizar

- **`zstd` necesita `--long=31` para abrir el `.tar.zst` de LLVM.** Sus *frames* usan una ventana de 1 GB y `tar --zstd` falla con `Window size larger than maximum: 1073741824 > 134217728`. El Dockerfile usa `zstd -d --long=31 -c … | tar -x`.
- **LLVM 23.1.6 no publica `.sig`.** Desde esa versión solo hay atestaciones Sigstore (`.jsonl`); el build verifica la firma GPG separada, así que usa la última que sí la trae (23.1.5). Cuando se quiera adoptar una versión sin `.sig`, hay que añadir antes la verificación Sigstore (p. ej. con `cosign`/`gh attestation verify`); **no** se debe quitar la verificación.
- **`set -e` no detiene un `! comando`.** Una comprobación escrita como `! llvm-readelf … | grep …` dentro de un `RUN set -eux` **no** hace fallar el build aunque el `grep` coincida; hay que usar `if … ; then exit 1; fi`.

## 7. Desplegar una revisión nueva y volver atrás

Con el modo servicio del host ([10](10-nginx-como-servicio-del-host.md)). El procedimiento se aplicó en **dos servidores**: el nuevo (migrado desde Compose) y el de referencia (de `1.31.6-r1` a `1.31.9-r2`, con 19 nombres de sitio).

### Antes de tocar el servicio

1. **Cargar la imagen** en el servidor (si se construyó en otro equipo):
   ```bash
   ssh construccion 'docker save nginx-clang:1.31.10-r1' | ssh servidor 'docker load'
   ```
2. **Comprobar la compatibilidad de la configuración.** Si la revisión quitó módulos (la `r2` no compila DAV, FLV, MP4, `random_index`, `addition`, `autoindex`, SSI, FastCGI, SCGI ni uwsgi), buscar sus directivas en la configuración **efectiva**:
   ```bash
   nginx -T 2>/dev/null | grep -nE '^\s*(autoindex|ssi|fastcgi_|scgi_|uwsgi_|dav_methods|mp4|flv|random_index|add_before_body|add_after_body)' | grep -v '^\S*\s*#'
   # sin salida = la configuración no usa ninguno
   ```
   Y comprobar que los `load_module` de la configuración existen en la imagen nueva.
3. **Pre-vuelo con la configuración real**, sin tocar el servicio:
   ```bash
   NGINX_IMAGE=nginx-clang:1.31.10-r1 nginx-container test       # debe terminar en "test is successful"
   ./smoke-test.sh nginx-clang:1.31.10-r1                         # prueba funcional en puertos altos
   ```

### Conmutar con línea base y retroceso automático

[`recursos/scripts/upgrade-nginx-image.sh`](recursos/scripts/upgrade-nginx-image.sh):

```bash
NEW=nginx-clang:1.31.10-r1 ./upgrade-nginx-image.sh
```

1. Lee los `server_name` de la configuración efectiva y mide **dos veces** cada sitio (HTTP, HTTPS y tamaño): lo que cambia entre las dos muestras es contenido dinámico y solo se compara su código de estado.
2. Guarda un respaldo del script de arranque y registra los puertos en escucha, el SNI desconocido y una ruta de ataque.
3. Cambia la etiqueta en `nginx-container` y ejecuta `systemctl restart nginx` (la unidad valida la configuración antes de arrancar). Imprime el corte.
4. **Compara todo con la línea base.** Si **cualquier** código, tamaño estable o comprobación extra difiere, restaura el script anterior, reinicia y termina con error.
5. Comprueba que el envoltorio `nginx` sea el primero del `PATH`, `nginx -t`, `systemctl reload nginx`, el usuario de los workers y el estado del servicio.

### Resultado en el servidor de referencia (`1.31.6-r1` → `1.31.9-r2`)

| Comprobación | Resultado |
|---|---|
| Configuración con la imagen nueva (pre-vuelo) | Válida, sin cambios (805 reglas de ModSecurity cargadas) |
| 19 nombres de sitio, HTTP y HTTPS | Idénticos a la línea base |
| Puertos en escucha, SNI desconocido, ruta de ataque | Idénticos |
| Corte | **2,4 s** |
| Workers | `www-data` |
| TLS | TLS 1.3, certificado válido; Brotli y gzip activos; la cabecera `Server` sigue oculta |
| WAF | Sigue evaluando y registrando en el log de auditoría |
| Logs por sitio y consola | Reciben tráfico; sin `emerg`/`alert`/`crit` nuevos |
| fail2ban, logrotate, `systemctl --failed` | Sin cambios; 0 unidades fallidas |
| `certbot renew --dry-run` | Las 12 renovaciones simuladas pasan (varios certificados usan el *plugin* `nginx`, que llama a `nginx -V`, `-t` y `-s reload` por el envoltorio) |

> **La línea base de `GET /` no basta.** Cubre que el sitio responde, no el comportamiento de cada aplicación. Tras el despliegue se probaron además un WebSocket, el WAF, TLS y la compresión. Dos resultados parecían una regresión y **no lo eran**: un WebSocket dio 404 y un SQLi a una API dio 200. Se comprobó que (1) el backend responde igual **sin nginx** (404 a `/ws`, 200 al SQLi) y (2) el CRS de ese servidor usa un umbral de anomalía de 10, de modo que una sola regla crítica (5 puntos) no bloquea. Ante una diferencia así, antes de culpar a la imagen hay que separar **qué responde la aplicación** de **qué decide el WAF** y de **qué hace nginx**.

### Volver atrás

```bash
sed -i 's#nginx-clang:1.31.10-r1#nginx-clang:1.31.9-r2#' /usr/local/sbin/nginx-container
systemctl restart nginx
```

Por eso **no se borra la imagen anterior** hasta tener la nueva comprobada y estable.

Para una migración desde el modo Compose o una puesta en marcha, el script [`migrate-nginx-to-host.sh`](recursos/scripts/migrate-nginx-to-host.sh) hace lo equivalente.

## 8. Verificación

| Comprobación | Esperado |
|---|---|
| `./check-updates.sh` | Todo `ok` (o `MAJOR` conocidos y justificados) |
| `./check-docs.sh` | Sin `OUT OF DATE` |
| `./build.sh` | `15 passed, 0 failed`, y `releases/Dockerfile.<tag>` creado |
| `docker image inspect nginx-clang:<tag> --format '{{json .Config.Labels}}'` | Revisión y versiones coherentes con la tabla del documento 06 |
| `nginx -V` en el servidor | La versión de nginx y de OpenSSL esperadas, `built by clang` |
| Línea base de sitios antes y después de desplegar | Idéntica |
| `ls releases/` | Un archivo por revisión publicada |

← Volver al [índice](README.md)
