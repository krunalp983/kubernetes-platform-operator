SHELL := /bin/bash

BINARY_NAME ?= kubernetes-platform-operator
IMG ?= ghcr.io/<PERSONAL_GITHUB_USERNAME>/<PERSONAL_GITHUB_REPOSITORY>:dev

.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./... -coverprofile=coverage.out

.PHONY: build
build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/operator

.PHONY: run
run:
	go run ./cmd/operator

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: manifests
manifests:
	kubectl kustomize config > /tmp/kubernetes-platform-operator-install.yaml

.PHONY: helm-lint
helm-lint:
	helm lint charts/kubernetes-platform-operator
	helm template kubernetes-platform-operator charts/kubernetes-platform-operator --namespace kubernetes-platform-operator-system > /tmp/kubernetes-platform-operator-helm-render.yaml

.PHONY: validate
validate: test manifests helm-lint

.PHONY: lint
lint: fmt vet

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) \
		--build-arg VERSION=dev \
		--build-arg COMMIT=local \
		--build-arg DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ) .
