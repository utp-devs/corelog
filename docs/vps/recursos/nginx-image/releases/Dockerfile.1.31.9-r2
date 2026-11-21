# syntax=docker/dockerfile:1
#
# nginx-clang -- nginx built with Clang/LLVM from upstream sources.
# Image revision: r2  (see CHANGELOG.md; tag as nginx-clang:<nginx version>-r<revision>)
#
# The compiler, CMake, Ninja, OpenSSL, PCRE2, zlib, brotli, libmaxminddb, yajl,
# libxml2 and libmodsecurity all come from their original projects, not from apt.
# apt is used once, in the toolchain stage, for the glibc and GCC runtime
# development files (libc6-dev, libgcc-N-dev, libstdc++-N-dev): Clang links against
# them by default and they define the system ABI, so they cannot be swapped.
# Do not remove libgcc-N-dev: without it every link fails (see docs/vps/16).
#
# Every version below is defined ONCE, here. check-updates.sh reads this block.
# Every downloaded source is pinned by SHA-256, on top of the GPG signature or
# upstream checksum that is verified when the project publishes one.
#
#   docker build -t nginx-clang:1.31.9-r2 .      (or ./build.sh)
#
# The build graph is parallel: the stages openssl, zlib, pcre2, maxminddb, yajl,
# libxml2 and brotli do not depend on each other, so BuildKit runs them together.

ARG DEBIAN=debian:13
ARG GCC_MAJOR=14
ARG IMAGE_REVISION=r2

ARG NGINX_VERSION=1.31.9
ARG NGINX_SHA256=a11c5eeee2e257f4478f0e649f84b0040787d277d2d3e5233952aa6b6dcb80ad
ARG LLVM_VERSION=23.1.5
ARG CMAKE_VERSION=4.5.1
ARG NINJA_VERSION=1.13.2
ARG NINJA_SHA256=5749cbc4e668273514150a80e387a957f933c6ed3f5f11e03fb30955e2bbead6
ARG PKGCONF_VERSION=3.0.11
ARG PKGCONF_SHA256=d0cd841bc8e55b9da888106c00dc256d05b7d1d8e5a46f4be4eb4c5125bd2dc4
ARG OPENSSL_VERSION=3.5.9
ARG PCRE2_VERSION=10.49
ARG PCRE2_SHA256=929f0b20e62879252a15886b06c89f1edef61a363cbd5826fb041080a5e557ae
ARG ZLIB_VERSION=1.4
ARG ZLIB_SHA256=02dd159783677a749c04adbdd02b55b8a2bcc7e054f51b76b549199a916ad584
ARG MAXMINDDB_VERSION=1.14.1
ARG MAXMINDDB_SHA256=ca5c87d41339f8bc4daabb53e8a9356b3c995f2d2419b85d7bff823b2ecc252d
ARG BROTLI_VERSION=1.2.1
ARG BROTLI_SHA256=98d35d11a78dc453aebb7d10c3748f3f73ff78c8267c0dc697b8631f67bf7e8f
ARG YAJL_VERSION=2.1.0
ARG YAJL_SHA256=3fb73364a5a30efe615046d07e6db9d09fd2b41c763c5f7d3bfb121cd5c5ac5a
ARG LIBXML2_VERSION=2.15.4
ARG MODSECURITY_VERSION=3.0.17
ARG MODSEC_NGINX_VERSION=1.0.4
ARG MODSEC_NGINX_SHA256=6bdc7570911be884c1e43aaf85046137f9fde0cfa0dd4a55b853c81c45a13313
ARG HEADERS_MORE_VERSION=0.42
ARG HEADERS_MORE_SHA256=6f0e023e40149b19c8f9548a648ac53ac010e27829aee109abe88106609c041f
ARG GEOIP2_MODULE_VERSION=3.4
ARG GEOIP2_MODULE_SHA256=ad72fc23348d715a330994984531fab9b3606e160483236737f9a4a6957d9452
ARG NGX_BROTLI_COMMIT=53e0fb5
ARG NGX_BROTLI_SHA256=27a4352dae4ba4922f9f013f3f0d02359aceeef2701872efa42849c3ba05457a

