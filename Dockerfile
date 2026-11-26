# Build the page. pnpm-workspace.yaml carries the supply-chain settings, and
# the lockfile pins every package.
FROM node:26-slim AS web

WORKDIR /src

RUN npm install --global pnpm@12.23.0

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY vite.config.ts tailwind.config.ts tsconfig.json tsconfig.app.json tsconfig.node.json ./
COPY web ./web
RUN pnpm run build

# Build the binary with the page embedded. CGO is off so the result runs on a
# distroless static image.
FROM golang:1.27 AS build

WORKDIR /src

# Dependencies are copied first so a source-only change reuses the module layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/internal/site/assets/app ./internal/site/assets/app

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/keyfordle ./cmd/keyfordle

# Run as a non-root user with no shell and no package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/keyfordle /keyfordle

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/keyfordle"]
