# ==================================================
#  Szerzői jog © 2025 Markus (markus@ynm.hu)
#  https://ynm.hu   – főoldal
#  https://forum.ynm.hu   – hivatalos fórum
#  https://ynm-go.ynm.hu     – bot oldala és dokumentáció
#  https://up.ynm.hu     – bot oldala és dokumentáció
#  https://bot.ynm.hu     – bot oldala és dokumentáció
#
#  Minden jog fenntartva. A kód Markus tulajdona, tilos terjeszteni vagy
#  módosítani a szerző írásos engedélye nélkül.
#
#  Ez a fájl a YnM-Go IRC-bot rendszerének része.
# ==================================================
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git gcc musl-dev sqlite-dev
WORKDIR /app

COPY go.mod go.sum ./

# modul cache + letöltés cache
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# build cache + modul cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=linux go build -o YnM-Go -ldflags="-s -w" .

# -------------------------------------------------------------

FROM alpine:3.18
RUN apk add --no-cache git gcc  musl-dev ca-certificates tzdata sqlite su-exec gawk
WORKDIR /app

COPY --from=builder /app/YnM-Go /app/YnM-Go

RUN mkdir -p /app/data /app/logs && \
    chmod -R 755 /app /app/data /app/logs

# Config template-ek bemásolása
COPY YnMConfig/ /app/YnMConfig.dist/

# Példa fájlok átnevezése + EDIT jelző hozzáadása
RUN set -eux; \
    cd /app/YnMConfig.dist; \
    find . -maxdepth 1 -type f -name "*.yaml" ! -name "example-*.yaml" -delete; \
    for f in example-*.yaml; do \
      [ -e "$f" ] || continue; \
      mv "$f" "${f#example-}"; \
    done; \
    true

# -------------------------------------------------------------
# ENTRYPOINT SCRIPT (repo-ból bemásolva)
# -------------------------------------------------------------
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# -------------------------------------------------------------
EXPOSE 2525
VOLUME ["/app/YnMConfig", "/app/data", "/app/logs"]
ENTRYPOINT ["/app/entrypoint.sh"]