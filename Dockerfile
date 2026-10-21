# Francois hosted catalog image: Go server + built UI + Node Pi worker.
# Used by CI (ghcr.io/go-go-golems/francois) and deploy/compose.
FROM node:24-bookworm-slim AS web
RUN corepack enable && corepack prepare pnpm@10.13.1 --activate
WORKDIR /src
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY tsconfig.json ./
COPY scripts ./scripts
COPY ui ./ui
COPY worker/src ./worker/src
COPY pkg/web/static ./pkg/web/static
COPY pkg/catalog/web/static ./pkg/catalog/web/static
RUN pnpm build && pnpm install --prod --frozen-lockfile

FROM golang:1.26.10 AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/pkg/web/static ./pkg/web/static
COPY --from=web /src/pkg/catalog/web/static ./pkg/catalog/web/static
RUN CGO_ENABLED=0 go build -trimpath -o /out/francois ./cmd/francois

FROM node:24-bookworm-slim
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="francois" \
      org.opencontainers.image.description="Francois hosted catalog: subscriber portal, isolated instance origins and Pi model worker" \
      org.opencontainers.image.source="https://github.com/go-go-golems/francois" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
RUN apt-get update && apt-get install -y --no-install-recommends curl jq ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=web /src/node_modules ./node_modules
COPY --from=web /src/package.json ./package.json
COPY --from=web /src/worker/dist ./worker/dist
COPY --from=server /out/francois /usr/local/bin/francois
COPY examples ./examples
COPY deploy/compose/bin/ /usr/local/bin/
RUN useradd --system --uid 10001 francois && mkdir -p /var/lib/francois && chown francois /var/lib/francois
USER francois
ENTRYPOINT ["francois"]
