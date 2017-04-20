LINKER_VERSION_SYMBOL := github.com/heroku/agentmon.VERSION
LINKER_VERSION := $(shell git describe --tags --always)

build:
	go build -a -ldflags "-X ${LINKER_VERSION_SYMBOL}=${LINKER_VERSION}" ./cmd/agentmon

install:
	go install -a -ldflags "-X ${LINKER_VERSION_SYMBOL} ${LINKER_VERSION}" ./...

release: GOOS := linux
release: GOARCH := amd64
release: PATH := /usr/local/opt/gnu-tar/libexec/gnubin:$(PATH)
release:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-X $(LINKER_VERSION_SYMBOL)=$(LINKER_VERSION)" -o agentmon ./cmd/agentmon
	tar czf "agentmon-$(LINKER_VERSION)-$(GOOS)-$(GOARCH).tar.gz" agentmon
	rm agentmon

test:
	CGO_ENABLED=1 go test -v -race ./...

bench:
	go test -v -bench . ./...
