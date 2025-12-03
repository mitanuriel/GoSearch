FROM golang:1.24.0-alpine AS builder

RUN apk add --no-cache build-base=0.5-r3 postgresql15-dev=15.13-r0

RUN addgroup -S nonroot \
    && adduser -S nonroot -G nonroot

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY src ./src

# Disables CGO and specifies the name for the compiled application as app
RUN CGO_ENABLED=1 GOOS=linux go build -o app ./src/backend

FROM alpine:3.21.3

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Install Node.js, npm, and PostgreSQL client without pinning versions
# Version pinning causes conflicts across Alpine updates, so we accept latest stable
# hadolint ignore=DL3018
RUN apk add --no-cache nodejs npm postgresql15-client

RUN addgroup -S nonroot \
    && adduser -S nonroot -G nonroot

WORKDIR /app        

COPY --from=builder /app/app /app/app
COPY src /app/src
COPY src/frontend /app/src/frontend

COPY knex-migrations /app/knex-migrations
WORKDIR /app/knex-migrations
RUN npm ci --only=production

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /app/src/backend/backups && chown -R nonroot:nonroot /app/src/backend/backups

WORKDIR /app/src/backend

USER nonroot

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
