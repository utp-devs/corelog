#!/usr/bin/env bash
# Builds, tags and verifies the nginx-clang image described by a Dockerfile.
#   ./build.sh                       build ./Dockerfile
#   DOCKERFILE=releases/Dockerfile.1.31.9-r1 ./build.sh
#   ./build.sh --build-arg OPENSSL_VERSION=3.6.5 --build-arg IMAGE_REVISION=r2-test
# The tag is derived from the Dockerfile: nginx-clang:<NGINX_VERSION>-<IMAGE_REVISION>.
# Extra arguments are passed to `docker build`. Set GEOIP_DB=/path/GeoLite2-Country.mmdb to
# include the GeoIP2 check in the smoke test. Set SKIP_SMOKE=1 to skip the smoke test.
set -euo pipefail
cd "$(dirname "$0")"

DF="${DOCKERFILE:-Dockerfile}"
arg() { grep -m1 -E "^ARG $1=" "$DF" | cut -d= -f2; }

NGINX_VERSION=$(arg NGINX_VERSION)
IMAGE_REVISION=$(arg IMAGE_REVISION)
# --build-arg overrides win, so the tag follows what is actually built
for a in "$@"; do
  case "$a" in
    NGINX_VERSION=*)  NGINX_VERSION="${a#*=}" ;;
    IMAGE_REVISION=*) IMAGE_REVISION="${a#*=}" ;;
  esac
done
TAG="nginx-clang:${NGINX_VERSION}-${IMAGE_REVISION}"
LOG="build-${NGINX_VERSION}-${IMAGE_REVISION}.log"

echo "== building $TAG from $DF (log: $LOG)"
start=$(date +%s)
DOCKER_BUILDKIT=1 docker build --progress=plain -f "$DF" -t "$TAG" "$@" . 2>&1 | tee "$LOG"
echo "== built in $(( $(date +%s) - start )) s"

echo "== verification"
docker run --rm --entrypoint /usr/sbin/nginx "$TAG" -V 2>&1 | head -3
docker image inspect "$TAG" --format 'labels: {{json .Config.Labels}}' | cut -c1-400
c=$(docker create "$TAG"); docker export "$c" | tar -t > /tmp/nginx-clang-image.lst; docker rm "$c" >/dev/null
echo "modules: $(grep -cE 'usr/lib/nginx/modules/.*\.so$' /tmp/nginx-clang-image.lst) (expected 7)"
echo "shell binaries: $(grep -cE '^(usr/)?s?bin/(sh|bash|dash)$' /tmp/nginx-clang-image.lst) (expected 0)"
rm -f /tmp/nginx-clang-image.lst
docker images nginx-clang --format 'image {{.Tag}} {{.Size}}' | head -5

if [ -z "${SKIP_SMOKE:-}" ] && [ -x ./smoke-test.sh ]; then
  echo "== smoke test"
  ./smoke-test.sh "$TAG" "${GEOIP_DB:-}"
fi
# Archive this revision unchanged, so any published tag can be rebuilt later.
mkdir -p releases
if [ ! -f "releases/Dockerfile.${NGINX_VERSION}-${IMAGE_REVISION}" ] && [ "$DF" = Dockerfile ]; then
  cp "$DF" "releases/Dockerfile.${NGINX_VERSION}-${IMAGE_REVISION}"
  echo "== archived releases/Dockerfile.${NGINX_VERSION}-${IMAGE_REVISION}"
fi
echo "== done: $TAG"
echo "Next: update CHANGELOG.md and chapter 06, then run ./check-docs.sh"
