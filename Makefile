GO ?= go
DOCKER_COMPOSE ?= docker compose
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/alryzden/ProtoRadar/internal/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')

.PHONY: build test test-race vet fmt docker-build docker-up docker-down migrate demo smoke-test

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/protoradar-server ./cmd/protoradar-server
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/protoradar ./cmd/protoradar

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $(GO_FILES)

docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up -d --build

docker-down:
	$(DOCKER_COMPOSE) down

migrate: docker-up
	@echo "Migrations are applied by protoradar-server during startup."

demo: build
	./scripts/demo.sh

smoke-test:
	./scripts/smoke-test.sh
