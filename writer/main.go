package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	if err := serve(); err != nil {
		log.WithError(err).Fatal("Error servicing write operations")
	}
}
