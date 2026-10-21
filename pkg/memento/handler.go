package memento

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Provider retrieves historical mementos for a given original URL.
type Provider interface {
	GetMementos(originalURL string) ([]*Memento, error)
}

// ServerHandler exposes RFC 7089 compliant HTTP endpoints for TimeGate and TimeMap.
type ServerHandler struct {
	baseURL  string
	provider Provider
}

// NewServerHandler creates a new RFC 7089 HTTP handler.
func NewServerHandler(baseURL string, provider Provider) *ServerHandler {
	return &ServerHandler{
		baseURL:  strings.TrimRight(baseURL, "/"),
		provider: provider,
	}
}

// HandleTimeGate handles requests to /timegate/{original_url...} (RFC 7089 Section 2.2).
func (h *ServerHandler) HandleTimeGate(w http.ResponseWriter, r *http.Request) {
	originalURL := extractTargetURL(r.URL.Path, "/timegate/")
	if originalURL == "" {
		http.Error(w, "missing target URL parameter", http.StatusBadRequest)
		return
	}

	mementos, err := h.provider.GetMementos(originalURL)
	if err != nil || len(mementos) == 0 {
		http.Error(w, fmt.Sprintf("no archived mementos found for %s", originalURL), http.StatusNotFound)
		return
	}

	tg := NewTimeGate(originalURL, h.baseURL, mementos)

	// Determine requested time from Accept-Datetime header (RFC 7089 Section 2.1.1)
	acceptDatetime := r.Header.Get("Accept-Datetime")
	var targetTime time.Time
	if acceptDatetime != "" {
		parsed, parseErr := ParseHTTPDate(acceptDatetime)
		if parseErr != nil {
			http.Error(w, fmt.Sprintf("invalid Accept-Datetime header '%s': %v", acceptDatetime, parseErr), http.StatusBadRequest)
			return
		}
		targetTime = parsed
	} else {
		// Default to current time (which resolves to the most recent memento)
		targetTime = time.Now().UTC()
	}

	resolution, err := tg.Resolve(targetTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build RFC 7089 Link headers
	linkHeader := tg.BuildLinkHeader(resolution)

	w.Header().Set("Vary", "Accept-Datetime")
	w.Header().Set("Link", linkHeader)
	w.Header().Set("Location", resolution.Selected.URI)
	w.Header().Set("Memento-Datetime", resolution.Selected.Datetime.Format(RFC1123Format))

	// Return 302 Found redirect to the selected Memento URI
	w.WriteHeader(http.StatusFound)
}

// HandleTimeMap handles requests to /timemap/link/{url...} and /timemap/json/{url...} (RFC 7089 Section 5).
func (h *ServerHandler) HandleTimeMap(w http.ResponseWriter, r *http.Request) {
	isJSON := strings.HasPrefix(r.URL.Path, "/timemap/json/")
	prefix := "/timemap/link/"
	if isJSON {
		prefix = "/timemap/json/"
	}

	originalURL := extractTargetURL(r.URL.Path, prefix)
	if originalURL == "" {
		http.Error(w, "missing target URL parameter", http.StatusBadRequest)
		return
	}

	mementos, err := h.provider.GetMementos(originalURL)
	if err != nil || len(mementos) == 0 {
		http.Error(w, fmt.Sprintf("no archived mementos found for %s", originalURL), http.StatusNotFound)
		return
	}

	tm := NewTimeMap(originalURL, h.baseURL, mementos)

	if isJSON || strings.Contains(r.Header.Get("Accept"), "application/json") {
		jsonBytes, err := tm.FormatJSON()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes)
		return
	}

	// Canonical RFC 7089 Section 5 format
	linkPayload := tm.FormatLinkFormat()
	w.Header().Set("Content-Type", LinkFormatMimeType+"; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(linkPayload))
}

func extractTargetURL(path, prefix string) string {
	raw := strings.TrimPrefix(path, prefix)
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		// Allow raw domains like example.com by prepending https://
		if strings.Contains(raw, ".") {
			raw = "https://" + raw
		}
	}
	return raw
}
