.PHONY: proto build build-web build-shield build-bridges build-all test lint clean

PROTOC ?= protoc

proto:
	$(PROTOC) --proto_path=proto \
	       --go_out=. --go_opt=module=github.com/openparallax/openparallax \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/openparallax/openparallax \
	       proto/openparallax/v1/*.proto

build-web:
	@if [ -f web/package.json ]; then \
		cd web && npm install --silent && npm run build; \
	fi

build:
	@mkdir -p dist
	CGO_ENABLED=0 go build -o dist/openparallax ./cmd/agent

build-shield:
	@mkdir -p dist
	CGO_ENABLED=0 go build -o dist/openparallax-shield ./cmd/shield

build-bridges:
	@mkdir -p dist
	CGO_ENABLED=0 go build -o dist/openparallax-shield-bridge ./cmd/shield-bridge
	CGO_ENABLED=0 go build -o dist/openparallax-audit-bridge ./cmd/audit-bridge
	CGO_ENABLED=0 go build -o dist/openparallax-memory-bridge ./cmd/memory-bridge
	CGO_ENABLED=0 go build -o dist/openparallax-sandbox-bridge ./cmd/sandbox-bridge
	CGO_ENABLED=0 go build -o dist/openparallax-channels-bridge ./cmd/channels-bridge

build-all: proto build-web build build-shield build-bridges

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

test-e2e:
	E2E_LLM=mock go test -tags e2e -v -timeout 300s ./e2e/...

test-e2e-mock:
	E2E_LLM=mock go test -tags e2e -v -timeout 300s ./e2e/...

test-e2e-ollama:
	E2E_LLM=ollama go test -tags e2e -v -timeout 300s ./e2e/...

clean:
	rm -rf dist/
