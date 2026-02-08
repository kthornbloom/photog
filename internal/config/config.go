package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Photos    PhotosConfig    `yaml:"photos"`
	Cache     CacheConfig     `yaml:"cache"`
	Thumbnail ThumbnailConfig `yaml:"thumbnail"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type PhotosConfig struct {
	Paths []string `yaml:"paths"`
}

type CacheConfig struct {
	Dir string `yaml:"dir"`
}

type ThumbnailConfig struct {
	SmallSize  int `yaml:"small_size"`
	MediumSize int `yaml:"medium_size"`
	LargeSize  int `yaml:"large_size"`
	Quality    int `yaml:"quality"`
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Photos: PhotosConfig{
			Paths: []string{"/photos"},
		},
		Cache: CacheConfig{
			Dir: "/cache",
		},
		Thumbnail: ThumbnailConfig{
			SmallSize:  250,
			MediumSize: 600,
			LargeSize:  1200,
			Quality:    80,
		},
	}
}

// Load reads config from file, then overlays environment variables.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try loading config file
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Environment variable overrides
	if port := os.Getenv("PHOTOG_PORT"); port != "" {
		var p int
		if _, err := parseIntEnv(port, &p); err == nil {
			cfg.Server.Port = p
		}
	}

	if paths := os.Getenv("PHOTOG_PHOTO_PATHS"); paths != "" {
		cfg.Photos.Paths = strings.Split(paths, ",")
	}

	if dir := os.Getenv("PHOTOG_CACHE_DIR"); dir != "" {
		cfg.Cache.Dir = dir
	}

	return cfg, nil
}

func parseIntEnv(s string, out *int) (bool, error) {
	var v int
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		v = v*10 + int(c-'0')
	}
	*out = v
	return true, nil
}
