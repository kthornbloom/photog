package thumbnail

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"photog/internal/config"
)

// Generator handles thumbnail creation and caching.
type Generator struct {
	cacheDir string
	config   config.ThumbnailConfig
}

// Size represents a thumbnail size preset.
type Size string

const (
	Small  Size = "sm"
	Medium Size = "md"
	Large  Size = "lg"
)

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

// ThumbPath returns the expected cache path for a thumbnail (without generating).
func (g *Generator) ThumbPath(photoPath string, size Size) string {
	return g.thumbPath(photoPath, size)
}

func (g *Generator) thumbPath(photoPath string, size Size) string {
	hash := sha256.Sum256([]byte(photoPath))
	hashStr := fmt.Sprintf("%x", hash[:16]) // 32 char hex
	// Organize into subdirectories for filesystem performance
	return filepath.Join(g.cacheDir, hashStr[:2], hashStr[2:4], fmt.Sprintf("%s_%s.webp", hashStr, size))
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

	// Open and decode source image
	src, err := openImage(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
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

	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Decode(f)
	case ".png":
		return png.Decode(f)
	default:
		// Try generic decode
		img, _, err := image.Decode(f)
		return img, err
	}
}
