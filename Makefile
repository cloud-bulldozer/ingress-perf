GIT_COMMIT = $(shell git rev-parse HEAD)

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ifeq ($(BRANCH),)
	VERSION := $(shell git describe --tags --abbrev=0)
else
	VERSION := $(subst main,latest,$(BRANCH))
endif

SOURCES := $(shell find . -type f -name "*.go")
BUILD_DATE = $(shell date '+%Y-%m-%d-%H:%M:%S')
INGRESS_PERF_VERSION= github.com/cloud-bulldozer/ingress-perf/pkg/version

BIN_DIR = bin
BIN_NAME = ingress-perf
BIN_PATH = $(BIN_DIR)/$(BIN_NAME)
SOURCES = $(shell find . -type f -name "*.go")
CGO = 0

.PHONY: build lint clean

all: lint build

build: $(BIN_PATH)

$(BIN_PATH): $(SOURCES)
	GOARCH=$(go env GOARCH) CGO_ENABLED=$(CGO) go build -v -ldflags "-X $(INGRESS_PERF_VERSION).GitCommit=$(GIT_COMMIT) -X $(INGRESS_PERF_VERSION).Version=$(VERSION)" -o $(BIN_PATH) cmd/ingress-perf.go

clean:
	rm -Rf $(BIN_DIR)

lint:
	golangci-lint run

