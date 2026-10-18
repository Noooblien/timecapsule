package watcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

// RunWatcherDemo executes an end-to-end multi-target demonstration across 4 realistic scenarios.
func RunWatcherDemo(renderMermaid, renderMarkdown bool) (string, error) {
	tempDir, err := os.MkdirTemp("", "timecapsule_watcher_demo_*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewStore(tempDir)
	if err != nil {
		return "", err
	}

	// State for dynamic mock server
	var stateMu sync.Mutex
	scenarioTick := 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stateMu.Lock()
		tick := scenarioTick
		stateMu.Unlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/terms":
			if tick == 0 {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Terms of Service v1.0</title></head><body>
				<h1>Terms of Service</h1><p>We respect your privacy and will never sell your personal data to third parties.</p>
				<p>All disputes will be settled in civil courts of competent jurisdiction.</p>
				<!-- updated: 2026-01-01 --></body></html>`)
			} else {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Terms of Service v2.0</title></head><body>
				<h1>Terms of Service</h1><p>We reserve the right to monetize and license aggregated user behavioral telemetry to advertising partners.</p>
				<p>Mandatory binding individual arbitration applies. Class actions are strictly waived.</p>
				<!-- updated: 2026-10-18 --></body></html>`)
			}

		case "/pricing":
			if tick == 0 {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>SaaS Pricing</title></head><body>
				<h1>Plans</h1><div class="tier"><h3>Pro</h3><p>$29/month</p><ul><li>10,000 requests</li></ul></div>
				</body></html>`)
			} else {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>SaaS Pricing - Enterprise</title></head><body>
				<h1>Plans</h1><div class="tier"><h3>Pro</h3><p>$49/month</p><ul><li>25,000 requests</li></ul></div>
				<div class="tier"><h3>Enterprise</h3><p>$499/month</p><ul><li>Unlimited requests</li><li>SLA 99.99%</li></ul></div>
				</body></html>`)
			}

		case "/docs":
			if tick == 0 {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>API Reference</title></head><body>
				<h1>Authentication</h1><p>Send your bearer token in the Authorizaton header.</p>
				</body></html>`)
			} else {
				fmt.Fprint(w, `<!DOCTYPE html><html><head><title>API Reference</title></head><body>
				<h1>Authentication</h1><p>Send your bearer token in the Authorization header.</p>
				</body></html>`)
			}

		case "/news":
			// Contains dynamic timestamps and ad banner that change every tick, but main content is static
			nonce := fmt.Sprintf("nonce_%d", time.Now().UnixNano())
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Tech Daily</title></head><body>
			<header><span class="timestamp">Published at: %s</span></header>
			<div class="ad-banner" id="google-ads"><p>Special Discount %s</p></div>
			<article><h1>Open Standards in Web Archiving</h1><p>ISO 28500 defines the WARC file format for long-term digital preservation.</p></article>
			</body></html>`, time.Now().Format(time.RFC3339), nonce)

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	archiver := archive.NewArchiver(5 * time.Second)
	engine, err := NewEngine(store, archiver, tempDir)
	if err != nil {
		return "", err
	}

	// Register 4 targets
	tTerms, _ := engine.AddTarget(mockServer.URL+"/terms", "Terms of Service", "1m", "", 0.05, nil, []string{"legal", "compliance"})
	tPricing, _ := engine.AddTarget(mockServer.URL+"/pricing", "Pricing Table", "5m", "", 0.05, nil, []string{"pricing", "business"})
	tDocs, _ := engine.AddTarget(mockServer.URL+"/docs", "API Documentation", "10m", "", 0.01, nil, []string{"docs"})
	tNews, _ := engine.AddTarget(mockServer.URL+"/news", "Tech Daily News", "2m", "", 0.05, []string{"ad-banner", "timestamp"}, []string{"news"})

	ctx := context.Background()

	// Tick 0: Initial Baseline Captures
	stateMu.Lock()
	scenarioTick = 0
	stateMu.Unlock()

	_, _ = engine.PollTarget(ctx, tTerms.ID)
	_, _ = engine.PollTarget(ctx, tPricing.ID)
	_, _ = engine.PollTarget(ctx, tDocs.ID)
	_, _ = engine.PollTarget(ctx, tNews.ID)

	// Tick 1: Modifications occur
	stateMu.Lock()
	scenarioTick = 1
	stateMu.Unlock()

	time.Sleep(20 * time.Millisecond)
	_ = engine.PollAll(ctx)

	targets := engine.ListTargets()
	events := engine.GetEvents("")

	if renderMermaid {
		return RenderMermaidTimeline(events), nil
	}
	if renderMarkdown {
		return RenderMarkdownReport(targets, events), nil
	}

	return RenderWatcherScorecard(targets, events), nil
}
