CONTAINER_BUILD ?= podman build --force-rm
CONTAINER_NS ?= quay.io/cloud-bulldozer
GIT_COMMIT = $(shell git rev-parse HEAD)

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ifeq ($(BRANCH),HEAD)
	VERSION := $(shell git describe --tags --abbrev=0)
else
	VERSION := $(BRANCH)
endif

BUILD_DATE = $(shell date '+%Y-%m-%d-%H:%M:%S')
INGRESS_PERF_VERSION= github.com/cloud-bulldozer/go-commons/version

BIN_DIR = bin
BIN_NAME = ingress-perf
BIN_PATH = $(BIN_DIR)/$(BIN_NAME)
SOURCES = $(shell find . -type f -name "*.go")
CGO = 0

.PHONY: build lint clean

all: lint build container-build

build: $(BIN_PATH)

$(BIN_PATH): $(SOURCES)
	GOARCH=$(shell go env GOARCH) CGO_ENABLED=$(CGO) go build -v -ldflags "-X $(INGRESS_PERF_VERSION).GitCommit=$(GIT_COMMIT) -X $(INGRESS_PERF_VERSION).Version=$(VERSION) -X $(INGRESS_PERF_VERSION).BuildDate=$(BUILD_DATE)" -o $(BIN_PATH) cmd/ingress-perf.go

container-build: build
	@echo "Building the container image"
	$(CONTAINER_BUILD) -f containers/Containerfile \
	-t $(CONTAINER_NS)/$(BIN_NAME) ./containers

clean:
	rm -Rf $(BIN_DIR)

lint:
	golangci-lint run

