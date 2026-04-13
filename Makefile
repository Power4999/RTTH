
GO      := go
GOFLAGS := -count=1 -timeout=120s
RACE    := -race

.PHONY: all lint vet test test-unit test-integration test-race coverage \
        build build-all clean help

all: lint test build

vet:
	$(GO) vet ./...

lint: vet
	@which staticcheck > /dev/null 2>&1 \
		&& staticcheck ./... \
		|| echo "staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest"


test-unit:
	$(GO) test $(GOFLAGS) ./internal/...

test-race:
	$(GO) test $(RACE) $(GOFLAGS) ./internal/...

test-integration:
	$(GO) test $(RACE) $(GOFLAGS) -v ./test/integration/...

test: test-race test-integration


coverage:
	$(GO) test $(RACE) $(GOFLAGS) \
		-coverprofile=coverage.out \
		-covermode=atomic \
		./internal/...
	$(GO) tool cover -func=coverage.out | tail -1
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

coverage-func:
	$(GO) test $(GOFLAGS) \
		-coverprofile=coverage.out \
		-covermode=atomic \
		./internal/...
	$(GO) tool cover -func=coverage.out


build:
	$(GO) build -o bin/server .

build-all:
	$(GO) build -o bin/server       .
	$(GO) build -o bin/channel      ./cmd/channel/
	$(GO) build -o bin/client       ./cmd/client/


run-channel:
	$(GO) run ./cmd/channel/

run-cluster: build-all
	@echo "Starting 3-node cluster..."
	@mkdir -p data/node1 data/node2 data/node3
	./bin/server 1 8081 300 ./data/node1 &
	./bin/server 2 8082 300 ./data/node2 &
	./bin/server 3 8083 300 ./data/node3 &
	@echo "Nodes started. Use 'make stop-cluster' to stop."

stop-cluster:
	@pkill -f 'bin/server' 2>/dev/null || true
	@pkill -f 'bin/channel' 2>/dev/null || true


clean:
	rm -rf bin/ coverage.out coverage.html data/

help:
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## /  /'