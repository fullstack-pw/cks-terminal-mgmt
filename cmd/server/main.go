package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fullstack-pw/cks-terminal-mgmt/internal/config"
	"github.com/fullstack-pw/cks-terminal-mgmt/internal/server"
)

func main() {
	cfg := config.Load()

	srv := server.New(cfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		srv.Shutdown()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
