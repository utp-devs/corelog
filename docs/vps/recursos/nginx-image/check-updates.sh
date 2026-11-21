#!/usr/bin/env bash
# Compares the versions pinned in a nginx-clang Dockerfile with the latest upstream releases.
#   ./check-updates.sh [Dockerfile]
# Needs: curl, gh (authenticated, for the GitHub API), sort -V.
# Output status: ok | PATCH | MINOR | MAJOR | n/a. A MAJOR bump is never applied blindly:
# read the upstream notes and build it under a test tag first.
set -uo pipefail
DF="${1:-Dockerfile}"

arg() { grep -m1 -E "^ARG $1=" "$DF" | cut -d= -f2; }
gh_latest() { gh api "repos/$1/releases/latest" --jq .tag_name 2>/dev/null; }
gh_tags()   { gh api "repos/$1/tags?per_page=100" --jq '.[].name' 2>/dev/null; }
strip()     { echo "$1" | sed -E 's/^(llvmorg-|openssl-|pcre2-|v)//'; }

classify() {  # current latest
  local c="$1" l="$2"
  [ "$c" = "$l" ] && { echo ok; return; }
  [ -z "$l" ] && { echo "n/a"; return; }
  local cM=${c%%.*} lM=${l%%.*} cm lm
  cm=$(echo "$c" | cut -d. -f2); lm=$(echo "$l" | cut -d. -f2)
  if [ "$cM" != "$lM" ]; then echo MAJOR
  elif [ "$cm" != "$lm" ]; then echo MINOR
  else echo PATCH; fi
}

row() { printf '%-24s %-12s %-12s %-6s %s\n' "$1" "$2" "$3" "$(classify "$2" "$3")" "${4:-}"; }

printf '%-24s %-12s %-12s %-6s %s\n' COMPONENT PINNED LATEST STATUS NOTE
printf '%s\n' "------------------------------------------------------------------------------------------"

# nginx: mainline = highest odd minor, stable = highest even minor
nl=$(curl -fsSL https://nginx.org/en/download.html | grep -oE 'nginx-[0-9]+\.[0-9]+\.[0-9]+\.tar\.gz' | sed 's/nginx-//;s/.tar.gz//' | sort -uV)
nm=$(echo "$nl" | awk -F. '$2%2==1' | tail -1); ns=$(echo "$nl" | awk -F. '$2%2==0' | tail -1)
row nginx "$(arg NGINX_VERSION)" "$nm" "mainline (stable line: $ns)"

# LLVM: the build verifies a detached GPG signature, so compare with the newest release that has one.
llvm_all=$(strip "$(gh_latest llvm/llvm-project)")
llvm_ok=$(gh api "repos/llvm/llvm-project/releases?per_page=40" --jq '.[] | select(.assets[]?.name | test("Linux-X64\\.tar\\.zst\\.sig$")) | .tag_name' 2>/dev/null | sed 's/llvmorg-//' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1)
llvm_note="newest release with a GPG .sig"
[ "$llvm_all" != "$llvm_ok" ] && llvm_note="newest with a GPG .sig; $llvm_all exists but has no .sig, so this build cannot verify it"
row "LLVM/Clang" "$(arg LLVM_VERSION)" "$llvm_ok" "$llvm_note"
row CMake "$(arg CMAKE_VERSION)" "$(strip "$(gh_latest Kitware/CMake)")"
row Ninja "$(arg NINJA_VERSION)" "$(strip "$(gh_latest ninja-build/ninja)")"

# OpenSSL: compare against the newest patch of the PINNED line, and show newer lines
o=$(arg OPENSSL_VERSION); oline=$(echo "$o" | cut -d. -f1,2)
olatest=$(gh_tags openssl/openssl | grep -E "^openssl-${oline//./\\.}\.[0-9]+$" | sed 's/openssl-//' | sort -V | tail -1)
others=$(gh_tags openssl/openssl | grep -E '^openssl-[0-9]+\.[0-9]+\.[0-9]+$' | sed 's/openssl-//' | sort -V | awk -F. '{k=$1"."$2; l[k]=$0} END{for(k in l) print l[k]}' | sort -V | tr '\n' ' ')
row OpenSSL "$o" "$olatest" "newest patch of the $oline line; newest per line: $others"

row PCRE2 "$(arg PCRE2_VERSION)" "$(strip "$(gh_latest PCRE2Project/pcre2)")"
row zlib "$(arg ZLIB_VERSION)" "$(strip "$(gh_latest madler/zlib)")"
row libmaxminddb "$(arg MAXMINDDB_VERSION)" "$(gh_latest maxmind/libmaxminddb)"
row brotli "$(arg BROTLI_VERSION)" "$(strip "$(gh_latest google/brotli)")"
row yajl "$(arg YAJL_VERSION)" "$(gh_tags lloyd/yajl | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1)" "unmaintained upstream"
row libxml2 "$(arg LIBXML2_VERSION)" "$(strip "$(gh_tags GNOME/libxml2 | grep -E '^v2\.[0-9]+\.[0-9]+$' | sort -V | tail -1)")"
row ModSecurity "$(arg MODSECURITY_VERSION)" "$(strip "$(gh_latest owasp-modsecurity/ModSecurity)")"
row ModSecurity-nginx "$(arg MODSEC_NGINX_VERSION)" "$(strip "$(gh_latest owasp-modsecurity/ModSecurity-nginx)")"
row headers-more "$(arg HEADERS_MORE_VERSION)" "$(strip "$(gh_tags openresty/headers-more-nginx-module | grep -E '^v[0-9.]+$' | sort -V | tail -1)")"
row geoip2-module "$(arg GEOIP2_MODULE_VERSION)" "$(strip "$(gh_latest leev/ngx_http_geoip2_module)")"
row pkgconf "$(arg PKGCONF_VERSION)" "$(curl -fsSL https://distfiles.ariadne.space/pkgconf/ | grep -oE 'pkgconf-[0-9]+\.[0-9]+\.[0-9]+\.tar\.xz' | sed 's/pkgconf-//;s/.tar.xz//' | sort -uV | tail -1)"

pinned=$(arg NGX_BROTLI_COMMIT); head=$(gh api repos/google/ngx_brotli/commits/master --jq '.sha[0:7]' 2>/dev/null)
printf '%-24s %-12s %-12s %-6s %s\n' ngx_brotli "$pinned" "$head" "$([ "$pinned" = "$head" ] && echo ok || echo "commit")" "pinned by commit (no tagged releases)"
