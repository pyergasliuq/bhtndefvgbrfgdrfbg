# ═══════════════════════════════════════════════════════════════════════════
# Dockerfile for Pweper Bot — NATIVE BUILD (собирает под процессор хоста).
# ═══════════════════════════════════════════════════════════════════════════
#
# Загрузи ИСХОДНЫЙ КОД (pweper-src.zip) + этот Dockerfile на bothost.
# Bothost сам:
#   1. Скачает Go 1.25
#   2. Скомпилирует бинарник под КОНКРЕТНЫЙ процессор сервера (GOAMD64=auto)
#   3. Скачает astcenc
#   4. Соберёт финальный образ с всем нужным
#
# Время сборки: ~2-3 минуты.
# ═══════════════════════════════════════════════════════════════════════════

# ── ЭТАП 1: Сборка ──────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

WORKDIR /build

# Копируем исходный код
COPY . .

# Скачиваем зависимости
RUN go mod download

# Сборка под КОНКРЕТНЫЙ процессор хоста (native).
# Go автоматически определит поддерживаемые инструкции (AVX2, SSE4.1, и т.д.)
# и использует их.
# CGO_ENABLED=0 — статический бинарник (без зависимостей от libc)
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /pweper-bot ./cmd/bot


# ── ЭТАП 2: Финальный образ ─────────────────────────────────────────────────
FROM debian:bookworm-slim

# Устанавливаем ca-certificates для HTTPS
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    unzip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем скомпилированный бинарник из build stage
COPY --from=builder /pweper-bot /app/pweper-bot
RUN chmod +x /app/pweper-bot

# Скачиваем и устанавливаем astcenc (SSE4.1 — работает на всех x86_64)
RUN wget -q https://github.com/ARM-software/astc-encoder/releases/download/4.7.0/astcenc-4.7.0-linux-x64.zip -O /tmp/astc.zip && \
    unzip -o /tmp/astc.zip -d /tmp/astc && \
    cp /tmp/astc/bin/astcenc-sse4.1 /usr/local/bin/astcenc && \
    chmod +x /usr/local/bin/astcenc && \
    rm -rf /tmp/astc /tmp/astc.zip

# Копируем ассеты (из исходников, т.к. они в репо)
COPY assets/ /app/assets/

# Создаём рабочие папки
RUN mkdir -p /app/data /app/logs /app/work

# Environment defaults (MUST be set via bothost panel)
ENV TOKEN="" \
    API_ID="" \
    API_HASH="" \
    ADMIN_IDS="" \
    ONLYSQ_API_KEY="" \
    DB_PATH=/app/data/users.db \
    WORK_DIR=/app/work \
    ASSETS_DIR=/app/assets \
    LOG_FILE=/app/logs/pweper.log \
    LOG_LEVEL=info \
    ENABLE_MTPROTO=1

# Volumes for persistent data
VOLUME ["/app/data", "/app/logs", "/app/work"]

# Run bot
ENTRYPOINT ["/app/pweper-bot"]
