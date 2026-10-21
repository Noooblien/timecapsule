package memento

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockProvider struct {
	mementos map[string][]*Memento
}

func (m *mockProvider) GetMementos(originalURL string) ([]*Memento, error) {
	return m.mementos[originalURL], nil
}

func sampleMementos(originalURL, baseURL string) []*Memento {
	t1, _ := time.Parse(time.RFC3339, "2024-01-15T10:00:00Z")
	t2, _ := time.Parse(time.RFC3339, "2025-06-01T12:00:00Z")
	t3, _ := time.Parse(time.RFC3339, "2026-03-20T18:00:00Z")

	return []*Memento{
		{
			URI:         baseURL + "/web/20240115100000/" + originalURL,
			Datetime:    t1,
			OriginalURL: originalURL,
			StatusCode:  200,
			MimeType:    "text/html",
		},
		{
			URI:         baseURL + "/web/20250601120000/" + originalURL,
			Datetime:    t2,
			OriginalURL: originalURL,
			StatusCode:  200,
			MimeType:    "text/html",
		},
		{
			URI:         baseURL + "/web/20260320180000/" + originalURL,
			Datetime:    t3,
			OriginalURL: originalURL,
			StatusCode:  200,
			MimeType:    "text/html",
		},
	}
}

func TestDateParsing(t *testing.T) {
	testCases := []struct {
		input       string
		expectYear  int
		expectMonth time.Month
		expectDay   int
		shouldPass  bool
	}{
		{"Tue, 20 Mar 2026 18:00:00 GMT", 2026, time.March, 20, true},
		{"Tuesday, 20-Mar-26 18:00:00 GMT", 2026, time.March, 20, true},
		{"2026-03-20T18:00:00Z", 2026, time.March, 20, true},
		{"20260320180000", 2026, time.March, 20, true},
		{"invalid-date-string", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}

	for _, tc := range testCases {
		parsed, err := ParseHTTPDate(tc.input)
		if tc.shouldPass {
			if err != nil {
				t.Errorf("failed to parse valid date '%s': %v", tc.input, err)
			}
			if parsed.Year() != tc.expectYear || parsed.Month() != tc.expectMonth || parsed.Day() != tc.expectDay {
				t.Errorf("date mismatch for '%s': got %v", tc.input, parsed)
			}
		} else {
			if err == nil {
				t.Errorf("expected error parsing invalid date '%s', but succeeded", tc.input)
			}
		}
	}
}

func TestTimeGate_Resolution(t *testing.T) {
	originalURL := "https://example.com/pricing"
	baseURL := "https://timecapsule.local"
	items := sampleMementos(originalURL, baseURL)

	tg := NewTimeGate(originalURL, baseURL, items)
	if tg.Count() != 3 {
		t.Fatalf("expected 3 mementos, got %d", tg.Count())
	}

	// 1. Request time way before first memento -> Should return first
	beforeTime, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	res1, err := tg.Resolve(beforeTime)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if res1.Selected.URI != items[0].URI {
		t.Errorf("expected first memento %s, got %s", items[0].URI, res1.Selected.URI)
	}
	if res1.Prev != nil {
		t.Errorf("first memento should have no prev link")
	}
	if res1.Next == nil || res1.Next.URI != items[1].URI {
		t.Errorf("expected next memento to be items[1]")
	}

	// 2. Request time after last memento -> Should return last
	afterTime, _ := time.Parse(time.RFC3339, "2027-01-01T00:00:00Z")
	res2, err := tg.Resolve(afterTime)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if res2.Selected.URI != items[2].URI {
		t.Errorf("expected last memento %s, got %s", items[2].URI, res2.Selected.URI)
	}
	if res2.Next != nil {
		t.Errorf("last memento should have no next link")
	}
	if res2.Prev == nil || res2.Prev.URI != items[1].URI {
		t.Errorf("expected prev memento to be items[1]")
	}

	// 3. Request time closer to middle memento (2025-06-01)
	midTime, _ := time.Parse(time.RFC3339, "2025-07-01T00:00:00Z")
	res3, err := tg.Resolve(midTime)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if res3.Selected.URI != items[1].URI {
		t.Errorf("expected middle memento %s, got %s", items[1].URI, res3.Selected.URI)
	}
	if res3.Prev == nil || res3.Prev.URI != items[0].URI {
		t.Errorf("expected prev to be items[0]")
	}
	if res3.Next == nil || res3.Next.URI != items[2].URI {
		t.Errorf("expected next to be items[2]")
	}
}

func TestTimeGate_LinkHeaderFormatting(t *testing.T) {
	originalURL := "https://example.com/pricing"
	baseURL := "https://timecapsule.local"
	items := sampleMementos(originalURL, baseURL)

	tg := NewTimeGate(originalURL, baseURL, items)
	targetTime, _ := time.Parse(time.RFC3339, "2025-06-01T12:00:00Z")
	res, _ := tg.Resolve(targetTime)

	links := tg.BuildLinkHeader(res)

	// Verify RFC 7089 Section 2.1 link relations
	if !strings.Contains(links, "<https://example.com/pricing>; rel=\"original\"") {
		t.Errorf("missing rel=original link header")
	}
	if !strings.Contains(links, "rel=\"timegate\"") {
		t.Errorf("missing rel=timegate link header")
	}
	if !strings.Contains(links, "rel=\"timemap\"; type=\"application/link-format\"") {
		t.Errorf("missing rel=timemap link header")
	}
	if !strings.Contains(links, "rel=\"memento\"") {
		t.Errorf("missing rel=memento link header")
	}
	if !strings.Contains(links, "first") || !strings.Contains(links, "last") {
		t.Errorf("missing first or last relations in link header")
	}
}

func TestTimeMap_Serialization(t *testing.T) {
	originalURL := "https://example.com/pricing"
	baseURL := "https://timecapsule.local"
	items := sampleMementos(originalURL, baseURL)

	tm := NewTimeMap(originalURL, baseURL, items)

	// 1. Test application/link-format
	linkFormat := tm.FormatLinkFormat()
	if !strings.Contains(linkFormat, "rel=\"original\"") {
		t.Errorf("link format missing original relation")
	}
	if !strings.Contains(linkFormat, "rel=\"first memento\"") {
		t.Errorf("link format missing first memento")
	}
	if !strings.Contains(linkFormat, "rel=\"last memento\"") {
		t.Errorf("link format missing last memento")
	}

	// 2. Test JSON TimeMap
	jsonBytes, err := tm.FormatJSON()
	if err != nil {
		t.Fatalf("failed serializing JSON timemap: %v", err)
	}

	var parsed JSONTimeMap
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed deserializing JSON timemap: %v", err)
	}

	if parsed.OriginalURL != originalURL {
		t.Errorf("expected original_uri %s, got %s", originalURL, parsed.OriginalURL)
	}
	if len(parsed.Mementos) != 3 {
		t.Errorf("expected 3 mementos in JSON timemap, got %d", len(parsed.Mementos))
	}
}

