package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"photog/internal/models"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// New creates or opens the SQLite database at the given cache directory.
func New(cacheDir string) (*DB, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	dbPath := filepath.Join(cacheDir, "photog.db")
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set connection pool for performance
	conn.SetMaxOpenConns(1) // SQLite works best with single writer
	conn.SetMaxIdleConns(2)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS photos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		filename TEXT NOT NULL,
		taken_at DATETIME NOT NULL,
		width INTEGER NOT NULL DEFAULT 0,
		height INTEGER NOT NULL DEFAULT 0,
		orientation INTEGER NOT NULL DEFAULT 1,
		media_type TEXT NOT NULL DEFAULT 'image',
		file_size INTEGER NOT NULL DEFAULT 0,
		duration REAL NOT NULL DEFAULT 0,
		thumb_path TEXT NOT NULL DEFAULT '',
		indexed_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_photos_taken_at ON photos(taken_at DESC);
	CREATE INDEX IF NOT EXISTS idx_photos_path ON photos(path);
	CREATE INDEX IF NOT EXISTS idx_photos_media_type ON photos(media_type);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// UpsertPhoto inserts or updates a photo record.
func (db *DB) UpsertPhoto(p *models.Photo) error {
	_, err := db.conn.Exec(`
		INSERT INTO photos (path, filename, taken_at, width, height, orientation, media_type, file_size, duration, thumb_path, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			filename=excluded.filename,
			taken_at=excluded.taken_at,
			width=excluded.width,
			height=excluded.height,
			orientation=excluded.orientation,
			media_type=excluded.media_type,
			file_size=excluded.file_size,
			duration=excluded.duration,
			thumb_path=excluded.thumb_path,
			indexed_at=excluded.indexed_at
	`, p.Path, p.Filename, p.TakenAt, p.Width, p.Height, p.Orientation, p.MediaType, p.FileSize, p.Duration, p.ThumbPath, p.IndexedAt)
	return err
}

// PhotoExists checks if a photo with the given path is already indexed.
func (db *DB) PhotoExists(path string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM photos WHERE path = ?", path).Scan(&count)
	return count > 0, err
}

// GetTimeline returns photos grouped by month, ordered by taken_at descending.
func (db *DB) GetTimeline(offset, limit int) (*models.TimelineResponse, error) {
	// Get total count
	var totalCount int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM photos").Scan(&totalCount); err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT id, path, filename, taken_at, width, height, orientation, media_type, file_size, duration, thumb_path, indexed_at
		FROM photos
		ORDER BY taken_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupMap := make(map[string]*models.TimelineGroup)
	var groupOrder []string

	for rows.Next() {
		p := &models.Photo{}
		if err := rows.Scan(&p.ID, &p.Path, &p.Filename, &p.TakenAt, &p.Width, &p.Height, &p.Orientation, &p.MediaType, &p.FileSize, &p.Duration, &p.ThumbPath, &p.IndexedAt); err != nil {
			log.Printf("scan error: %v", err)
			continue
		}

		key := p.TakenAt.Format("2006-01")
		label := p.TakenAt.Format("January 2006")

		if _, ok := groupMap[key]; !ok {
			groupMap[key] = &models.TimelineGroup{
				Date:   key,
				Label:  label,
				Photos: make([]*models.Photo, 0),
			}
			groupOrder = append(groupOrder, key)
		}
		groupMap[key].Photos = append(groupMap[key].Photos, p)
		groupMap[key].Count++
	}

	groups := make([]*models.TimelineGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		groups = append(groups, groupMap[key])
	}

	return &models.TimelineResponse{
		Groups:     groups,
		TotalCount: totalCount,
		HasMore:    offset+limit < totalCount,
	}, nil
}

// GetPhoto returns a single photo by ID.
func (db *DB) GetPhoto(id int64) (*models.Photo, error) {
	p := &models.Photo{}
	err := db.conn.QueryRow(`
		SELECT id, path, filename, taken_at, width, height, orientation, media_type, file_size, duration, thumb_path, indexed_at
		FROM photos WHERE id = ?
	`, id).Scan(&p.ID, &p.Path, &p.Filename, &p.TakenAt, &p.Width, &p.Height, &p.Orientation, &p.MediaType, &p.FileSize, &p.Duration, &p.ThumbPath, &p.IndexedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetStats returns library statistics.
func (db *DB) GetStats() (*models.StatsResponse, error) {
	stats := &models.StatsResponse{}

	db.conn.QueryRow("SELECT COUNT(*) FROM photos WHERE media_type = 'image'").Scan(&stats.TotalPhotos)
	db.conn.QueryRow("SELECT COUNT(*) FROM photos WHERE media_type = 'video'").Scan(&stats.TotalVideos)
	db.conn.QueryRow("SELECT COALESCE(SUM(file_size), 0) FROM photos").Scan(&stats.TotalSize)
	db.conn.QueryRow("SELECT COALESCE(MIN(taken_at), '') FROM photos").Scan(&stats.OldestDate)
	db.conn.QueryRow("SELECT COALESCE(MAX(taken_at), '') FROM photos").Scan(&stats.NewestDate)

	return stats, nil
}

// RemoveMissing deletes photos from the database whose files no longer exist.
func (db *DB) RemoveMissing() (int64, error) {
	rows, err := db.conn.Query("SELECT id, path FROM photos")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []int64
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			toDelete = append(toDelete, id)
		}
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("DELETE FROM photos WHERE id = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var count int64
	for _, id := range toDelete {
		if _, err := stmt.Exec(id); err == nil {
			count++
		}
	}

	return count, tx.Commit()
}

// SearchByDateRange returns photos within a date range.
func (db *DB) SearchByDateRange(start, end time.Time, offset, limit int) ([]*models.Photo, int, error) {
	var total int
	db.conn.QueryRow("SELECT COUNT(*) FROM photos WHERE taken_at BETWEEN ? AND ?", start, end).Scan(&total)

	rows, err := db.conn.Query(`
		SELECT id, path, filename, taken_at, width, height, orientation, media_type, file_size, duration, thumb_path, indexed_at
		FROM photos WHERE taken_at BETWEEN ? AND ?
		ORDER BY taken_at DESC
		LIMIT ? OFFSET ?
	`, start, end, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var photos []*models.Photo
	for rows.Next() {
		p := &models.Photo{}
		if err := rows.Scan(&p.ID, &p.Path, &p.Filename, &p.TakenAt, &p.Width, &p.Height, &p.Orientation, &p.MediaType, &p.FileSize, &p.Duration, &p.ThumbPath, &p.IndexedAt); err != nil {
			continue
		}
		photos = append(photos, p)
	}

	return photos, total, nil
}

// GetMonthBuckets returns per-month counts ordered by date descending, with cumulative offsets.
// This is a lightweight query used by the scrubber to know the full date range and jump to any month.
func (db *DB) GetMonthBuckets() ([]*models.MonthBucket, error) {
	rows, err := db.conn.Query(`
		SELECT strftime('%Y-%m', taken_at) AS month, COUNT(*) AS cnt
		FROM photos
		GROUP BY month
		ORDER BY month DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []*models.MonthBucket
	cumulative := 0
	for rows.Next() {
		var month string
		var count int
		if err := rows.Scan(&month, &count); err != nil {
			continue
		}
		// Parse the month string to generate a label
		t, _ := time.Parse("2006-01", month)
		label := t.Format("January 2006")

		buckets = append(buckets, &models.MonthBucket{
			Month:            month,
			Label:            label,
			Count:            count,
			CumulativeOffset: cumulative,
		})
		cumulative += count
	}
	return buckets, nil
}

// GetAllPaths returns all photo/video paths and media types for thumbnail pre-generation.
func (db *DB) GetAllPaths() ([]struct{ Path, MediaType string }, error) {
	rows, err := db.conn.Query("SELECT path, media_type FROM photos ORDER BY taken_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []struct{ Path, MediaType string }
	for rows.Next() {
		var item struct{ Path, MediaType string }
		if err := rows.Scan(&item.Path, &item.MediaType); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// GetMemories returns random photos from the past at 5-year intervals
// (e.g. 5, 10, 15, 20 years ago). Each interval contributes at most one photo.
// Returns up to maxCount photos, ordered oldest first.
func (db *DB) GetMemories(maxCount int) ([]*models.Photo, error) {
	if maxCount <= 0 {
		maxCount = 5
	}

	now := time.Now()
	currentYear := now.Year()
	var memories []*models.Photo

	for i := 1; i <= maxCount; i++ {
		targetYear := currentYear - (i * 5)
		if targetYear < 1900 {
			break
		}

		start := time.Date(targetYear, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(targetYear, 12, 31, 23, 59, 59, 999999999, time.UTC)

		row := db.conn.QueryRow(`
			SELECT id, path, filename, taken_at, width, height, orientation, media_type, file_size, duration, thumb_path, indexed_at
			FROM photos
			WHERE taken_at BETWEEN ? AND ?
			ORDER BY RANDOM()
			LIMIT 1
		`, start, end)

		p := &models.Photo{}
		if err := row.Scan(&p.ID, &p.Path, &p.Filename, &p.TakenAt, &p.Width, &p.Height, &p.Orientation, &p.MediaType, &p.FileSize, &p.Duration, &p.ThumbPath, &p.IndexedAt); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		memories = append(memories, p)
	}

	return memories, nil
}

// RemoveDotfiles deletes indexed entries whose filename starts with a dot
// (hidden files, .pending-* sync temp files, etc.). These should never have
// been indexed and will never produce valid thumbnails.
func (db *DB) RemoveDotfiles() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM photos WHERE filename LIKE '.%'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}
