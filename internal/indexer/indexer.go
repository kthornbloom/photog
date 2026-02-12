package indexer

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"photog/internal/database"
	"photog/internal/models"
)

// Supported file extensions
var (
	imageExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
		".heic": true, ".heif": true, ".avif": true,
	}
	videoExts = map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".webm": true, ".m4v": true, ".3gp": true, ".wmv": true,
	}
)

// shouldSkipFile returns true for files that should never be indexed:
// hidden/dot files, temporary sync files (.pending-*, .trashed-*, etc.),
// macOS resource forks, and thumbnail database files.
func shouldSkipFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if lower == "thumbs.db" || lower == "desktop.ini" || lower == ".ds_store" {
		return true
	}
	return false
}

// Indexer scans photo directories and populates the database.
type Indexer struct {
	db       *database.DB
	paths    []string
	mu       sync.Mutex
	running  bool
	Progress IndexProgress
}

// IndexProgress tracks the current indexing state.
type IndexProgress struct {
	Running    bool   `json:"running"`
	Total      int64  `json:"total"`
	Processed  int64  `json:"processed"`
	Skipped    int64  `json:"skipped"`
	Errors     int64  `json:"errors"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	FilesPerSec float64 `json:"files_per_sec"`
}

// New creates a new Indexer.
func New(db *database.DB, paths []string) *Indexer {
	return &Indexer{
		db:    db,
		paths: paths,
	}
}

// GetProgress returns the current indexing progress.
func (idx *Indexer) GetProgress() IndexProgress {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.Progress
}

// IsRunning returns whether indexing is in progress.
func (idx *Indexer) IsRunning() bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.running
}

// Scan walks all configured paths and indexes media files.
func (idx *Indexer) Scan() error {
	idx.mu.Lock()
	if idx.running {
		idx.mu.Unlock()
		return fmt.Errorf("indexing already in progress")
	}
	idx.running = true
	idx.Progress = IndexProgress{
		Running:   true,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.running = false
		idx.Progress.Running = false
		idx.Progress.FinishedAt = time.Now().Format(time.RFC3339)
		elapsed := time.Since(parseTime(idx.Progress.StartedAt)).Seconds()
		if elapsed > 0 {
			idx.Progress.FilesPerSec = float64(idx.Progress.Processed) / elapsed
		}
		idx.mu.Unlock()
	}()

	// First pass: count files
	var totalFiles int64
	for _, root := range idx.paths {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if shouldSkipFile(d.Name()) {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if imageExts[ext] || videoExts[ext] {
				totalFiles++
			}
			return nil
		})
	}

	atomic.StoreInt64(&idx.Progress.Total, totalFiles)
	log.Printf("Indexer: found %d media files to process", totalFiles)

	// Second pass: index files
	for _, root := range idx.paths {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors, keep going
			}
			if d.IsDir() {
				return nil
			}

			if shouldSkipFile(d.Name()) {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			isImage := imageExts[ext]
			isVideo := videoExts[ext]

			if !isImage && !isVideo {
				return nil
			}

			// Check if already indexed
			exists, err := idx.db.PhotoExists(path)
			if err != nil {
				atomic.AddInt64(&idx.Progress.Errors, 1)
				return nil
			}
			if exists {
				atomic.AddInt64(&idx.Progress.Skipped, 1)
				atomic.AddInt64(&idx.Progress.Processed, 1)
				return nil
			}

			photo := idx.processFile(path, d, isImage)
			if photo != nil {
				if err := idx.db.UpsertPhoto(photo); err != nil {
					log.Printf("Indexer: error upserting %s: %v", path, err)
					atomic.AddInt64(&idx.Progress.Errors, 1)
				}
			}

			atomic.AddInt64(&idx.Progress.Processed, 1)
			return nil
		}); err != nil {
			log.Printf("Indexer: walk error for %s: %v", root, err)
		}
	}

	log.Printf("Indexer: complete. Processed %d, skipped %d, errors %d",
		idx.Progress.Processed, idx.Progress.Skipped, idx.Progress.Errors)

	return nil
}

func (idx *Indexer) processFile(path string, d fs.DirEntry, isImage bool) *models.Photo {
	info, err := d.Info()
	if err != nil {
		atomic.AddInt64(&idx.Progress.Errors, 1)
		return nil
	}

	photo := &models.Photo{
		Path:      path,
		Filename:  d.Name(),
		FileSize:  info.Size(),
		IndexedAt: time.Now(),
		TakenAt:   info.ModTime(), // fallback to file modification time
	}

	if isImage {
		photo.MediaType = "image"
		idx.extractExif(photo)
	} else {
		photo.MediaType = "video"
		// Video date falls back to file modification time
	}

	return photo
}

func (idx *Indexer) extractExif(photo *models.Photo) {
	f, err := os.Open(photo.Path)
	if err != nil {
		return
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return // No EXIF data, use file mod time
	}

	// Extract date taken
	if dt, err := x.DateTime(); err == nil {
		photo.TakenAt = dt
	}

	// Extract dimensions
	if w, err := x.Get(exif.PixelXDimension); err == nil {
		if val, err := w.Int(0); err == nil {
			photo.Width = val
		}
	}
	if h, err := x.Get(exif.PixelYDimension); err == nil {
		if val, err := h.Int(0); err == nil {
			photo.Height = val
		}
	}

	// Extract orientation
	if o, err := x.Get(exif.Orientation); err == nil {
		if val, err := o.Int(0); err == nil {
			photo.Orientation = val
		}
	}
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
