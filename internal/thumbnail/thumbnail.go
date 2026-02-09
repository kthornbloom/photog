package thumbnail

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"photog/internal/config"
)

// thumbVersion is embedded into cache filenames. Bump this to invalidate
// all existing cached thumbnails (e.g. after fixing orientation bugs).
const thumbVersion = "v2"

// Generator handles thumbnail creation and caching.
type Generator struct {
	cacheDir string
	config   config.ThumbnailConfig
	// ffmpeg availability (cached)
	ffmpegOnce sync.Once
	ffmpegPath string
}

// Size represents a thumbnail size preset.
type Size string

const (
	Small  Size = "sm"
	Medium Size = "md"
	Large  Size = "lg"
)

// PregenProgress tracks background thumbnail pre-generation state.
type PregenProgress struct {
	Running   bool  `json:"running"`
	Total     int64 `json:"total"`
	Generated int64 `json:"generated"`
	Skipped   int64 `json:"skipped"`
	Errors    int64 `json:"errors"`
}

// New creates a thumbnail generator.
func New(cacheDir string, cfg config.ThumbnailConfig) (*Generator, error) {
	thumbDir := filepath.Join(cacheDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return nil, fmt.Errorf("create thumb dir: %w", err)
	}

	return &Generator{
		cacheDir: thumbDir,
		config:   cfg,
	}, nil
}

// GetOrCreate returns the path to a cached thumbnail, generating it if needed.
func (g *Generator) GetOrCreate(photoPath string, size Size) (string, error) {
	thumbPath := g.thumbPath(photoPath, size)

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbPath); err == nil {
		return thumbPath, nil
	}

	// Generate thumbnail
	if err := g.generate(photoPath, thumbPath, size); err != nil {
		return "", fmt.Errorf("generate thumbnail: %w", err)
	}

	return thumbPath, nil
}

// GetOrCreateVideo returns the path to a cached video thumbnail, generating it if needed.
// Uses ffmpeg to extract a frame from the video.
func (g *Generator) GetOrCreateVideo(videoPath string, size Size) (string, error) {
	thumbPath := g.thumbPath(videoPath, size)

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbPath); err == nil {
		return thumbPath, nil
	}

	// Check for ffmpeg
	ffmpeg := g.getFFmpeg()
	if ffmpeg == "" {
		return "", fmt.Errorf("ffmpeg not available")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return "", err
	}

	// Extract a frame at 1 second (or start if shorter) as a temporary JPEG
	tmpJpg := thumbPath + ".tmp.jpg"
	defer os.Remove(tmpJpg)

	maxDim := g.maxDimension(size)
	scaleFilter := fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", maxDim, maxDim)

	cmd := exec.Command(ffmpeg,
		"-i", videoPath,
		"-ss", "1",        // seek to 1 second
		"-frames:v", "1",  // extract single frame
		"-vf", scaleFilter,
		"-y",              // overwrite
		tmpJpg,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Retry at 0 seconds (video might be < 1 second)
		cmd2 := exec.Command(ffmpeg,
			"-i", videoPath,
			"-frames:v", "1",
			"-vf", scaleFilter,
			"-y",
			tmpJpg,
		)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("ffmpeg error: %v: %s / %s", err, string(out), string(out2))
		}
	}

	// Now open the extracted JPEG and convert to WebP thumbnail
	src, err := openImage(tmpJpg)
	if err != nil {
		return "", fmt.Errorf("open extracted frame: %w", err)
	}

	thumb := imaging.Fit(src, maxDim, maxDim, imaging.Lanczos)

	out, err := os.Create(thumbPath)
	if err != nil {
		return "", fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if err := webp.Encode(out, thumb, &webp.Options{Quality: float32(g.config.Quality)}); err != nil {
		os.Remove(thumbPath)
		return "", fmt.Errorf("encode webp: %w", err)
	}

	return thumbPath, nil
}

// HasFFmpeg returns whether ffmpeg is available for video thumbnails.
func (g *Generator) HasFFmpeg() bool {
	return g.getFFmpeg() != ""
}

func (g *Generator) getFFmpeg() string {
	g.ffmpegOnce.Do(func() {
		path, err := exec.LookPath("ffmpeg")
		if err == nil {
			g.ffmpegPath = path
			log.Printf("Thumbnail: ffmpeg found at %s (video thumbnails enabled)", path)
		} else {
			log.Printf("Thumbnail: ffmpeg not found (video thumbnails disabled)")
		}
	})
	return g.ffmpegPath
}

