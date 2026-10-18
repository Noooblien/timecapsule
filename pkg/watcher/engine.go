package watcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/diff"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

// Engine governs the continuous scheduling, polling, and drift evaluation of web targets.
type Engine struct {
	mu           sync.RWMutex
	store        *storage.Store
	archiver     *archive.Archiver
	targets      map[string]*WatchTarget
	events       []ChangeEvent
	registryPath string
	httpClient   *http.Client
}

// RegistryFile captures the serialized configuration of all watch targets and events.
type RegistryFile struct {
	Targets []*WatchTarget `json:"targets"`
	Events  []ChangeEvent  `json:"events"`
}

// NewEngine initializes a thread-safe continuous watcher engine.
func NewEngine(store *storage.Store, archiver *archive.Archiver, dataDir string) (*Engine, error) {
	regPath := filepath.Join(dataDir, "watchers.json")
	eng := &Engine{
		store:        store,
		archiver:     archiver,
		targets:      make(map[string]*WatchTarget),
		events:       make([]ChangeEvent, 0),
		registryPath: regPath,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}

	if err := eng.loadRegistry(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load watcher registry: %w", err)
	}

	return eng, nil
}

func (e *Engine) loadRegistry() error {
	data, err := os.ReadFile(e.registryPath)
	if err != nil {
		return err
	}

	var reg RegistryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return err
	}

	for _, t := range reg.Targets {
		d, err := time.ParseDuration(t.IntervalStr)
		if err != nil || d <= 0 {
			d = 5 * time.Minute
		}
		t.IntervalDuration = d
		e.targets[t.ID] = t
	}

	e.events = reg.Events
	return nil
}

func (e *Engine) saveRegistry() error {
	var targets []*WatchTarget
	for _, t := range e.targets {
		targets = append(targets, t)
	}

	// Keep events bounded to last 500
	events := e.events
	if len(events) > 500 {
		events = events[len(events)-500:]
	}

	reg := RegistryFile{
		Targets: targets,
		Events:  events,
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(e.registryPath, data, 0644)
}

// AddTarget registers a new URL for continuous temporal monitoring.
func (e *Engine) AddTarget(urlStr, name, intervalStr, webhookURL string, threshold float64, ignoreSelectors, tags []string) (*WatchTarget, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return nil, fmt.Errorf("target URL cannot be empty")
	}

	if intervalStr == "" {
		intervalStr = "5m"
	}
	d, err := time.ParseDuration(intervalStr)
	if err != nil || d <= 0 {
		d = 5 * time.Minute
		intervalStr = "5m"
	}

	if name == "" {
		name = urlStr
	}

	if threshold <= 0 {
		threshold = 0.05 // 5% default drift threshold
	}

	// Check if already registered
	for _, t := range e.targets {
		if strings.EqualFold(t.URL, urlStr) {
			t.Name = name
			t.IntervalStr = intervalStr
			t.IntervalDuration = d
			t.WebhookURL = webhookURL
			t.DriftThreshold = threshold
			t.IgnoreSelectors = ignoreSelectors
			t.Tags = tags
			_ = e.saveRegistry()
			return t, nil
		}
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	targetID := "wt_" + hex.EncodeToString(b)

	target := &WatchTarget{
		ID:               targetID,
		URL:              urlStr,
		Name:             name,
		IntervalStr:      intervalStr,
		IntervalDuration: d,
		Status:           StatusActive,
		DriftThreshold:   threshold,
		IgnoreSelectors:  ignoreSelectors,
		WebhookURL:       webhookURL,
		Tags:             tags,
		CreatedAt:        time.Now().UTC(),
	}

	e.targets[targetID] = target
	_ = e.saveRegistry()
	return target, nil
}

// RemoveTarget deletes a watch target by ID.
func (e *Engine) RemoveTarget(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.targets[id]; !ok {
		return fmt.Errorf("target %s not found", id)
	}

	delete(e.targets, id)
	return e.saveRegistry()
}

// GetTarget retrieves a watch target by ID.
func (e *Engine) GetTarget(id string) (*WatchTarget, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.targets[id]
	return t, ok
}

// ListTargets returns all configured watch targets sorted by creation date.
func (e *Engine) ListTargets() []*WatchTarget {
	e.mu.RLock()
	defer e.mu.RUnlock()

	targets := make([]*WatchTarget, 0, len(e.targets))
	for _, t := range e.targets {
		targets = append(targets, t)
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].CreatedAt.Before(targets[j].CreatedAt)
	})

	return targets
}

// GetEvents returns detected change events, optionally filtered by target ID.
func (e *Engine) GetEvents(targetID string) []ChangeEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if targetID == "" {
		res := make([]ChangeEvent, len(e.events))
		copy(res, e.events)
		return res
	}

	var res []ChangeEvent
	for _, ev := range e.events {
		if ev.TargetID == targetID {
			res = append(res, ev)
		}
	}
	return res
}

