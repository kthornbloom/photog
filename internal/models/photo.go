package models

import "time"

// Photo represents a single photo or video in the library.
type Photo struct {
	ID          int64     `json:"id"`
	Path        string    `json:"path"`
	Filename    string    `json:"filename"`
	TakenAt     time.Time `json:"taken_at"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Orientation int       `json:"orientation"`
	MediaType   string    `json:"type"` // "image" or "video"
	FileSize    int64     `json:"file_size"`
	Duration    float64   `json:"duration,omitempty"` // video duration in seconds
	ThumbPath   string    `json:"thumb_path,omitempty"`
	IndexedAt   time.Time `json:"indexed_at"`
}

// TimelineGroup represents a group of photos for a date period.
type TimelineGroup struct {
	Date   string   `json:"date"`   // "2024-01" or "2024-01-15"
	Label  string   `json:"label"`  // "January 2024"
	Count  int      `json:"count"`
	Photos []*Photo `json:"photos"`
}

// TimelineResponse is the API response for the timeline endpoint.
type TimelineResponse struct {
	Groups     []*TimelineGroup `json:"groups"`
	TotalCount int              `json:"total_count"`
	HasMore    bool             `json:"has_more"`
}

// StatsResponse returns library statistics.
type StatsResponse struct {
	TotalPhotos int   `json:"total_photos"`
	TotalVideos int   `json:"total_videos"`
	TotalSize   int64 `json:"total_size"`
	OldestDate  string `json:"oldest_date"`
	NewestDate  string `json:"newest_date"`
}