# ---------------------------------------------------------------------------
# toolchain: compiler + build tools (cached; only rebuilt when a tool version changes)
FROM ${DEBIAN} AS toolchain
ARG GCC_MAJOR
ARG LLVM_VERSION
ARG CMAKE_VERSION
ARG NINJA_VERSION
ARG NINJA_SHA256
ARG PKGCONF_VERSION
ARG PKGCONF_SHA256
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      libc6-dev libgcc-${GCC_MAJOR}-dev libstdc++-${GCC_MAJOR}-dev \
      make perl xz-utils zstd unzip curl ca-certificates gnupg \
 && rm -rf /var/lib/apt/lists/*

# fetch URL SHA256 OUTFILE: download and verify a pinned source
RUN <<'EOF'
cat > /usr/local/bin/fetch <<'SH'
#!/bin/sh
set -eu
url=$1; sha=$2; out=$3
curl -fsSL --retry 3 -o "$out" "$url"
echo "$sha  $out" | sha256sum -c -
SH
chmod +x /usr/local/bin/fetch
EOF

WORKDIR /src
# LLVM comes as .tar.zst (40 % smaller than .tar.xz and much faster to unpack). Its frames use a
# 1 GB window, so zstd needs --long=31 to open it. The detached
# GPG signature is verified before anything is extracted. The download is kept in a BuildKit
# cache mount, so changing an unrelated argument does not download 1.2 GB again.
RUN --mount=type=cache,target=/dl,sharing=locked set -eux; \
    tar=LLVM-${LLVM_VERSION}-Linux-X64.tar.zst; \
    base=https://github.com/llvm/llvm-project/releases/download/llvmorg-${LLVM_VERSION}; \
    [ -f "/dl/$tar" ] || curl -fsSL -o "/dl/$tar" "$base/$tar"; \
    curl -fsSL -o "$tar.sig" "$base/$tar.sig"; \
    curl -fsSL https://releases.llvm.org/release-keys.asc | gpg -q --import; \
    gpg -q --verify "$tar.sig" "/dl/$tar"; \
    zstd -d --long=31 -c "/dl/$tar" | tar -x -C /opt; rm -f "$tar.sig"; \
    ln -s /opt/LLVM-${LLVM_VERSION}-Linux-X64 /opt/llvm; \
    mkdir -p /opt/llvm-toolchain/bin; \
    for t in ld:ld.lld ar:llvm-ar ranlib:llvm-ranlib nm:llvm-nm strip:llvm-strip objcopy:llvm-objcopy readelf:llvm-readelf; do \
      ln -s "/opt/llvm/bin/${t#*:}" "/opt/llvm-toolchain/bin/${t%%:*}"; \
    done

RUN set -eux; \
    tar=cmake-${CMAKE_VERSION}-linux-x86_64.tar.gz; \
    base=https://github.com/Kitware/CMake/releases/download/v${CMAKE_VERSION}; \
    curl -fsSLO "$base/$tar"; curl -fsSL -o cmake.sha256 "$base/cmake-${CMAKE_VERSION}-SHA-256.txt"; \
    sha256sum --ignore-missing -c cmake.sha256; \
    tar -xzf "$tar" -C /opt; rm -f "$tar" cmake.sha256; \
    ln -s /opt/cmake-${CMAKE_VERSION}-linux-x86_64 /opt/cmake

# Ninja: CMake builds are generated for it (-G Ninja) instead of Makefiles.
RUN set -eux; \
    fetch https://github.com/ninja-build/ninja/releases/download/v${NINJA_VERSION}/ninja-linux.zip "${NINJA_SHA256}" ninja.zip; \
    mkdir -p /opt/ninja && unzip -q ninja.zip -d /opt/ninja && rm ninja.zip

ENV PATH=/opt/llvm-toolchain/bin:/opt/llvm/bin:/opt/cmake/bin:/opt/ninja:/opt/pkgconf/bin:$PATH \
    CC=clang CXX=clang++ AR=llvm-ar NM=llvm-nm RANLIB=llvm-ranlib \
    CFLAGS="-O2 -fPIC -fstack-protector-strong" \
    CXXFLAGS="-O2 -fPIC -fstack-protector-strong" \
    PKG_CONFIG_PATH=/opt/nginx-deps/lib/pkgconfig:/opt/nginx-deps/lib64/pkgconfig \
    DEPS=/opt/nginx-deps

# pkgconf (pkg-config implementation), needed by libmodsecurity's configure.
RUN set -eux; \
    fetch https://distfiles.ariadne.space/pkgconf/pkgconf-${PKGCONF_VERSION}.tar.xz "${PKGCONF_SHA256}" pkgconf.tar.xz; \
    tar -xJf pkgconf.tar.xz; cd pkgconf-${PKGCONF_VERSION}; \
    LDFLAGS="-fuse-ld=lld" ./configure --prefix=/opt/pkgconf --disable-shared; \
    make -j"$(nproc)"; make install; \
    ln -s pkgconf /opt/pkgconf/bin/pkg-config; \
    cd /src; rm -rf pkgconf*; pkg-config --version

# ---------------------------------------------------------------------------
# Independent library stages. Each installs only into $DEPS; later stages merge them.

FROM toolchain AS openssl
ARG OPENSSL_VERSION
# Same options nginx would use for its own integrated OpenSSL build (no-shared no-threads),
# built once here so a new nginx release does not recompile it.
RUN set -eux; \
    base=https://github.com/openssl/openssl/releases/download/openssl-${OPENSSL_VERSION}; \
    curl -fsSLO "$base/openssl-${OPENSSL_VERSION}.tar.gz"; curl -fsSLO "$base/openssl-${OPENSSL_VERSION}.tar.gz.sha256"; \
    echo "$(cut -d' ' -f1 openssl-${OPENSSL_VERSION}.tar.gz.sha256)  openssl-${OPENSSL_VERSION}.tar.gz" | sha256sum -c; \
    tar -xzf openssl-${OPENSSL_VERSION}.tar.gz; cd openssl-${OPENSSL_VERSION}; \
    ./config --prefix=$DEPS --libdir=lib no-shared no-threads no-tests no-docs no-ssl3 no-weak-ssl-ciphers; \
    make -j"$(nproc)"; make install_sw

FROM toolchain AS zlib
ARG ZLIB_VERSION
ARG ZLIB_SHA256
RUN set -eux; \
    fetch https://github.com/madler/zlib/releases/download/v${ZLIB_VERSION}/zlib-${ZLIB_VERSION}.tar.gz "${ZLIB_SHA256}" zlib.tar.gz; \
    tar -xzf zlib.tar.gz; cd zlib-${ZLIB_VERSION}; \
    ./configure --static --prefix=$DEPS; make -j"$(nproc)"; make install

FROM toolchain AS pcre2
ARG PCRE2_VERSION
ARG PCRE2_SHA256
RUN set -eux; \
    fetch https://github.com/PCRE2Project/pcre2/releases/download/pcre2-${PCRE2_VERSION}/pcre2-${PCRE2_VERSION}.tar.gz "${PCRE2_SHA256}" pcre2.tar.gz; \
    tar -xzf pcre2.tar.gz; \
    cmake -G Ninja -S pcre2-${PCRE2_VERSION} -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=$DEPS \
      -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON -DPCRE2_SUPPORT_JIT=ON \
      -DPCRE2_BUILD_PCRE2GREP=OFF -DPCRE2_BUILD_TESTS=OFF; \
    cmake --build build; cmake --install build

FROM toolchain AS maxminddb
ARG MAXMINDDB_VERSION
ARG MAXMINDDB_SHA256
RUN set -eux; \
    fetch https://github.com/maxmind/libmaxminddb/releases/download/${MAXMINDDB_VERSION}/libmaxminddb-${MAXMINDDB_VERSION}.tar.gz "${MAXMINDDB_SHA256}" mmdb.tar.gz; \
    tar -xzf mmdb.tar.gz; \
    cmake -G Ninja -S libmaxminddb-${MAXMINDDB_VERSION} -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=$DEPS \
      -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON -DBUILD_TESTING=OFF; \
    cmake --build build; cmake --install build

# yajl's 2014-era CMakeLists no longer configures with CMake 4, and the
# library is a handful of C files: compile it directly.
FROM toolchain AS yajl
ARG YAJL_VERSION
ARG YAJL_SHA256
RUN set -eux; \
    fetch https://github.com/lloyd/yajl/archive/refs/tags/${YAJL_VERSION}.tar.gz "${YAJL_SHA256}" yajl.tar.gz; \
    tar -xzf yajl.tar.gz; \
    mkdir -p $DEPS/include/yajl $DEPS/lib build-yajl; \
    cp yajl-${YAJL_VERSION}/src/api/*.h $DEPS/include/yajl/; \
    major=${YAJL_VERSION%%.*}; rest=${YAJL_VERSION#*.}; minor=${rest%%.*}; micro=${rest#*.}; \
    sed -e "s/\${YAJL_MAJOR}/$major/" -e "s/\${YAJL_MINOR}/$minor/" -e "s/\${YAJL_MICRO}/$micro/" \
      yajl-${YAJL_VERSION}/src/api/yajl_version.h.cmake > $DEPS/include/yajl/yajl_version.h; \
    for f in yajl-${YAJL_VERSION}/src/*.c; do \
      clang $CFLAGS -I$DEPS/include -c "$f" -o "build-yajl/$(basename "$f" .c).o"; \
    done; \
    llvm-ar rcs $DEPS/lib/libyajl.a build-yajl/*.o; \
    mkdir -p $DEPS/lib/pkgconfig; \
    printf '%s\n' "prefix=$DEPS" 'libdir=${prefix}/lib' 'includedir=${prefix}/include' '' \
      'Name: yajl' 'Description: Yet Another JSON Library' "Version: ${YAJL_VERSION}" \
      'Cflags: -I${includedir}' 'Libs: -L${libdir} -lyajl' > $DEPS/lib/pkgconfig/yajl.pc; \
    pkg-config --modversion yajl

FROM toolchain AS libxml2
ARG LIBXML2_VERSION
RUN set -eux; \
    minor=${LIBXML2_VERSION%.*}; \
    curl -fsSLO https://download.gnome.org/sources/libxml2/$minor/libxml2-${LIBXML2_VERSION}.tar.xz; \
    curl -fsSLO https://download.gnome.org/sources/libxml2/$minor/libxml2-${LIBXML2_VERSION}.sha256sum; \
    sha256sum --ignore-missing -c libxml2-${LIBXML2_VERSION}.sha256sum; \
    tar -xJf libxml2-${LIBXML2_VERSION}.tar.xz; \
    cmake -G Ninja -S libxml2-${LIBXML2_VERSION} -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=$DEPS \
      -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
      -DLIBXML2_WITH_PYTHON=OFF -DLIBXML2_WITH_ZLIB=OFF -DLIBXML2_WITH_LZMA=OFF -DLIBXML2_WITH_ICU=OFF \
      -DLIBXML2_WITH_TESTS=OFF -DLIBXML2_WITH_PROGRAMS=OFF -DLIBXML2_WITH_DOCS=OFF; \
    cmake --build build; cmake --install build

# ngx_brotli with brotli rebuilt from the pinned release instead of its bundled submodule.
FROM toolchain AS brotli
ARG BROTLI_VERSION
ARG BROTLI_SHA256
ARG NGX_BROTLI_COMMIT
ARG NGX_BROTLI_SHA256
RUN set -eux; \
    fetch https://github.com/google/brotli/archive/refs/tags/v${BROTLI_VERSION}.tar.gz "${BROTLI_SHA256}" brotli.tar.gz; \
    tar -xzf brotli.tar.gz; \
    fetch https://github.com/google/ngx_brotli/archive/${NGX_BROTLI_COMMIT}.tar.gz "${NGX_BROTLI_SHA256}" ngx_brotli.tar.gz; \
    mkdir ngx_brotli && tar -xzf ngx_brotli.tar.gz -C ngx_brotli --strip-components=1; \
    rm -rf ngx_brotli/deps/brotli && cp -a brotli-${BROTLI_VERSION} ngx_brotli/deps/brotli; \
    cmake -G Ninja -S ngx_brotli/deps/brotli -B ngx_brotli/deps/brotli/out -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON -DBROTLI_DISABLE_TESTS=ON; \
    cmake --build ngx_brotli/deps/brotli/out --target brotlienc brotlicommon

# nginx sources and third-party module sources (downloads only; verified here).
FROM toolchain AS srcs
ARG NGINX_VERSION
ARG NGINX_SHA256
ARG MODSEC_NGINX_VERSION
ARG MODSEC_NGINX_SHA256
ARG HEADERS_MORE_VERSION
ARG HEADERS_MORE_SHA256
ARG GEOIP2_MODULE_VERSION
ARG GEOIP2_MODULE_SHA256
RUN set -eux; \
    curl -fsSLO https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz; \
    curl -fsSLO https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz.asc; \
    echo "${NGINX_SHA256}  nginx-${NGINX_VERSION}.tar.gz" | sha256sum -c -; \
    for key in arut pluknet sb thresh; do curl -fsSL https://nginx.org/keys/$key.key | gpg -q --import; done; \
    gpg -q --verify nginx-${NGINX_VERSION}.tar.gz.asc nginx-${NGINX_VERSION}.tar.gz; \
    tar -xzf nginx-${NGINX_VERSION}.tar.gz; \
    fetch https://github.com/owasp-modsecurity/ModSecurity-nginx/releases/download/v${MODSEC_NGINX_VERSION}/ModSecurity-nginx-v${MODSEC_NGINX_VERSION}.tar.gz "${MODSEC_NGINX_SHA256}" modsec-nginx.tar.gz; \
    tar -xzf modsec-nginx.tar.gz; \
    fetch https://github.com/openresty/headers-more-nginx-module/archive/refs/tags/v${HEADERS_MORE_VERSION}.tar.gz "${HEADERS_MORE_SHA256}" headers-more.tar.gz; \
    tar -xzf headers-more.tar.gz; \
    fetch https://github.com/leev/ngx_http_geoip2_module/archive/refs/tags/${GEOIP2_MODULE_VERSION}.tar.gz "${GEOIP2_MODULE_SHA256}" geoip2.tar.gz; \
    tar -xzf geoip2.tar.gz

# libmodsecurity as a shared library with its dependencies linked in
# statically: the runtime image needs nothing but libstdc++.
FROM toolchain AS modsecurity
ARG MODSECURITY_VERSION
COPY --from=pcre2 /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=yajl /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=libxml2 /opt/nginx-deps/ /opt/nginx-deps/
RUN set -eux; \
    base=https://github.com/owasp-modsecurity/ModSecurity/releases/download/v${MODSECURITY_VERSION}; \
    curl -fsSLO "$base/modsecurity-v${MODSECURITY_VERSION}.tar.gz"; \
    curl -fsSLO "$base/modsecurity-v${MODSECURITY_VERSION}.tar.gz.sha256"; \
    echo "$(cut -d' ' -f1 modsecurity-v${MODSECURITY_VERSION}.tar.gz.sha256)  modsecurity-v${MODSECURITY_VERSION}.tar.gz" | sha256sum -c; \
    tar -xzf modsecurity-v${MODSECURITY_VERSION}.tar.gz; \
    cd modsecurity-v${MODSECURITY_VERSION}; \
    pkg-config --modversion libpcre2-8 libxml-2.0 yajl; \
    PKG_CONFIG="pkg-config --static" \
    LDFLAGS="-fuse-ld=lld -L$DEPS/lib" CPPFLAGS="-I$DEPS/include -I$DEPS/include/libxml2" \
    ./configure --prefix=/opt/modsecurity \
      --with-pcre2=$DEPS --with-yajl=$DEPS --with-libxml=$DEPS \
      --without-lua --without-curl --without-geoip --without-maxmind --without-lmdb --without-ssdeep \
      --disable-doxygen-doc --disable-examples --disable-dependency-tracking; \
    make -j"$(nproc)"; make install; \
    cp unicode.mapping /opt/modsecurity/unicode.mapping; \
    rm -f /opt/modsecurity/lib/*.la /opt/modsecurity/lib/*.a

# ---------------------------------------------------------------------------
FROM toolchain AS nginx
ARG NGINX_VERSION
ARG MODSEC_NGINX_VERSION
ARG HEADERS_MORE_VERSION
ARG GEOIP2_MODULE_VERSION
COPY --from=openssl /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=zlib /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=pcre2 /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=maxminddb /opt/nginx-deps/ /opt/nginx-deps/
COPY --from=modsecurity /opt/modsecurity/ /opt/modsecurity/
COPY --from=brotli /src/ngx_brotli/ /src/ngx_brotli/
COPY --from=srcs /src/nginx-${NGINX_VERSION}/ /src/nginx-${NGINX_VERSION}/
COPY --from=srcs /src/ModSecurity-nginx-v${MODSEC_NGINX_VERSION}/ /src/ModSecurity-nginx-v${MODSEC_NGINX_VERSION}/
COPY --from=srcs /src/headers-more-nginx-module-${HEADERS_MORE_VERSION}/ /src/headers-more-nginx-module-${HEADERS_MORE_VERSION}/
COPY --from=srcs /src/ngx_http_geoip2_module-${GEOIP2_MODULE_VERSION}/ /src/ngx_http_geoip2_module-${GEOIP2_MODULE_VERSION}/
# OpenSSL, PCRE2 and zlib are linked statically from $DEPS (-I/-L below); nginx detects them there.
RUN set -eux; \
    cd nginx-${NGINX_VERSION}; \
    MODSECURITY_INC=/opt/modsecurity/include MODSECURITY_LIB=/opt/modsecurity/lib \
    ./configure \
      --with-cc=clang \
      --with-cc-opt="-O2 -flto=thin -fstack-protector-strong -D_FORTIFY_SOURCE=3 -fPIC -Wformat -Werror=format-security -I$DEPS/include" \
      --with-ld-opt="-fuse-ld=lld -flto=thin -Wl,-z,relro -Wl,-z,now -fPIC -L$DEPS/lib -Wl,-rpath,/opt/modsecurity/lib" \
      --with-pcre-jit \
      --prefix=/usr/share/nginx \
      --conf-path=/etc/nginx/nginx.conf \
      --http-log-path=/var/log/nginx/access.log \
      --error-log-path=stderr \
      --lock-path=/var/lock/nginx.lock \
      --pid-path=/run/nginx.pid \
      --modules-path=/usr/lib/nginx/modules \
      --http-client-body-temp-path=/var/lib/nginx/body \
      --http-proxy-temp-path=/var/lib/nginx/proxy \
      --with-compat \
      --with-threads \
      --with-file-aio \
      --with-http_ssl_module \
      --with-http_v2_module \
      --with-http_realip_module \
      --with-http_stub_status_module \
      --with-http_auth_request_module \
      --with-http_gzip_static_module \
      --with-http_gunzip_module \
      --with-http_sub_module \
      --with-http_slice_module \
      --with-http_secure_link_module \
      --without-http_autoindex_module \
      --without-http_ssi_module \
      --without-http_fastcgi_module \
      --without-http_scgi_module \
      --without-http_uwsgi_module \
      --with-stream=dynamic \
      --with-stream_ssl_module \
      --with-stream_ssl_preread_module \
      --with-stream_realip_module \
      --add-dynamic-module=/src/ModSecurity-nginx-v${MODSEC_NGINX_VERSION} \
      --add-dynamic-module=/src/headers-more-nginx-module-${HEADERS_MORE_VERSION} \
      --add-dynamic-module=/src/ngx_brotli \
      --add-dynamic-module=/src/ngx_http_geoip2_module-${GEOIP2_MODULE_VERSION}; \
    make -j"$(nproc)"; \
    llvm-strip --strip-unneeded objs/nginx objs/*.so; \
    echo "--- shared libraries the nginx binary needs:"; llvm-readelf -d objs/nginx | grep NEEDED; \
    ! llvm-readelf -d objs/nginx | grep -E 'NEEDED.*(libssl|libcrypto|libz|libpcre2)'; \
    install -D -m 755 objs/nginx /out/usr/sbin/nginx; \
    install -D -m 755 -t /out/usr/lib/nginx/modules objs/*.so; \
    mkdir -p /out/opt/modsecurity/lib; \
    cp -a /opt/modsecurity/lib/libmodsecurity.so* /out/opt/modsecurity/lib/; \
    llvm-strip --strip-unneeded /out/opt/modsecurity/lib/libmodsecurity.so.3.*; \
    install -D -m 644 /opt/modsecurity/unicode.mapping /out/opt/modsecurity/unicode.mapping

# ---------------------------------------------------------------------------
# Minimal distroless-style root filesystem: only the shared libraries the
# binaries actually load (glibc + libcrypt for nginx, libstdc++/libgcc for
# libmodsecurity), taken from fully patched Debian packages. Each package
# is recorded in /var/lib/dpkg/status.d/ the way distroless does it, so
# vulnerability scanners still see and report them. No shell, no package
# manager, no system OpenSSL, no tzdata (the host's /etc/localtime is mounted).
# glibc gconv modules are left out on purpose: libxml2 handles UTF-8/16,
# Latin-1 and ASCII natively, and dropping them removes the attacker-reachable
# iconv DoS paths (exotic encodings declared in XML request bodies).
FROM ${DEBIAN} AS rootfs
ARG GCC_MAJOR
ENV DEBIAN_FRONTEND=noninteractive
RUN set -eux; \
    apt-get update; apt-get upgrade -y; \
    mkdir -p /rootfs/var/lib/dpkg/status.d /rootfs/usr/lib /rootfs/usr/lib64 /rootfs/usr/bin /rootfs/usr/sbin; \
    for d in lib lib64 bin sbin; do ln -s "usr/$d" "/rootfs/$d"; done; \
    for pkg in libc6 libcrypt1 libgcc-s1 libstdc++6 gcc-${GCC_MAJOR}-base; do \
      dpkg -L "$pkg" | while read -r path; do \
        case "$path" in /usr/share/doc/*/changelog*|/usr/share/lintian/*|/usr/share/man/*|/usr/share/locale/*|*/gconv/*) continue;; esac; \
        [ -d "$path" ] && [ ! -L "$path" ] && continue; \
        [ -e "$path" ] || [ -L "$path" ] || continue; \
        real="$(readlink -f "$(dirname "$path")")/$(basename "$path")"; \
        cp -a --parents "$real" /rootfs; \
      done; \
      dpkg-query -s "$pkg" > "/rootfs/var/lib/dpkg/status.d/$pkg"; \
    done; \
    cp -a --parents /etc/os-release /usr/lib/os-release /etc/debian_version /rootfs; \
    printf '%s\n' 'root:x:0:0:root:/root:/sbin/nologin' 'www-data:x:33:33:www-data:/var/www:/sbin/nologin' \
      'nobody:x:65534:65534:nobody:/nonexistent:/sbin/nologin' > /rootfs/etc/passwd; \
    printf '%s\n' 'root:x:0:' 'adm:x:4:' 'www-data:x:33:' 'nogroup:x:65534:' > /rootfs/etc/group; \
    printf '%s\n' 'passwd: files' 'group: files' 'hosts: files dns' > /rootfs/etc/nsswitch.conf; \
    install -d -m 1777 /rootfs/tmp; \
    install -d /rootfs/run /rootfs/var/lock /rootfs/var/log/nginx /rootfs/etc/nginx /rootfs/usr/share/nginx; \
    install -d -o 33 -g 0 /rootfs/var/lib/nginx/body /rootfs/var/lib/nginx/proxy; \
    ln -s /usr/lib/nginx/modules /rootfs/usr/share/nginx/modules; \
    rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------------------
FROM scratch AS runtime
ARG NGINX_VERSION
ARG IMAGE_REVISION
ARG OPENSSL_VERSION
ARG ZLIB_VERSION
ARG PCRE2_VERSION
ARG LLVM_VERSION
ARG MODSECURITY_VERSION
LABEL org.opencontainers.image.title="nginx-clang" \
      org.opencontainers.image.description="nginx built with Clang/LLVM with ModSecurity v3, GeoIP2, headers-more and Brotli; distroless-style root filesystem" \
      org.opencontainers.image.version="${NGINX_VERSION}-${IMAGE_REVISION}" \
      org.opencontainers.image.base.name="scratch (rootfs from debian:13)" \
      org.opencontainers.image.licenses="BSD-2-Clause" \
      nginx-clang.revision="${IMAGE_REVISION}" \
      nginx-clang.nginx="${NGINX_VERSION}" \
      nginx-clang.openssl="${OPENSSL_VERSION}" \
      nginx-clang.zlib="${ZLIB_VERSION}" \
      nginx-clang.pcre2="${PCRE2_VERSION}" \
      nginx-clang.modsecurity="${MODSECURITY_VERSION}" \
      nginx-clang.llvm="${LLVM_VERSION}"
COPY --from=rootfs /rootfs/ /
COPY --from=nginx /out/ /
STOPSIGNAL SIGQUIT
CMD ["/usr/sbin/nginx", "-g", "daemon off;"]
