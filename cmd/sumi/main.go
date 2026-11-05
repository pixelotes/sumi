// Command sumi is a minimal, filesystem-first EPUB reader server.
//
// Usage:
//
//	sumi -config config/config.yml
package main

import (
	"flag"
	"log"

	"sumi/internal/config"
	"sumi/internal/server"
)

func main() {
	configPath := flag.String("config", "config/config.yml", "path to the YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config %s: %v", *configPath, err)
	}
	for _, l := range cfg.Libraries {
		log.Printf("library %q at %s", l.FriendlyName, l.Path)
	}

	// Libraries are read from disk on each request; nothing to index up front.
	log.Fatal(server.New(cfg).Start())
}
