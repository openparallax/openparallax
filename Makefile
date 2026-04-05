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
	go build -o dist/openparallax ./cmd/agent

build-shield:
	@mkdir -p dist
	go build -o dist/openparallax-shield ./cmd/shield

build-bridges:
	@mkdir -p dist
	go build -o dist/openparallax-shield-bridge ./cmd/shield-bridge
	go build -o dist/openparallax-audit-bridge ./cmd/audit-bridge
	go build -o dist/openparallax-memory-bridge ./cmd/memory-bridge
	go build -o dist/openparallax-sandbox-bridge ./cmd/sandbox-bridge
	go build -o dist/openparallax-channels-bridge ./cmd/channels-bridge

build-all: proto build-web build build-shield build-bridges

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf dist/
