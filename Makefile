BIN_DIR = bin
BIN_NAME = ingress-perf
BIN_PATH = $(BIN_DIR)/$(BIN_NAME)
SOURCES = $(shell find . -type f -name "*.go")

.PHONY: build lint clean

all: lint build

build: $(BIN_PATH)

$(BIN_PATH): $(SOURCES)
	CGO_ENABLED=0 go build -o $(BIN_PATH) cmd/ingress-perf.go

clean:
	rm -Rf $(BIN_DIR)

lint:
	golangci-lint run

