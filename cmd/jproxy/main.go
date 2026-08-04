package main

import (
	"log"
	"net/http"

	"jproxy-go/internal/config"
	"jproxy-go/internal/proxy"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	srv := proxy.NewServer(cfg)
	log.Printf("jproxy-go listening on %s", cfg.Addr)
	log.Printf("jackett=%s prowlarr=%s", cfg.JackettURL, cfg.ProwlarrURL)
	if err := http.ListenAndServe(cfg.Addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
