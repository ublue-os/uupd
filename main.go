package main

import (
	"github.com/ublue-os/uupd/cmd"
	"log"
	"os/user"
)

func main() {
	log.SetFlags(0)
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("Error fetching current user: %v", err)
	}
	if currentUser.Uid != "0" {
		log.Fatalf("uupd needs to be invoked as root.")
	}
	cmd.Execute()
}
