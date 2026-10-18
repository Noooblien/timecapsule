package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/diff"
	"github.com/Noooblien/timecapsule/pkg/reader"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

// Server provides the Wayback Machine HTTP interface and API.
type Server struct {
	store    *storage.Store
	archiver *archive.Archiver
	mux      *http.ServeMux
	server   *http.Server
}

// NewServer initializes the HTTP server routes.
func NewServer(store *storage.Store, archiver *archive.Archiver) *Server {
	s := &Server{
		store:    store,
		archiver: archiver,
		mux:      http.NewServeMux(),
	}

	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/snapshots", s.handleAPISnapshots)
	s.mux.HandleFunc("/api/capture", s.handleAPICapture)
	s.mux.HandleFunc("/api/diff", s.handleAPIDiff)
	s.mux.HandleFunc("/cdx", s.handleCDX)
	s.mux.HandleFunc("/api/cdx", s.handleCDX)
	s.mux.HandleFunc("/web/", s.handleWaybackReplay)
	s.mux.HandleFunc("/diff/", s.handleDiff)
	s.mux.HandleFunc("/raw/", s.handleRaw)
	s.mux.HandleFunc("/markdown/", s.handleMarkdown)
	s.mux.HandleFunc("/reader/", s.handleReader)
	s.mux.HandleFunc("/warc/", s.handleWARC)

	return s
}

// ServeHTTP dispatches requests, handling uncleaned double-slash URLs for /web/ directly.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/web/") {
		s.handleWaybackReplay(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// Start begins listening on addr.
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s,
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully terminates the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Handler returns the underlying http.Handler for testing.
func (s *Server) Handler() http.Handler {
	return s
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query().Get("q")
	var snapshots []*archive.Snapshot
	if query != "" {
		snapshots = s.store.Search(query)
	} else {
		snapshots = s.store.List()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.renderDashboard(w, snapshots, query)
}

func (s *Server) handleAPISnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots := s.store.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshots)
}

