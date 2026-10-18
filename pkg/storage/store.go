package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// Store manages persistence and querying of snapshots.
type Store struct {
	mu      sync.RWMutex
	baseDir string
	index   map[string]*archive.Snapshot
}

// NewStore initializes the storage directory structure.
func NewStore(baseDir string) (*Store, error) {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".timecapsule")
	}

	capsulesDir := filepath.Join(baseDir, "capsules")
	if err := os.MkdirAll(capsulesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage dir: %w", err)
	}

	s := &Store{
		baseDir: baseDir,
		index:   make(map[string]*archive.Snapshot),
	}

	// Load existing index if present
	if err := s.loadIndex(); err != nil {
		return nil, err
	}

	return s, nil
}

// BaseDir returns the root storage directory path.
func (s *Store) BaseDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseDir
}

// Save stores the snapshot metadata, raw HTML, and WARC file atomically.
func (s *Store) Save(snap *archive.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.baseDir, "capsules", snap.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	// 1. Write HTML
	htmlPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(htmlPath, []byte(snap.HTML), 0644); err != nil {
		return fmt.Errorf("failed to write html: %w", err)
	}

	// 2. Write Markdown
	mdPath := filepath.Join(dir, "content.md")
	if err := os.WriteFile(mdPath, []byte(snap.Markdown), 0644); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	// 3. Write WARC
	warcBytes := archive.GenerateWARC(snap)
	warcPath := filepath.Join(dir, "archive.warc")
	if err := os.WriteFile(warcPath, warcBytes, 0644); err != nil {
		return fmt.Errorf("failed to write warc: %w", err)
	}

	// 4. Update in-memory index
	s.index[snap.ID] = snap

	// 5. Persist index.json
	return s.persistIndex()
}

// Get retrieves a snapshot by ID.
func (s *Store) Get(id string) (*archive.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.index[id]
	return snap, ok
}

// FindByWayback retrieves closest snapshot for a given URL and timestamp.
func (s *Store) FindByWayback(targetURL, waybackTS string) (*archive.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetNorm := normalizeURL(targetURL)
	var matches []*archive.Snapshot

	for _, snap := range s.index {
		if normalizeURL(snap.URL) == targetNorm {
			matches = append(matches, snap)
		}
	}

	if len(matches) == 0 {
		return nil, false
	}

	// Sort chronologically
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].WaybackTS < matches[j].WaybackTS
	})

	if waybackTS == "" {
		// Return latest
		return matches[len(matches)-1], true
	}

	// Find closest timestamp
	for _, m := range matches {
		if m.WaybackTS >= waybackTS {
			return m, true
		}
	}

	return matches[len(matches)-1], true
}

// GetHistory returns all historical snapshots for targetURL sorted chronologically.
func (s *Store) GetHistory(targetURL string) []*archive.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetNorm := normalizeURL(targetURL)
	var matches []*archive.Snapshot

	for _, snap := range s.index {
		if normalizeURL(snap.URL) == targetNorm {
			matches = append(matches, snap)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Timestamp.Before(matches[j].Timestamp)
	})

	return matches
}

// List returns all stored snapshots sorted by timestamp descending.
func (s *Store) List() []*archive.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*archive.Snapshot, 0, len(s.index))
	for _, snap := range s.index {
		list = append(list, snap)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp.After(list[j].Timestamp)
	})

	return list
}

// Search queries snapshots by keyword in URL, title, or markdown content.
func (s *Store) Search(query string) []*archive.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return s.List()
	}

	var results []*archive.Snapshot
	for _, snap := range s.index {
		if strings.Contains(strings.ToLower(snap.Title), q) ||
			strings.Contains(strings.ToLower(snap.URL), q) ||
			strings.Contains(strings.ToLower(snap.Markdown), q) {
			results = append(results, snap)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results
}

func (s *Store) persistIndex() error {
	indexPath := filepath.Join(s.baseDir, "index.json")
	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, data, 0644)
}

func (s *Store) loadIndex() error {
	indexPath := filepath.Join(s.baseDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.index)
}

func normalizeURL(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimSuffix(u, "/")
	return u
}
