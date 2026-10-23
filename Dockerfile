FROM node:24-bookworm-slim AS frontend
WORKDIR /src
RUN corepack enable && corepack prepare pnpm@11.13.0 --activate
COPY web/package.json web/pnpm-lock.yaml ./web/
RUN cd web && pnpm install --frozen-lockfile
COPY web ./web
COPY scripts/sync-web.mjs ./scripts/sync-web.mjs
COPY scripts/index-local-art.mjs ./scripts/index-local-art.mjs
COPY scripts/lib/webp-art.mjs ./scripts/lib/webp-art.mjs
RUN cd web && pnpm build
RUN node scripts/sync-web.mjs

FROM golang:1.26-bookworm AS backend
WORKDIR /src
ARG TARGETOS=linux
ARG TARGETARCH
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/assets/dist ./internal/assets/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/westeros ./cmd/server
RUN mkdir /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/westeros /westeros
COPY --from=backend --chown=65532:65532 /data /data
COPY LICENSE /LICENSE
ENV LISTEN_ADDR=0.0.0.0:18081 DATA_DIR=/data
EXPOSE 18081
ENTRYPOINT ["/westeros"]
