LINKER_VERSION_SYMBOL := github.com/heroku/agentmon.VERSION
LINKER_VERSION := $(shell git describe --tags --always)
ARCHES := linux/amd64

build:
	go build -a -ldflags "-X ${LINKER_VERSION_SYMBOL}=${LINKER_VERSION}" ./cmd/agentmon

install:
	go install -a -ldflags "-X ${LINKER_VERSION_SYMBOL} ${LINKER_VERSION}" ./...

release: gox
	{\
		TMP=$$(mktemp -d -t agentmon.XXXXX); \
		for arch in $(ARCHES); do \
			gox -osarch="$$arch" -output="$$TMP/{{.OS}}/{{.Arch}}/{{.Dir}}" -ldflags "-X $(LINKER_VERSION_SYMBOL)=$(LINKER_VERSION)" ./...; \
			tar -C $$TMP/$$arch -czf "agentmon-$(LINKER_VERSION)-$$(echo $$arch | sed 's;/;-;').tar.gz" .; \
		done; \
		rm -rf $$TMP; \
	}

test:
	CGO_ENABLED=1 go test -v -race ./...

bench:
	go test -v -bench . ./...

gox:
	go get -u github.com/mitchellh/gox
