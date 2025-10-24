package main

import (
	"foonly.dev/foondot/cmd/foondot"
	"foonly.dev/foondot/internal/config"
)

var version = "undefined"

func main() {
	config.Version = version
	foondot.Execute()
}
