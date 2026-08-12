// Package cache implements disk-based document parse cache.
// Cache path: ~/.xparse-cli/cache/{doc_id}
package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.intsig.net/xparse/xparse-client/internal/models"
)

// DocID computes the document identifier: sha256(abs_filepath)[:12].
func DocID(absFilepath string) string {
	h := sha256.Sum256([]byte(absFilepath))
	return fmt.Sprintf("%x", h)[:12]
}

// CacheDir returns the cache directory path (~/.xparse-cli/cache).
func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xparse-cli", "cache")
}

// cachePath returns the full cache file path for a given doc_id.
func cachePath(docID string) string {
	return filepath.Join(CacheDir(), docID+".json")
}

// Has checks if a cached parse result exists for the given doc_id.
func Has(docID string) bool {
	_, err := os.Stat(cachePath(docID))
	return err == nil
}

// Get reads a cached ParseResult from disk.
func Get(docID string) (*models.ParseResult, error) {
	data, err := os.ReadFile(cachePath(docID))
	if err != nil {
		return nil, fmt.Errorf("cache miss for doc_id %s: %w", docID, err)
	}
	var result models.ParseResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("cache corrupt for doc_id %s: %w", docID, err)
	}
	return &result, nil
}

// Set writes a ParseResult to disk cache.
func Set(docID string, result *models.ParseResult) error {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal parse result: %w", err)
	}
	return os.WriteFile(cachePath(docID), data, 0o644)
}

// docInfoDir returns the directory for doc_info files (~/.xparse-cli/docinfo).
func docInfoDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xparse-cli", "docinfo")
}

// docInfoPath returns the full path for a doc_info file.
func docInfoPath(docID string) string {
	return filepath.Join(docInfoDir(), docID+".json")
}

// SetDocInfo persists a DocInfo to disk.
func SetDocInfo(docID string, info *models.DocInfo) error {
	dir := docInfoDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create docinfo dir: %w", err)
	}
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal doc info: %w", err)
	}
	return os.WriteFile(docInfoPath(docID), data, 0o644)
}

// GetDocInfo reads a DocInfo from disk.
func GetDocInfo(docID string) (*models.DocInfo, error) {
	data, err := os.ReadFile(docInfoPath(docID))
	if err != nil {
		return nil, fmt.Errorf("doc_id %s not found; call get_doc_info first: %w", docID, err)
	}
	var info models.DocInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("corrupt doc_info for %s: %w", docID, err)
	}
	return &info, nil
}

// GetFilepathByDocID retrieves filepath from persisted DocInfo.
func GetFilepathByDocID(docID string) (string, error) {
	info, err := GetDocInfo(docID)
	if err != nil {
		return "", err
	}
	return info.Filepath, nil
}

// CacheEntry represents a cached document entry for listing.
type CacheEntry struct {
	DocID    string `json:"doc_id"`
	Filepath string `json:"filepath"`
	Filename string `json:"filename"`
	Parsed   bool   `json:"parsed"`
}

// ListAll lists all cached doc_info entries with their parse status.
func ListAll() ([]CacheEntry, error) {
	dir := docInfoDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []CacheEntry
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		docID := e.Name()[:len(e.Name())-5] // strip .json
		info, err := GetDocInfo(docID)
		if err != nil {
			continue
		}
		result = append(result, CacheEntry{
			DocID:    docID,
			Filepath: info.Filepath,
			Filename: info.Filename,
			Parsed:   Has(docID),
		})
	}
	return result, nil
}

// BaseDir returns the root cache directory (~/.xparse-cli).
func BaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".xparse-cli")
}

// CleanAll removes document data without deleting credentials or telemetry outbox files.
func CleanAll() error {
	for _, dir := range []string{CacheDir(), docInfoDir()} {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to clean cache: %w", err)
		}
	}
	return nil
}
