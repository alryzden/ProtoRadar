GO ?= go
DOCKER ?= docker
DOCKER_COMPOSE ?= docker compose
CLI_IMAGE ?= protoradar-cli:local
SERVER_IMAGE ?= protoradar-server:local
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/alryzden/ProtoRadar/internal/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')
DOCKER_BUILD_ARGS := --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE)

.PHONY: build test test-race vet fmt check-gitlab-templates docker-build docker-build-server docker-build-cli docker-build-images docker-smoke-cli docker-up docker-down migrate demo smoke-test

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

check-gitlab-templates:
	./scripts/check-gitlab-templates.sh

docker-build:
	$(DOCKER_COMPOSE) build

docker-build-server:
	$(DOCKER) build --target server -t $(SERVER_IMAGE) $(DOCKER_BUILD_ARGS) .

docker-build-cli:
	$(DOCKER) build --target cli -t $(CLI_IMAGE) $(DOCKER_BUILD_ARGS) .

docker-build-images: docker-build-server docker-build-cli

docker-smoke-cli:
	CLI_IMAGE=$(CLI_IMAGE) ./scripts/smoke-cli-image.sh

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
