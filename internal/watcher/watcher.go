package watcher

import (
	"log"
	"time"

	"photog/internal/database"
	"photog/internal/indexer"
)

// Watcher periodically scans for new/deleted files.
type Watcher struct {
	indexer  *indexer.Indexer
	db       *database.DB
	interval time.Duration
	stop     chan struct{}
}

// New creates a file watcher that triggers periodic scans.
func New(idx *indexer.Indexer, db *database.DB, interval time.Duration) *Watcher {
	return &Watcher{
		indexer:  idx,
		db:       db,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins the periodic scan loop. It runs the first scan after one full interval.
func (w *Watcher) Start() {
	go w.loop()
}

// Stop signals the watcher to stop.
func (w *Watcher) Stop() {
	close(w.stop)
}

func (w *Watcher) loop() {
	log.Printf("Watcher: periodic scan every %s", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			log.Println("Watcher: stopped")
			return
		case <-ticker.C:
			w.runScan()
		}
	}
}

func (w *Watcher) runScan() {
	if w.indexer.IsRunning() {
		log.Println("Watcher: skipping scan, indexer already running")
		return
	}

	log.Println("Watcher: starting periodic scan for new/deleted files...")

	// Scan for new files
	if err := w.indexer.Scan(); err != nil {
		log.Printf("Watcher: scan error: %v", err)
	}

	// Remove deleted files from the database
	removed, err := w.db.RemoveMissing()
	if err != nil {
		log.Printf("Watcher: error removing missing files: %v", err)
	} else if removed > 0 {
		log.Printf("Watcher: removed %d files that no longer exist on disk", removed)
	}

	log.Println("Watcher: periodic scan complete")
}