func TestServerHandler_HTTPFlow(t *testing.T) {
	originalURL := "https://example.com/pricing"
	baseURL := "http://localhost:8080"
	items := sampleMementos(originalURL, baseURL)

	provider := &mockProvider{
		mementos: map[string][]*Memento{
			originalURL: items,
		},
	}

	handler := NewServerHandler(baseURL, provider)

	// 1. TimeGate Request with Accept-Datetime
	req := httptest.NewRequest(http.MethodGet, "/timegate/"+originalURL, nil)
	req.Header.Set("Accept-Datetime", "Sun, 01 Jun 2025 12:00:00 GMT")
	w := httptest.NewRecorder()

	handler.HandleTimeGate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 Found, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Vary") != "Accept-Datetime" {
		t.Errorf("expected Vary: Accept-Datetime header")
	}
	if resp.Header.Get("Location") != items[1].URI {
		t.Errorf("expected Location %s, got %s", items[1].URI, resp.Header.Get("Location"))
	}
	if resp.Header.Get("Memento-Datetime") == "" {
		t.Errorf("expected Memento-Datetime header")
	}
	if resp.Header.Get("Link") == "" {
		t.Errorf("expected RFC 7089 Link headers")
	}

	// 2. TimeMap Request (link-format)
	reqMap := httptest.NewRequest(http.MethodGet, "/timemap/link/"+originalURL, nil)
	wMap := httptest.NewRecorder()

	handler.HandleTimeMap(wMap, reqMap)
	respMap := wMap.Result()

	if respMap.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for timemap, got %d", respMap.StatusCode)
	}
	if !strings.Contains(respMap.Header.Get("Content-Type"), LinkFormatMimeType) {
		t.Errorf("expected Content-Type application/link-format, got %s", respMap.Header.Get("Content-Type"))
	}

	// 3. TimeMap Request (json)
	reqJSON := httptest.NewRequest(http.MethodGet, "/timemap/json/"+originalURL, nil)
	wJSON := httptest.NewRecorder()

	handler.HandleTimeMap(wJSON, reqJSON)
	respJSON := wJSON.Result()

	if respJSON.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for json timemap, got %d", respJSON.StatusCode)
	}
	if !strings.Contains(respJSON.Header.Get("Content-Type"), "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", respJSON.Header.Get("Content-Type"))
	}
}

func TestTimeGate_ConcurrentAccess(t *testing.T) {
	originalURL := "https://example.com/pricing"
	baseURL := "https://timecapsule.local"
	items := sampleMementos(originalURL, baseURL)

	tg := NewTimeGate(originalURL, baseURL, items)

	var wg sync.WaitGroup
	workers := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				reqTime := time.Date(2024+workerID%3, time.Month(1+(j%12)), 15, 12, 0, 0, 0, time.UTC)
				res, err := tg.Resolve(reqTime)
				if err != nil {
					t.Errorf("concurrent resolve error: %v", err)
				}
				_ = tg.BuildLinkHeader(res)
			}
		}(i)
	}

	wg.Wait()
}