// Exists checks if a thumbnail already exists in the cache.
func (g *Generator) Exists(photoPath string, size Size) bool {
	thumbPath := g.thumbPath(photoPath, size)
	_, err := os.Stat(thumbPath)
	return err == nil
}

// ThumbPath returns the expected cache path for a thumbnail (without generating).
func (g *Generator) ThumbPath(photoPath string, size Size) string {
	return g.thumbPath(photoPath, size)
}

func (g *Generator) thumbPath(photoPath string, size Size) string {
	hash := sha256.Sum256([]byte(photoPath))
	hashStr := fmt.Sprintf("%x", hash[:16]) // 32 char hex
	// Organize into subdirectories for filesystem performance.
	// thumbVersion is included so bumping it invalidates old caches.
	return filepath.Join(g.cacheDir, hashStr[:2], hashStr[2:4], fmt.Sprintf("%s_%s_%s.webp", hashStr, size, thumbVersion))
}

func (g *Generator) maxDimension(size Size) int {
	switch size {
	case Small:
		return g.config.SmallSize
	case Medium:
		return g.config.MediumSize
	case Large:
		return g.config.LargeSize
	default:
		return g.config.MediumSize
	}
}

func (g *Generator) generate(srcPath, dstPath string, size Size) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	// Open and decode source image with auto-orientation (handles EXIF rotation)
	src, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		// Fallback to manual decode for formats imaging doesn't handle natively
		src, err = openImage(srcPath)
		if err != nil {
			return fmt.Errorf("open source: %w", err)
		}
	}

	maxDim := g.maxDimension(size)

	// Resize while maintaining aspect ratio (fit within maxDim x maxDim)
	thumb := imaging.Fit(src, maxDim, maxDim, imaging.Lanczos)

	// Encode as WebP
	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if err := webp.Encode(out, thumb, &webp.Options{Quality: float32(g.config.Quality)}); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("encode webp: %w", err)
	}

	return nil
}

func openImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))

	var img image.Image
	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(f)
	case ".png":
		img, err = png.Decode(f)
	default:
		img, _, err = image.Decode(f)
	}
	if err != nil {
		return nil, err
	}

	// Apply EXIF orientation for JPEGs (the fallback path doesn't get
	// imaging.AutoOrientation, so we handle it manually here).
	if ext == ".jpg" || ext == ".jpeg" {
		img = applyExifOrientation(path, img)
	}

	return img, nil
}

// applyExifOrientation reads the EXIF orientation tag from a JPEG file
// and returns a correctly oriented image.
func applyExifOrientation(path string, img image.Image) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return img
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return img
	}

	orient, err := x.Get(exif.Orientation)
	if err != nil {
		return img // no orientation tag — image is upright
	}

	orientVal, err := orient.Int(0)
	if err != nil {
		return img
	}

	switch orientVal {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img // orientation 1 = normal
	}
}

// PregenResult holds stats from a pre-generation run.
type PregenResult struct {
	Generated int64
	Skipped   int64
	Errors    int64
}

// PregenSmallThumbnails generates small thumbnails for all provided items in slow background batches.
// items is a slice of {Path, MediaType} pairs. It sleeps between batches to avoid resource abuse.
// The stop channel can be closed to abort early.
func (g *Generator) PregenSmallThumbnails(items []PregenItem, batchSize int, batchDelay time.Duration, stop <-chan struct{}, progress *atomic.Int64) PregenResult {
	var result PregenResult

	for i := 0; i < len(items); i += batchSize {
		// Check for stop signal
		select {
		case <-stop:
			return result
		default:
		}

		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		for _, item := range batch {
			// Check if already cached
			if g.Exists(item.Path, Small) {
				result.Skipped++
				if progress != nil {
					progress.Add(1)
				}
				continue
			}

			var err error
			if item.MediaType == "video" {
				if g.HasFFmpeg() {
					_, err = g.GetOrCreateVideo(item.Path, Small)
				} else {
					result.Skipped++
					if progress != nil {
						progress.Add(1)
					}
					continue
				}
			} else {
				_, err = g.GetOrCreate(item.Path, Small)
			}

			if err != nil {
				result.Errors++
				log.Printf("Pregen: error generating thumb for %s: %v", item.Path, err)
			} else {
				result.Generated++
			}
			if progress != nil {
				progress.Add(1)
			}
		}

		// Sleep between batches to avoid resource abuse
		if end < len(items) {
			select {
			case <-stop:
				return result
			case <-time.After(batchDelay):
			}
		}
	}

	return result
}

// PregenItem represents a media file for pre-generation.
type PregenItem struct {
	Path      string
	MediaType string
}
