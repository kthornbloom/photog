package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"photog/internal/config"
	"photog/internal/database"
	"photog/internal/indexer"
	"photog/internal/server"
	"photog/internal/thumbnail"
)

func main() {
	configPath := flag.String("config", "", "Path to config.yaml (optional, uses defaults + env vars)")
	autoIndex := flag.Bool("auto-index", true, "Automatically start indexing on startup")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Photog - Self-hosted Photo Viewer")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Photo paths: %v", cfg.Photos.Paths)
	log.Printf("Cache dir: %s", cfg.Cache.Dir)

	// Initialize database
	db, err := database.New(cfg.Cache.Dir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize thumbnail generator
	thumbGen, err := thumbnail.New(cfg.Cache.Dir, cfg.Thumbnail)
	if err != nil {
		log.Fatalf("Failed to initialize thumbnail generator: %v", err)
	}

	// Initialize indexer
	idx := indexer.New(db, cfg.Photos.Paths)

	// Auto-index on startup
	if *autoIndex {
		go func() {
			log.Println("Starting initial index scan...")
			if err := idx.Scan(); err != nil {
				log.Printf("Initial indexing error: %v", err)
			}
		}()
	}

	// Start HTTP server
	srv := server.New(cfg, db, idx, thumbGen)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		db.Close()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
