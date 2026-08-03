package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// CacheExpiryDuration is how long the cache is valid (1 minute for rate limiting)
	CacheExpiryDuration = 1 * time.Minute
	// DefaultCacheFile is the default location for the export cache
	DefaultCacheFile = ".workflowy/export-cache.json"
)

// ExportCache represents the cached export data with timestamp
// Data is stored as raw JSON to avoid circular dependencies
type ExportCache struct {
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// GetCachePath returns the full path to a cache file beneath the user's home directory.
func GetCachePath(relativePath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Cannot resolve cache path %q: cannot get home directory: %w", relativePath, err)
	}
	return filepath.Join(homeDir, relativePath), nil
}

// ReadExportCache reads the cached export data if it exists and is valid
func ReadExportCache(cachePath string) (*ExportCache, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("cache file does not exist", "path", cachePath)
			return nil, nil // No cache exists, not an error
		}
		return nil, fmt.Errorf("Cannot read export cache %q: %w", cachePath, err)
	}

	var cache ExportCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("Cannot parse export cache %q: %w", cachePath, err)
	}

	slog.Debug("cache file read successfully", "path", cachePath, "timestamp", cache.Timestamp)
	return &cache, nil
}

// WriteExportCache writes the export data to cache with current timestamp
// data should be any type that can be marshaled to JSON
func WriteExportCache(cachePath string, data interface{}) error {
	// Ensure cache directory exists
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("Cannot create export cache directory %q: %w", cacheDir, err)
	}

	// Marshal the data to JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Cannot encode export cache data for %q: %w", cachePath, err)
	}

	cache := ExportCache{
		Timestamp: time.Now().Unix(),
		Data:      dataJSON,
	}

	cacheData, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("Cannot encode export cache document for %q: %w", cachePath, err)
	}

	if err := os.WriteFile(cachePath, cacheData, 0644); err != nil {
		return fmt.Errorf("Cannot write export cache %q: %w", cachePath, err)
	}

	slog.Debug("cache file written", "path", cachePath, "timestamp", cache.Timestamp)
	return nil
}

// IsCacheValid checks if the cache exists and is within the expiry duration
func IsCacheValid(cache *ExportCache) bool {
	if cache == nil {
		return false
	}

	cacheTime := time.Unix(cache.Timestamp, 0)
	age := time.Since(cacheTime)

	valid := age < CacheExpiryDuration
	slog.Debug("cache validity check", "age_seconds", int(age.Seconds()), "valid", valid)

	return valid
}

// GetCacheAge returns the age of the cache in seconds
func GetCacheAge(cache *ExportCache) time.Duration {
	if cache == nil {
		return 0
	}
	cacheTime := time.Unix(cache.Timestamp, 0)
	return time.Since(cacheTime)
}
