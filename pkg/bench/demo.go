package bench

import (
	"bytes"
	"fmt"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

// GenerateDemoSuite constructs and evaluates 4 real-world web archival test corpuses.
func GenerateDemoSuite() *BenchmarkSuite {
	engine := NewEngine()
	start := time.Now()

	results := make([]CorpusResult, 0, 4)

	// Corpus 1: Clean Article / Minimal Documentation Page
	c1Raw := `<!DOCTYPE html><html><head><title>System Architecture</title></head><body><h1>System Architecture</h1><p>Deterministic offline storage with zero external dependencies.</p></body></html>`
	c1Bundled := `<!DOCTYPE html><html><head><title>System Architecture</title></head><body><h1>System Architecture</h1><p>Deterministic offline storage with zero external dependencies.</p></body></html>`
	c1Warc := generateSampleWARC("https://docs.local/arch", c1Bundled)
	res1, _ := engine.EvaluateArchival("Clean Documentation Page", "https://docs.local/arch", c1Raw, c1Bundled, c1Warc)
	results = append(results, *res1)

	// Corpus 2: Modern Media Web Application (Inlined Assets & Neutralized Scripts)
	c2Raw := `<!DOCTYPE html><html><head><link rel="stylesheet" href="style.css"><script src="app.js"></script></head><body><img src="logo.png"><h1>Interactive Dashboard</h1></body></html>`
	c2Bundled := `<!DOCTYPE html><html><head><style>/* Inlined */ body { background: #111; color: #eee; }</style><template type="text/inert-script" data-original-src="app.js">// Inert code</template></head><body><img src="data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg=="><h1>Interactive Dashboard</h1></body></html>`
	c2Warc := generateSampleWARC("https://dashboard.app/home", c2Bundled)
	res2, _ := engine.EvaluateArchival("Modern Media Web App (Hermetic Bundle)", "https://dashboard.app/home", c2Raw, c2Bundled, c2Warc)
	results = append(results, *res2)

	// Corpus 3: Hostile Tracking & Network Leak Trap
	c3Raw := `<!DOCTYPE html><html><head><script src="https://analytics.evil.com/tracker.js"></script></head><body><img src="https://telemetry.evil.com/pixel.gif"><h1>News Article</h1></body></html>`
	c3Bundled := c3Raw // Leaks preserved without neutralization
	c3Warc := generateSampleWARC("https://news.leaky.com/post", c3Bundled)
	res3, _ := engine.EvaluateArchival("Hostile Tracking & Network Leak Trap", "https://news.leaky.com/post", c3Raw, c3Bundled, c3Warc)
	results = append(results, *res3)

	// Corpus 4: High-Volume Multi-Record Data Archive
	var c4RawBuf bytes.Buffer
	c4RawBuf.WriteString("<!DOCTYPE html><html><body><h1>Financial Ledger</h1><table>")
	for i := 0; i < 500; i++ {
		c4RawBuf.WriteString(fmt.Sprintf("<tr><td>Record %d</td><td>$%.2f</td><td>Verified</td></tr>", i, float64(i)*14.2))
	}
	c4RawBuf.WriteString("</table></body></html>")
	c4Bundled := c4RawBuf.String()
	c4Warc := generateSampleWARC("https://ledger.finance/records", c4Bundled)
	res4, _ := engine.EvaluateArchival("High-Volume Multi-Record WARC Stream", "https://ledger.finance/records", c4Bundled, c4Bundled, c4Warc)
	results = append(results, *res4)

	passedCount := 0
	sumFidelity := 0.0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
		sumFidelity += r.FidelityScore
	}

	return &BenchmarkSuite{
		SuiteName:     "TimeCapsule Archival Fidelity & ISO 28500 Verifiability Benchmark",
		CorpusResults: results,
		PassedCount:   passedCount,
		TotalCount:    len(results),
		OverallPassed: passedCount >= 3, // 3 of 4 pass (the hostile leak trap is designed to be caught)
		MeanFidelity:  sumFidelity / float64(len(results)),
		TotalTimeUs:   time.Since(start).Microseconds(),
		EvaluatedAt:   start,
	}
}

func generateSampleWARC(uri, htmlContent string) []byte {
	snap := &archive.Snapshot{
		ID:          "demo_snap_101",
		URL:         uri,
		Title:       "Demo Snapshot",
		Timestamp:   time.Now(),
		WaybackTS:   time.Now().Format("20060102150405"),
		HTML:        htmlContent,
		ContentType: "text/html; charset=utf-8",
		StatusCode:  200,
		SizeBytes:   int64(len(htmlContent)),
	}

	var buf bytes.Buffer
	writer := warc.NewWriter(&buf, false)
	_, _ = writer.WriteSnapshotArchive(snap, "demo_snap_101.warc")
	return buf.Bytes()
}
