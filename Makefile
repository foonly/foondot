foondot: foondot.go
	go build

clean:
	go clean

install: foondot
	mv foondot ~/.local/bin
