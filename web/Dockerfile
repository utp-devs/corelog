ARG NODE_IMAGE=node:24-alpine
ARG NGINX_IMAGE=nginxinc/nginx-unprivileged:1.30-alpine-slim

FROM ${NODE_IMAGE} AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund --prefer-offline

FROM deps AS build
ARG BUILD_CONFIGURATION=production
ENV NG_CLI_ANALYTICS=false
COPY . .
RUN case "${BUILD_CONFIGURATION}" in production) SUFIJO=prod ;; *) SUFIJO="${BUILD_CONFIGURATION}" ;; esac \
    && npm run build:libs \
    && npm run "build:all:${SUFIJO}" \
    && mkdir -p /site/workspace /site/admin \
    && cp -r dist/portal/browser/. /site/ \
    && cp -r dist/workspace/browser/. /site/workspace/ \
    && cp -r dist/admin/browser/. /site/admin/ \
    && find /site -type f \
        \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.svg' -o -name '*.json' -o -name '*.txt' -o -name '*.ico' \) \
        -size +1k -exec gzip -9 -k -n {} +

FROM ${NGINX_IMAGE} AS runtime
ARG SECURITY_HEADERS=security-headers.conf
ENV API_UPSTREAM=http://api:8080
LABEL org.opencontainers.image.title="corelog-web" \
      org.opencontainers.image.description="Aplicaciones web de CoreLog" \
      org.opencontainers.image.vendor="jhonsferg"
COPY nginx/${SECURITY_HEADERS} /etc/nginx/snippets/security-headers.conf
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /site /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
