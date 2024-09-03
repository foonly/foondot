dotsync: dotsync.go
	go build -ldflags "-s -w"

clean:
	go clean

install:
	mv dotsync ~/bin