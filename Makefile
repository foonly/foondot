BIN := foondot
DESTDIR :=
PREFIX := /usr/local
VERSION := $(shell git describe --tags --always --long --dirty)

foondot: main.go
	go build -v -ldflags="-X main.version=${VERSION}"

.PHONY: clean
clean:
	go clean

.PHONY: install
install:
	install -Dm755 ${BIN} $(DESTDIR)$(PREFIX)/bin/${BIN}
