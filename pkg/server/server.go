package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/storage"
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
	s.mux.HandleFunc("/web/", s.handleWaybackReplay)
	s.mux.HandleFunc("/raw/", s.handleRaw)
	s.mux.HandleFunc("/markdown/", s.handleMarkdown)
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
	renderDashboard(w, snapshots, query)
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

func (s *Server) handleWARC(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/warc/")
	snap, ok := s.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	warc := archive.GenerateWARC(snap)
	w.Header().Set("Content-Type", "application/warc")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.warc", snap.ID))
	_, _ = w.Write(warc)
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

func renderDashboard(w http.ResponseWriter, snapshots []*archive.Snapshot, query string) {
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
.card-actions { display: flex; gap: 12px; font-size: 13px; }
.card-actions a { color: #8b949e; text-decoration: none; padding: 4px 8px; border-radius: 4px; background: #21262d; }
.card-actions a:hover { color: #fff; background: #30363d; }
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
		for _, s := range snapshots {
			fmt.Fprintf(w, `
    <div class="card">
      <div class="card-header">
        <a class="card-title" href="/web/%s/%s">%s</a>
        <span class="card-time">%s</span>
      </div>
      <div class="card-url">%s &bull; <span class="badge">%d bytes</span> &bull; <span class="badge">%d assets</span></div>
      <div class="card-actions">
        <a href="/web/%s/%s" target="_blank">Replay in Wayback ⏳</a>
        <a href="/raw/%s" target="_blank">Raw HTML</a>
        <a href="/markdown/%s" target="_blank">Markdown View 📄</a>
        <a href="/warc/%s">Download WARC 📦</a>
      </div>
    </div>`,
				s.WaybackTS, s.URL, html.EscapeString(s.Title),
				s.Timestamp.Format("Jan 02, 2006 15:04:05 UTC"),
				html.EscapeString(s.URL), s.SizeBytes, s.AssetCount,
				s.WaybackTS, s.URL,
				s.ID, s.ID, s.ID)
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
