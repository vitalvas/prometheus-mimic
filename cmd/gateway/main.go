package main

import (
	"flag"
	"log"

	"github.com/vitalvas/prometheus-mimic/internal/gateway"
)

func main() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)

	configPath := flag.String("config", "config.yaml", "path to config file")

	flag.Parse()

	gateway, err := gateway.New(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := gateway.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