func (s *Server) handleAPICapture(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}

	snap, err := s.archiver.Capture(r.Context(), targetURL)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if err := s.store.Save(snap); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"save failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleAPIDiff(w http.ResponseWriter, r *http.Request) {
	oldID := r.URL.Query().Get("old")
	newID := r.URL.Query().Get("new")

	if oldID == "" || newID == "" {
		http.Error(w, `{"error":"missing 'old' or 'new' query parameter"}`, http.StatusBadRequest)
		return
	}

	oldSnap, ok := s.store.Get(oldID)
	if !ok {
		http.Error(w, fmt.Sprintf(`{"error":"snapshot '%s' not found"}`, oldID), http.StatusNotFound)
		return
	}
	newSnap, ok := s.store.Get(newID)
	if !ok {
		http.Error(w, fmt.Sprintf(`{"error":"snapshot '%s' not found"}`, newID), http.StatusNotFound)
		return
	}

	res := diff.CompareSnapshots(oldSnap, newSnap)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleWaybackReplay(w http.ResponseWriter, r *http.Request) {
	// Format: /web/{timestamp}/{url}
	trimmed := strings.TrimPrefix(r.URL.Path, "/web/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "Invalid Wayback URL format. Expected: /web/{timestamp}/{url}", http.StatusBadRequest)
		return
	}

	waybackTS := parts[0]
	targetURL := parts[1]
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	snap, ok := s.store.FindByWayback(targetURL, waybackTS)
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>TimeCapsule: Not Found</title><style>body{background:#0d1117;color:#c9d1d9;font-family:sans-serif;padding:40px;text-align:center;}</style></head><body><h1>404 Snapshot Not Found</h1><p>No archive found for <code>%s</code> at timestamp <code>%s</code>.</p><a href="/" style="color:#58a6ff;">← Back to TimeCapsule</a></body></html>`, html.EscapeString(targetURL), html.EscapeString(waybackTS))
		return
	}

	banner := fmt.Sprintf(`<div style="position:fixed;top:0;left:0;right:0;height:42px;background:#161b22;border-bottom:2px solid #238636;color:#c9d1d9;font-family:-apple-system,BlinkMacSystemFont,sans-serif;font-size:13px;display:flex;align-items:center;justify-content:space-between;padding:0 20px;z-index:999999;box-shadow:0 4px 12px rgba(0,0,0,0.5);">`+
		`<div><strong>⏳ TimeCapsule Archive</strong> &bull; Snapshot from <code>%s</code> (Hash: <code>%s</code>)</div>`+
		`<div><a href="%s" target="_blank" style="color:#58a6ff;margin-right:15px;text-decoration:none;">Original Live URL ↗</a><a href="/" style="color:#7ee787;text-decoration:none;font-weight:bold;">← Exit Archive</a></div>`+
		`</div><div style="height:42px;"></div>`, snap.Timestamp.Format(time.RFC1123), snap.ContentHash[:8], snap.URL)

	injectedHTML := injectBanner(snap.HTML, banner)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(injectedHTML))
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/diff/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Invalid diff path. Expected: /diff/{old_id}/{new_id}", http.StatusBadRequest)
		return
	}

	oldID := parts[0]
	newID := parts[1]

	oldSnap, ok := s.store.Get(oldID)
	if !ok {
		http.Error(w, fmt.Sprintf("Old snapshot '%s' not found", oldID), http.StatusNotFound)
		return
	}
	newSnap, ok := s.store.Get(newID)
	if !ok {
		http.Error(w, fmt.Sprintf("New snapshot '%s' not found", newID), http.StatusNotFound)
		return
	}

	res := diff.CompareSnapshots(oldSnap, newSnap)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	diffHTML := res.FormatHTML()

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>TimeCapsule &bull; Diff %s vs %s</title>
<style>
:root { --bg: #0d1117; --card: #161b22; --border: #30363d; --text: #c9d1d9; --accent: #58a6ff; --green: #238636; --red: #da3633; }
body { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
.container { max-width: 1100px; margin: 30px auto; padding: 0 20px; }
header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border); padding-bottom: 16px; margin-bottom: 24px; }
h1 { margin: 0; font-size: 22px; color: #f0f6fc; }
.back-link { color: var(--accent); text-decoration: none; font-size: 14px; }
.meta-card { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 20px; margin-bottom: 24px; display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
.snap-info h3 { margin: 0 0 8px 0; font-size: 16px; }
.snap-info p { margin: 4px 0; font-size: 13px; color: #8b949e; }
.stats-bar { display: flex; gap: 16px; margin-bottom: 20px; align-items: center; font-size: 14px; }
.pill { padding: 4px 12px; border-radius: 12px; font-weight: 600; font-size: 12px; }
.pill-green { background: #2ea04326; color: #7ee787; border: 1px solid #238636; }
.pill-red { background: #da363326; color: #f85149; border: 1px solid #da3633; }
.pill-blue { background: #1f6feb26; color: #58a6ff; border: 1px solid #1f6feb; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>⇄ Snapshot Comparison</h1>
    <a class="back-link" href="/">← Return to Dashboard</a>
  </header>

  <div class="meta-card">
    <div class="snap-info">
      <h3 style="color:#f85149;">Baseline (Older)</h3>
      <p><strong>ID:</strong> <code>%s</code></p>
      <p><strong>Timestamp:</strong> %s</p>
      <p><strong>Title:</strong> %s</p>
      <p><a href="/web/%s/%s" target="_blank" style="color:var(--accent);">View in Wayback ↗</a></p>
    </div>
    <div class="snap-info">
      <h3 style="color:#7ee787;">Target (Newer)</h3>
      <p><strong>ID:</strong> <code>%s</code></p>
      <p><strong>Timestamp:</strong> %s</p>
      <p><strong>Title:</strong> %s</p>
      <p><a href="/web/%s/%s" target="_blank" style="color:var(--accent);">View in Wayback ↗</a></p>
    </div>
  </div>

  <div class="stats-bar">
    <span class="pill pill-blue">Similarity: %.1f%%%%</span>
    <span class="pill pill-green">+%d lines added</span>
    <span class="pill pill-red">-%d lines removed</span>
    <span style="color:#8b949e;font-size:13px;">URL: <code>%s</code></span>
  </div>

  %s
</div>
</body>
</html>`,
		html.EscapeString(oldID[:8]), html.EscapeString(newID[:8]),
		html.EscapeString(oldSnap.ID), oldSnap.Timestamp.Format("Jan 02, 2006 15:04:05 UTC"), html.EscapeString(oldSnap.Title), oldSnap.WaybackTS, oldSnap.URL,
		html.EscapeString(newSnap.ID), newSnap.Timestamp.Format("Jan 02, 2006 15:04:05 UTC"), html.EscapeString(newSnap.Title), newSnap.WaybackTS, newSnap.URL,
		res.Similarity*100, res.AddedCount, res.RemovedCount, html.EscapeString(newSnap.URL),
		diffHTML,
	)
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/raw/")
	snap, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(snap.HTML))
}

func (s *Server) handleMarkdown(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/markdown/")
	snap, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(snap.Markdown))
}

func (s *Server) handleReader(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/reader/")
	snap, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	opts := reader.DefaultOptions(snap.URL)
	article := reader.Extract(snap.HTML, opts)
	if article.Title == "Untitled Document" && snap.Title != "" {
		article.Title = snap.Title
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(reader.RenderReaderHTML(article)))
}

func (s *Server) handleCDX(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	outputFormat := strings.ToLower(r.URL.Query().Get("output"))
	if outputFormat == "" {
		outputFormat = "json"
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	var snapshots []*archive.Snapshot
	if targetURL != "" {
		snapshots = s.store.GetHistory(targetURL)
	} else {
		snapshots = s.store.List()
	}

	if len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}

	if outputFormat == "cdxj" || outputFormat == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, snap := range snapshots {
			entry := warc.CDXJEntry{
				SURT:        warc.ToSURT(snap.URL),
				Timestamp:   snap.WaybackTS,
				OriginalURL: snap.URL,
				MimeType:    snap.ContentType,
				StatusCode:  snap.StatusCode,
				Digest:      snap.ContentHash,
				Length:      snap.SizeBytes,
				Offset:      0,
				Filename:    fmt.Sprintf("%s.warc", snap.ID),
			}
			w.Write([]byte(warc.FormatCDXJ(entry)))
		}
		return
	}

	// JSON output compatible with Wayback CDX API
	var entries []warc.CDXJEntry
	for _, snap := range snapshots {
		entries = append(entries, warc.CDXJEntry{
			SURT:        warc.ToSURT(snap.URL),
			Timestamp:   snap.WaybackTS,
			OriginalURL: snap.URL,
			MimeType:    snap.ContentType,
			StatusCode:  snap.StatusCode,
			Digest:      snap.ContentHash,
			Length:      snap.SizeBytes,
			Offset:      0,
			Filename:    fmt.Sprintf("%s.warc", snap.ID),
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleWARC(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/warc/")
	isGzip := strings.HasSuffix(id, ".gz") || r.URL.Query().Get("gzip") == "true"
	id = strings.TrimSuffix(id, ".gz")
	id = strings.TrimSuffix(id, ".warc")

	snap, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	filename := fmt.Sprintf("%s.warc", snap.ID)
	if isGzip {
		filename += ".gz"
		w.Header().Set("Content-Type", "application/gzip")
	} else {
		w.Header().Set("Content-Type", "application/warc")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := warc.NewWriter(w, isGzip)
	_, _ = writer.WriteSnapshotArchive(snap, filename)
}

func injectBanner(htmlContent, banner string) string {
	lower := strings.ToLower(htmlContent)
	bodyIdx := strings.Index(lower, "<body")
	if bodyIdx == -1 {
		return banner + htmlContent
	}
	closeIdx := strings.Index(lower[bodyIdx:], ">")
	if closeIdx == -1 {
		return banner + htmlContent
	}
	insertPos := bodyIdx + closeIdx + 1
	return htmlContent[:insertPos] + banner + htmlContent[insertPos:]
}

func (s *Server) renderDashboard(w http.ResponseWriter, snapshots []*archive.Snapshot, query string) {
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>TimeCapsule &bull; Personal Wayback Machine</title>
<style>
:root { --bg: #0d1117; --card: #161b22; --border: #30363d; --text: #c9d1d9; --accent: #58a6ff; --green: #238636; }
body { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
.container { max-width: 1000px; margin: 40px auto; padding: 0 20px; }
header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 30px; border-bottom: 1px solid var(--border); padding-bottom: 20px; }
h1 { margin: 0; font-size: 26px; color: #f0f6fc; display: flex; align-items: center; gap: 10px; }
.stats { font-size: 14px; color: #8b949e; }
.capture-box { display: flex; gap: 10px; margin-bottom: 30px; }
input[type="text"] { flex: 1; background: var(--card); border: 1px solid var(--border); color: #fff; padding: 12px 16px; border-radius: 6px; font-size: 15px; }
input[type="text"]:focus { outline: 1px solid var(--accent); }
button { background: var(--green); color: #fff; border: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; cursor: pointer; }
button:hover { opacity: 0.9; }
.card { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 18px 24px; margin-bottom: 16px; transition: transform 0.1s; }
.card:hover { border-color: #58a6ff; }
.card-header { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 8px; }
.card-title { font-size: 18px; font-weight: 600; color: #58a6ff; text-decoration: none; }
.card-time { font-size: 13px; color: #8b949e; }
.card-url { font-size: 13px; color: #8b949e; word-break: break-all; margin-bottom: 12px; }
.card-actions { display: flex; gap: 12px; font-size: 13px; flex-wrap: wrap; }
.card-actions a { color: #8b949e; text-decoration: none; padding: 4px 8px; border-radius: 4px; background: #21262d; }
.card-actions a:hover { color: #fff; background: #30363d; }
.card-actions a.diff-btn { color: #58a6ff; border: 1px solid #1f6feb44; }
.card-actions a.diff-btn:hover { background: #1f6feb22; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; background: #1f6feb22; color: #58a6ff; border: 1px solid #1f6feb; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>⏳ TimeCapsule</h1>
    <div class="stats">%d Snapshots Preserved</div>
  </header>
  <form class="capture-box" onsubmit="event.preventDefault(); captureURL();">
    <input type="text" id="targetURL" placeholder="Enter URL to preserve (e.g. https://news.ycombinator.com)..." required>
    <button type="submit">Preserve Page</button>
  </form>
  <div style="margin-bottom: 20px;">
    <form method="GET" action="/">
      <input type="text" name="q" value="%s" placeholder="Search preserved snapshots by title, URL, or content..." style="width: calc(100%% - 32px);">
    </form>
  </div>
  <div id="snapshotList">`, len(snapshots), html.EscapeString(query))

	if len(snapshots) == 0 {
		fmt.Fprintf(w, `<div style="text-align:center;padding:50px;color:#8b949e;">No snapshots yet. Enter a URL above to preserve your first page!</div>`)
	} else {
		for _, snap := range snapshots {
			// Find if there is an earlier snapshot for comparison
			history := s.store.GetHistory(snap.URL)
			var prevSnapID string
			for i, h := range history {
				if h.ID == snap.ID && i > 0 {
					prevSnapID = history[i-1].ID
					break
				}
			}

			diffBtn := ""
			if prevSnapID != "" {
				diffBtn = fmt.Sprintf(`<a class="diff-btn" href="/diff/%s/%s" target="_blank">Compare with Prev ⇄</a>`, prevSnapID, snap.ID)
			}

			fmt.Fprintf(w, `
    <div class="card">
      <div class="card-header">
        <a class="card-title" href="/web/%s/%s">%s</a>
        <span class="card-time">%s</span>
      </div>
      <div class="card-url">%s &bull; <span class="badge">%d bytes</span> &bull; <span class="badge">%d assets</span></div>
      <div class="card-actions">
        <a href="/web/%s/%s" target="_blank">Replay in Wayback ⏳</a>
        %s
        <a href="/reader/%s" target="_blank">Reader Mode 📖</a>
        <a href="/raw/%s" target="_blank">Raw HTML</a>
        <a href="/markdown/%s" target="_blank">Markdown View 📄</a>
        <a href="/warc/%s">Download WARC 📦</a>
      </div>
    </div>`,
				snap.WaybackTS, snap.URL, html.EscapeString(snap.Title),
				snap.Timestamp.Format("Jan 02, 2006 15:04:05 UTC"),
				html.EscapeString(snap.URL), snap.SizeBytes, snap.AssetCount,
				snap.WaybackTS, snap.URL,
				diffBtn,
				snap.ID, snap.ID, snap.ID, snap.ID)
		}
	}

	fmt.Fprintf(w, `
  </div>
</div>
<script>
async function captureURL() {
  const url = document.getElementById('targetURL').value;
  if (!url) return;
  const btn = document.querySelector('button');
  btn.textContent = 'Archiving...';
  btn.disabled = true;
  try {
    const res = await fetch('/api/capture?url=' + encodeURIComponent(url));
    const data = await res.json();
    if (data.error) {
      alert('Error: ' + data.error);
    } else {
      window.location.reload();
    }
  } catch(e) {
    alert('Capture failed: ' + e);
  } finally {
    btn.textContent = 'Preserve Page';
    btn.disabled = false;
  }
}
</script>
</body>
</html>`)
}
