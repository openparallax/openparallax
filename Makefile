.PHONY: proto build build-shield build-all test lint clean

PROTOC ?= protoc

proto:
	$(PROTOC) --proto_path=proto \
	       --go_out=. --go_opt=module=github.com/openparallax/openparallax \
	       --go-grpc_out=. --go-grpc_opt=module=github.com/openparallax/openparallax \
	       proto/openparallax/v1/*.proto

build:
	@mkdir -p dist
	go build -o dist/openparallax ./cmd/agent

build-shield:
	@mkdir -p dist
	go build -o dist/openparallax-shield ./cmd/shield

build-all: proto build build-shield

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf dist/
