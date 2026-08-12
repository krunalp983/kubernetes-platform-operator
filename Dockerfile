# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine3.22 AS builder
WORKDIR /workspace

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
  -trimpath \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${DATE}" \
  -o /workspace/kubernetes-platform-operator ./cmd/operator

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
LABEL org.opencontainers.image.title="kubernetes-platform-operator" \
  org.opencontainers.image.description="Kubernetes Platform Operator for ApplicationEnvironment resources" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.revision="${COMMIT}" \
  org.opencontainers.image.created="${DATE}"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/kubernetes-platform-operator /kubernetes-platform-operator

USER nonroot:nonroot
ENTRYPOINT ["/kubernetes-platform-operator"]
