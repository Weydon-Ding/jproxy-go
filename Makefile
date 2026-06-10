APP := jproxy
PKG := ./cmd/jproxy
BIN_DIR := bin

.PHONY: build test lint run tidy clean

build:
	go build -o $(BIN_DIR)/$(APP) $(PKG)

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run $(PKG)

tidy:
	go mod tidy

clean:
	@if exist $(BIN_DIR) rmdir /S /Q $(BIN_DIR)
