package main

import (
	"log"

	"github.com/federicoserini/mobile-db/server"
)

func main() {
	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	_ = cfg
	log.Println("mobile-db starting — config OK")
}
