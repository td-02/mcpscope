APP := mcpscope
PORT ?= 4444
TRANSPORT ?= stdio

.PHONY: build test run

build:
	go build ./...

test:
	go test ./...

run:
	go run . proxy --server "$(SERVER)" --port "$(PORT)" --transport "$(TRANSPORT)"
