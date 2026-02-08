package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"photog/internal/config"
	"photog/internal/database"
	"photog/internal/indexer"
	"photog/internal/server"
	"photog/internal/thumbnail"
	"photog/internal/watcher"
)

func main() {
	configPath := flag.String("config", "", "Path to config.yaml (optional, uses defaults + env vars)")
	autoIndex := flag.Bool("auto-index", true, "Automatically start indexing on startup")
	watchInterval := flag.Duration("watch-interval", 24*time.Hour, "Interval between periodic scans for new/deleted files (0 to disable)")
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

	// Stop channel for background tasks
	pregenStop := make(chan struct{})

	// Auto-index on startup, then pre-generate thumbnails
	if *autoIndex {
		go func() {
			log.Println("Starting initial index scan...")
			if err := idx.Scan(); err != nil {
				log.Printf("Initial indexing error: %v", err)
			}

			// After indexing completes, start background thumbnail pre-generation
			startPregen(db, thumbGen, pregenStop)
		}()
	}

	// Start periodic file watcher
	var w *watcher.Watcher
	if *watchInterval > 0 {
		w = watcher.New(idx, db, *watchInterval)
		w.Start()
	}

	// Start HTTP server
	srv := server.New(cfg, db, idx, thumbGen)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		close(pregenStop)
		if w != nil {
			w.Stop()
		}
		db.Close()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// startPregen runs background thumbnail pre-generation in slow batches.
func startPregen(db *database.DB, thumbGen *thumbnail.Generator, stop <-chan struct{}) {
	items, err := db.GetAllPaths()
	if err != nil {
		log.Printf("Pregen: failed to get paths: %v", err)
		return
	}

	if len(items) == 0 {
		return
	}

	// Convert to PregenItem slice
	pregenItems := make([]thumbnail.PregenItem, len(items))
	for i, item := range items {
		pregenItems[i] = thumbnail.PregenItem{
			Path:      item.Path,
			MediaType: item.MediaType,
		}
	}

	var progress atomic.Int64

	log.Printf("Pregen: starting background thumbnail generation for %d items", len(pregenItems))

	// Process in batches of 10, with a 2-second pause between batches
	// This keeps resource usage low while steadily building the cache
	result := thumbGen.PregenSmallThumbnails(pregenItems, 10, 2*time.Second, stop, &progress)

	log.Printf("Pregen: complete. Generated %d, skipped %d (already cached), errors %d",
		result.Generated, result.Skipped, result.Errors)
}
