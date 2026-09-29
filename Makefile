BIN := foondot
DESTDIR :=
PREFIX := /usr/local
# Override when building a release, e.g. `make build VERSION=v1.0.0`.
VERSION := $(shell git describe --tags --always --long --dirty)

foondot: main.go
	CGO_ENABLED=0 go build -v -trimpath -ldflags="-X main.version=${VERSION}"

.PHONY: clean
clean:
	go clean

.PHONY: install
install:
	install -Dm755 ${BIN} $(DESTDIR)$(PREFIX)/bin/${BIN}

.PHONY: build
build: clean foondot

.PHONY: test
test:
	go vet ./...
	go test ./...
