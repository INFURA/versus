# Protips:
# - RUN can be overridden for debugging, like:
#   $ RUN="dlv debug --" make -e

VERSION := $(shell git describe --tags --dirty --always 2> /dev/null || echo "dev")
LDFLAGS = "-X main.Version=$(VERSION) -w -s"
SOURCES = $(shell find . -type f -name '*.go')

BINARY = $(notdir $(PWD))
RUN = ./$(BINARY)

all: $(BINARY)

$(BINARY): $(SOURCES)
	GO111MODULE=on go build -ldflags $(LDFLAGS) -o "$@"

deps:
	GO111MODULE=on go get -d

build: $(BINARY)

clean:
	rm $(BINARY)

run: $(BINARY)
	$(RUN) --help

test:
	go test -vet "all" -timeout 5s -race ./...
