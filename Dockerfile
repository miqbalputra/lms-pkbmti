FROM node:24-slim AS frontend-build
ARG VITE_TURNSTILE_SITE_KEY
ENV VITE_TURNSTILE_SITE_KEY=$VITE_TURNSTILE_SITE_KEY
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.25-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -o /pkbm-server ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates postgresql-client su-exec \
    && addgroup -S -g 10001 lms \
    && adduser -S -D -H -u 10001 -G lms lms \
    && mkdir -p /app/backups /app/uploads \
    && chown -R lms:lms /app
COPY --from=backend-build /pkbm-server ./pkbm-server
COPY --from=frontend-build /app/frontend/dist ./public
COPY deploy/entrypoint.sh /usr/local/bin/pkbm-entrypoint
RUN chmod 0755 /usr/local/bin/pkbm-entrypoint
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 CMD wget --spider -q http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/pkbm-entrypoint"]
CMD ["./pkbm-server"]
