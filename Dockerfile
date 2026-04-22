## syntax=docker/dockerfile:1.7

# ---- build stage ----
FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=none

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X main.Version=${VERSION} \
      -X main.BuildTime=${BUILD_TIME} \
      -X main.GitCommit=${GIT_COMMIT}" \
    -o /out/infra-composer ./cmd/infra-composer

# ---- runtime stage ----
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/infra-composer /usr/local/bin/infra-composer
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/infra-composer"]
