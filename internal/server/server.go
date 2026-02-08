package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"photog/internal/config"
	"photog/internal/database"
	"photog/internal/indexer"
	"photog/internal/thumbnail"
)

// Server is the main HTTP server.
type Server struct {
	cfg     *config.Config
	db      *database.DB
	indexer *indexer.Indexer
	thumbs  *thumbnail.Generator
	mux     *http.ServeMux
}

// New creates a new Server.
func New(cfg *config.Config, db *database.DB, idx *indexer.Indexer, thumbs *thumbnail.Generator) *Server {
	s := &Server{
		cfg:     cfg,
		db:      db,
		indexer: idx,
		thumbs:  thumbs,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// API routes
	s.mux.HandleFunc("/api/timeline/months", s.handleTimelineMonths)
	s.mux.HandleFunc("/api/timeline", s.handleTimeline)
	s.mux.HandleFunc("/api/photo/", s.handlePhoto)
	s.mux.HandleFunc("/api/thumb/", s.handleThumb)
	s.mux.HandleFunc("/api/media/", s.handleMedia)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/index", s.handleIndex)
	s.mux.HandleFunc("/api/index/progress", s.handleIndexProgress)

	// Static file serving (embedded frontend in production)
	s.mux.HandleFunc("/", s.handleFrontend)
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, s.corsMiddleware(s.mux))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleTimeline returns paginated timeline data grouped by month.
func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	timeline, err := s.db.GetTimeline(offset, limit)
	if err != nil {
		jsonError(w, "Failed to fetch timeline", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, timeline)
}

// handleTimelineMonths returns the lightweight month-bucket list for the scrubber.
func (s *Server) handleTimelineMonths(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.db.GetMonthBuckets()
	if err != nil {
		jsonError(w, "Failed to fetch month buckets", http.StatusInternalServerError)
		return
	}
	// Cache for 5 minutes — lightweight and doesn't change often
	w.Header().Set("Cache-Control", "public, max-age=300")
	jsonResponse(w, buckets)
}

// handlePhoto returns photo metadata by ID.
func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/photo/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "Invalid photo ID", http.StatusBadRequest)
		return
	}

	photo, err := s.db.GetPhoto(id)
	if err != nil {
		jsonError(w, "Photo not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, photo)
}

// handleThumb serves or generates a thumbnail.
func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	// URL pattern: /api/thumb/{id}/{size}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/thumb/"), "/")
	if len(parts) < 1 {
		http.Error(w, "Invalid thumbnail request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid photo ID", http.StatusBadRequest)
		return
	}

	size := thumbnail.Small
	if len(parts) > 1 {
		switch parts[1] {
		case "md":
			size = thumbnail.Medium
		case "lg":
			size = thumbnail.Large
		}
	}

	photo, err := s.db.GetPhoto(id)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	var thumbPath string
	if photo.MediaType == "video" {
		// Video thumbnail via ffmpeg
		if !s.thumbs.HasFFmpeg() {
			http.Error(w, "Video thumbnails unavailable (ffmpeg not installed)", http.StatusNotImplemented)
			return
		}
		thumbPath, err = s.thumbs.GetOrCreateVideo(photo.Path, size)
	} else {
		thumbPath, err = s.thumbs.GetOrCreate(photo.Path, size)
	}
	if err != nil {
		log.Printf("Thumbnail error for %s: %v", photo.Path, err)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	// Set aggressive cache headers
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/webp")

	// Serve with ETag support
	http.ServeFile(w, r, thumbPath)
}

// handleMedia serves the original media file with range request support.
func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/media/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid photo ID", http.StatusBadRequest)
		return
	}

	photo, err := s.db.GetPhoto(id)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	// Validate file still exists
	if _, err := os.Stat(photo.Path); err != nil {
		http.Error(w, "File not found on disk", http.StatusNotFound)
		return
	}

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(photo.Path))
	contentType := mimeForExt(ext)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")

	// http.ServeFile handles Range requests, ETag, etc.
	http.ServeFile(w, r, photo.Path)
}

// handleStats returns library statistics.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		jsonError(w, "Failed to fetch stats", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, stats)
}

// handleIndex triggers a re-index.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.indexer.IsRunning() {
		jsonResponse(w, map[string]interface{}{
			"status":   "already_running",
			"progress": s.indexer.GetProgress(),
		})
		return
	}

	// Start indexing in background
	go func() {
		if err := s.indexer.Scan(); err != nil {
			log.Printf("Indexing error: %v", err)
		}
	}()

	jsonResponse(w, map[string]string{"status": "started"})
}

// handleIndexProgress returns current indexing progress.
func (s *Server) handleIndexProgress(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, s.indexer.GetProgress())
}

// handleFrontend serves the embedded frontend or proxies in dev mode.
func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	// In production, this would serve from embedded filesystem.
	// During development, Vite dev server handles this.
	// For now, serve a simple placeholder or the dist directory.
	distDir := "ui/dist"
	if _, err := os.Stat(distDir); err == nil {
		fileServer := http.FileServer(http.Dir(distDir))
		// Try serving the file, fall back to index.html for SPA routing
		path := filepath.Join(distDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// SPA fallback
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
		return
	}

	// Development mode placeholder
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, `<!DOCTYPE html>
<html><body style="background:#111;color:#fff;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center">
<h1>Photog</h1>
<p>Frontend not built yet. Run <code>cd ui && npm run dev</code> for development.</p>
<p><a href="/api/stats" style="color:#60a5fa">API Stats</a> |
<a href="/api/index/progress" style="color:#60a5fa">Index Progress</a></p>
</div></body></html>`)
}

func mimeForExt(ext string) string {
	types := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp",
		".tiff": "image/tiff", ".tif": "image/tiff",
		".heic": "image/heic", ".heif": "image/heif", ".avif": "image/avif",
		".mp4": "video/mp4", ".mov": "video/quicktime", ".avi": "video/x-msvideo",
		".mkv": "video/x-matroska", ".webm": "video/webm", ".m4v": "video/mp4",
		".3gp": "video/3gpp", ".wmv": "video/x-ms-wmv",
	}
	if ct, ok := types[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Unused but reserved for future use with time-based searches
var _ = time.Now
