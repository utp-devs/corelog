#!/usr/bin/env bash
# Fails when the documentation no longer matches the Dockerfile.
# Run it before opening a PR that changes the Dockerfile or the guide (CI can run it too):
#   ./check-docs.sh [Dockerfile]
# It checks that every component version pinned in the Dockerfile appears in the version table
# of chapter 06, that the revision/tag in the CHANGELOG is the current one, and that an archived
# copy of the current Dockerfile exists in releases/.
set -uo pipefail
cd "$(dirname "$0")"
DF="${1:-Dockerfile}"
DOCS=../..
DOC06="$DOCS/06-imagen-nginx-clang.md"
arg() { grep -m1 -E "^ARG $1=" "$DF" | cut -d= -f2; }
fail=0
bad() { echo "  OUT OF DATE  $*"; fail=1; }

echo "== version table in $(basename "$DOC06") against $DF"
check() {   # label | ARG name | text shown in the table row prefix
  local v; v=$(arg "$2")
  if grep -qE "^\| $1 .*\| $v( |\|)" "$DOC06"; then echo "  ok           $1 $v"; else bad "$1: Dockerfile pins $v, table says: $(grep -E "^\| $1 " "$DOC06" | head -1 | cut -c1-70)"; fi
}
check "Nginx" NGINX_VERSION
check "LLVM/Clang" LLVM_VERSION
check "CMake" CMAKE_VERSION
check "Ninja" NINJA_VERSION
check "pkgconf" PKGCONF_VERSION
check "OpenSSL" OPENSSL_VERSION
check "PCRE2" PCRE2_VERSION
check "zlib" ZLIB_VERSION
check "libmaxminddb" MAXMINDDB_VERSION
check "Brotli" BROTLI_VERSION
check "yajl" YAJL_VERSION
check "libxml2" LIBXML2_VERSION
check "ModSecurity \(libmodsecurity\)" MODSECURITY_VERSION
check "ModSecurity-nginx" MODSEC_NGINX_VERSION
check "headers-more-nginx-module" HEADERS_MORE_VERSION
check "ngx_http_geoip2_module" GEOIP2_MODULE_VERSION
check "ngx_brotli" NGX_BROTLI_COMMIT

echo "== revision"
TAG="$(arg NGINX_VERSION)-$(arg IMAGE_REVISION)"
grep -q "^## $TAG " CHANGELOG.md && echo "  ok           CHANGELOG.md has an entry for $TAG" || bad "CHANGELOG.md has no '## $TAG' entry"
grep -q "nginx-clang:$TAG" "$DOC06" && echo "  ok           chapter 06 mentions nginx-clang:$TAG" || bad "chapter 06 does not mention nginx-clang:$TAG"
grep -q "nginx-clang:$TAG" "$DOCS/recursos/scripts/nginx-container" && echo "  ok           scripts/nginx-container defaults to $TAG" || bad "scripts/nginx-container default image is not $TAG"

echo "== archive"
# Every published revision is archived unchanged in releases/; the current Dockerfile must equal its archive.
arch="releases/Dockerfile.$TAG"
if [ -f "$arch" ]; then
  if cmp -s "$DF" "$arch"; then echo "  ok           $arch is identical to $DF"; else bad "$arch differs from $DF (a published revision must not change: bump IMAGE_REVISION instead)"; fi
else bad "$arch does not exist (run ./build.sh, which archives it)"; fi
for f in releases/Dockerfile.*; do echo "  archived     $(basename "$f")"; done

[ "$fail" -eq 0 ] && echo "== docs match the Dockerfile" || echo "== update the documentation (see chapter 16, section 'Checklist')"
exit "$fail"
