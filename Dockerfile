FROM node:24-slim AS frontend-build
ARG VITE_TURNSTILE_SITE_KEY
ENV VITE_TURNSTILE_SITE_KEY=$VITE_TURNSTILE_SITE_KEY
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.26-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -o /pkbm-server ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=backend-build /pkbm-server ./pkbm-server
COPY --from=frontend-build /app/frontend/dist ./public
EXPOSE 8080
CMD ["./pkbm-server"]