// PollTarget executes a single check cycle for a specific target.
func (e *Engine) PollTarget(ctx context.Context, targetID string) (*ChangeEvent, error) {
	e.mu.Lock()
	target, ok := e.targets[targetID]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("target %s not found", targetID)
	}
	e.mu.Unlock()

	snap, err := e.archiver.Capture(ctx, target.URL)
	now := time.Now().UTC()

	e.mu.Lock()
	defer e.mu.Unlock()

	target.LastCheck = now
	target.CheckCount++

	if err != nil {
		target.Status = StatusUnreachable
		_ = e.saveRegistry()
		return nil, fmt.Errorf("failed to fetch target: %w", err)
	}

	cleanNew := SanitizeContent(snap.HTML, target.IgnoreSelectors)
	newHash := ComputeContentHash(cleanNew)

	// Check existing history
	history := e.store.GetHistory(target.URL)
	if len(history) == 0 {
		// First capture establishes initial baseline
		if err := e.store.Save(snap); err != nil {
			return nil, fmt.Errorf("failed to save baseline snapshot: %w", err)
		}

		target.BaselineHash = newHash
		target.LastHash = newHash
		target.LastSnapshotID = snap.ID
		target.Status = StatusActive
		_ = e.saveRegistry()
		return nil, nil
	}

	latest := history[len(history)-1]
	cleanOld := SanitizeContent(latest.HTML, target.IgnoreSelectors)
	oldHash := ComputeContentHash(cleanOld)

	if oldHash == newHash {
		target.Status = StatusActive
		_ = e.saveRegistry()
		return nil, nil // No drift
	}

	// Classify drift
	severity, delta, summary := ClassifyDrift(cleanOld, cleanNew, snap.StatusCode, target.DriftThreshold)
	if severity == DriftNone {
		// Change was below configured noise threshold
		target.Status = StatusActive
		_ = e.saveRegistry()
		return nil, nil
	}

	// Substantial drift detected: persist snapshot
	if err := e.store.Save(snap); err != nil {
		return nil, fmt.Errorf("failed to store mutated snapshot: %w", err)
	}

	diffResult := diff.CompareSnapshots(latest, snap)

	evID := fmt.Sprintf("chg_%s_%d", snap.ID[:8], now.Unix())
	changeEv := ChangeEvent{
		ID:            evID,
		TargetID:      target.ID,
		TargetName:    target.Name,
		TargetURL:     target.URL,
		Timestamp:     now,
		OldSnapshotID: latest.ID,
		NewSnapshotID: snap.ID,
		OldHash:       oldHash,
		NewHash:       newHash,
		Severity:      severity,
		DeltaRatio:    delta,
		Summary:       summary,
		LinesAdded:    diffResult.AddedCount,
		LinesDeleted:  diffResult.RemovedCount,
	}

	// Dispatch Webhook if configured
	if target.WebhookURL != "" {
		whStatus := e.dispatchWebhook(target, changeEv)
		changeEv.WebhookStatus = whStatus
	}

	target.LastChange = now
	target.LastSnapshotID = snap.ID
	target.LastHash = newHash
	target.ChangeCount++
	target.Status = StatusDriftDetected

	e.events = append(e.events, changeEv)
	_ = e.saveRegistry()

	return &changeEv, nil
}

// PollAll concurrently checks all active watch targets.
func (e *Engine) PollAll(ctx context.Context) []*ChangeEvent {
	e.mu.RLock()
	var activeIDs []string
	for id, t := range e.targets {
		if t.Status != StatusPaused {
			activeIDs = append(activeIDs, id)
		}
	}
	e.mu.RUnlock()

	var wg sync.WaitGroup
	resultsChan := make(chan *ChangeEvent, len(activeIDs))

	for _, id := range activeIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			ev, err := e.PollTarget(ctx, tid)
			if err == nil && ev != nil {
				resultsChan <- ev
			}
		}(id)
	}

	wg.Wait()
	close(resultsChan)

	var detected []*ChangeEvent
	for ev := range resultsChan {
		detected = append(detected, ev)
	}

	return detected
}

func (e *Engine) dispatchWebhook(t *WatchTarget, ev ChangeEvent) string {
	payload := WebhookPayload{
		Event:         "website.drift_detected",
		TargetID:      t.ID,
		TargetName:    t.Name,
		TargetURL:     t.URL,
		Timestamp:     ev.Timestamp,
		Severity:      ev.Severity,
		DeltaRatio:    ev.DeltaRatio,
		Summary:       ev.Summary,
		OldSnapshotID: ev.OldSnapshotID,
		NewSnapshotID: ev.NewSnapshotID,
		DiffURL:       fmt.Sprintf("/diff?base=%s&target=%s", ev.OldSnapshotID, ev.NewSnapshotID),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "marshal_failed"
	}

	req, err := http.NewRequest("POST", t.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return "req_error"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TimeCapsule-Watcher/1.0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "failed: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return fmt.Sprintf("delivered (HTTP %d)", resp.StatusCode)
	}
	return fmt.Sprintf("rejected (HTTP %d)", resp.StatusCode)
}
