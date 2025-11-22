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

// GetCachePath returns the full path to the cache file
func GetCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(homeDir, DefaultCacheFile), nil
}

// ReadExportCache reads the cached export data if it exists and is valid
func ReadExportCache() (*ExportCache, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("cache file does not exist", "path", cachePath)
			return nil, nil // No cache exists, not an error
		}
		return nil, fmt.Errorf("error reading cache file: %w", err)
	}

	var cache ExportCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("error parsing cache file: %w", err)
	}

	slog.Debug("cache file read successfully", "path", cachePath, "timestamp", cache.Timestamp)
	return &cache, nil
}

// WriteExportCache writes the export data to cache with current timestamp
// data should be any type that can be marshaled to JSON
func WriteExportCache(data interface{}) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("error creating cache directory: %w", err)
	}

	// Marshal the data to JSON
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error encoding data: %w", err)
	}

	cache := ExportCache{
		Timestamp: time.Now().Unix(),
		Data:      dataJSON,
	}

	cacheData, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("error encoding cache data: %w", err)
	}

	if err := os.WriteFile(cachePath, cacheData, 0644); err != nil {
		return fmt.Errorf("error writing cache file: %w", err)
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
