package main

import (
	"github.com/ublue-os/uupd/cmd"
	"log"
)

func main() {
	log.SetFlags(0)
	cmd.Execute()
}
