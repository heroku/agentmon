LINKER_VERSION_SYMBOL := github.com/heroku/agentmon.VERSION
LINKER_VERSION := $(shell git describe --tags --always)

build:
	go build -a -ldflags "-X ${LINKER_VERSION_SYMBOL}=${LINKER_VERSION}" ./cmd/agentmon

install:
	go install -a -ldflags "-X ${LINKER_VERSION_SYMBOL} ${LINKER_VERSION}" ./...
