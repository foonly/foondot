BIN := foondot
DESTDIR :=
PREFIX := /usr/local

foondot: foondot.go
	go build

.PHONY: clean
clean:
	go clean

.PHONY: install
install:
	install -Dm755 ${BIN} $(DESTDIR)$(PREFIX)/bin/${BIN}
