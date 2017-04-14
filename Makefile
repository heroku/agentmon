LINKER_VERSION_SYMBOL := github.com/heroku/agentmon.VERSION
LINKER_VERSION := $(shell git describe --tags --always)

build:
	go build -a -ldflags "-X ${LINKER_VERSION_SYMBOL}=${LINKER_VERSION}" ./cmd/agentmon

install:
	go install -a -ldflags "-X ${LINKER_VERSION_SYMBOL} ${LINKER_VERSION}" ./...

release:
	{\
		export GOOS=linux; \
		export GOARCH=amd64; \
		go build -ldflags "-X $(LINKER_VERSION_SYMBOL)=$(LINKER_VERSION)" -o agentmon ./cmd/agentmon; \
		tar czf "agentmon-$(LINKER_VERSION)-$$(echo $$GOOS-$$GOARCH | sed 's;/;-;').tar.gz" agentmon; \
		rm -rf agentmon; \
	}

test:
	CGO_ENABLED=1 go test -v -race ./...

bench:
	go test -v -bench . ./...
