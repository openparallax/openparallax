.PHONY: proto build build-web build-shield build-all test lint clean

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

build-all: proto build-web build build-shield

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf dist/
