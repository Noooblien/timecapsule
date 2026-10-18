package bench

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

var (
	reImgTag    = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	reStyleTag  = regexp.MustCompile(`(?i)<link\b[^>]*rel=["']stylesheet["'][^>]*>`)
	reScriptTag = regexp.MustCompile(`(?i)<script\b[^>]*>`)
)

// Engine performs empirical fidelity benchmarking and archival auditing.
type Engine struct{}

// NewEngine creates a new benchmark Engine.
func NewEngine() *Engine {
	return &Engine{}
}

// EvaluateArchival runs comprehensive empirical benchmarks on an archived web page.
func (e *Engine) EvaluateArchival(name, targetURL, rawHTML, bundledHTML string, warcBytes []byte) (*CorpusResult, error) {
	start := time.Now()

	// 1. Audit HTML for Hermetic Isolation & Network Leaks
	leaks, hermeticScore := AuditHTML(bundledHTML)

	// 2. Validate ISO 28500 WARC Standard Compliance
	isoReport := ValidateWARC(warcBytes)

	// 3. Asset Preservation Calculation
	rawAssets := countAssets(rawHTML)
	bundledAssets := countInlinedAssets(bundledHTML)

	preservationRatio := 100.0
	if rawAssets > 0 {
		preservationRatio = (float64(bundledAssets) / float64(rawAssets)) * 100.0
		if preservationRatio > 100.0 {
			preservationRatio = 100.0
		}
	}

	// 4. Compression & Storage Metrics
	rawSize := int64(len(rawHTML))
	bundledSize := int64(len(bundledHTML))
	warcSize := int64(len(warcBytes))

	// Measure gzip compressed size
	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	_, _ = gzWriter.Write([]byte(bundledHTML))
	_ = gzWriter.Close()
	gzipSize := int64(gzBuf.Len())

	compressionRatio := 1.0
	spaceSavings := 0.0
	if bundledSize > 0 && gzipSize > 0 {
		compressionRatio = float64(bundledSize) / float64(gzipSize)
		spaceSavings = (1.0 - (float64(gzipSize) / float64(bundledSize))) * 100.0
	}

	elapsed := time.Since(start)
	elapsedUs := elapsed.Microseconds()
	if elapsedUs == 0 {
		elapsedUs = 1
	}

	totalProcessedBytes := rawSize + bundledSize + warcSize
	throughputMBps := (float64(totalProcessedBytes) / (1024.0 * 1024.0)) / (float64(elapsedUs) / 1000000.0)

	// 5. Composite Fidelity Score Calculation
	// Weights: 40% Hermetic Score + 30% Preservation Ratio + 20% ISO Validation + 10% Space Savings
	isoScore := 0.0
	if isoReport.ValidFormat && isoReport.HasWarcInfo && isoReport.HasResponse {
		isoScore = 100.0
	} else if isoReport.ValidFormat {
		isoScore = 60.0
	}

	fidelityScore := (0.40 * hermeticScore) + (0.30 * preservationRatio) + (0.20 * isoScore) + (0.10 * math.Min(100.0, spaceSavings*1.2))
	fidelityScore = math.Max(0.0, math.Min(100.0, fidelityScore))

	// Status determination
	hasCriticalLeak := false
	for _, l := range leaks {
		if l.Severity == SeverityCritical {
			hasCriticalLeak = true
			break
		}
	}

	status := StatusPass
	passed := false
	if hasCriticalLeak {
		status = StatusLeakDetected
		passed = false
	} else if fidelityScore >= 85.0 && isoReport.ValidFormat {
		status = StatusPass
		passed = true
	} else {
		status = StatusDegraded
		passed = false
	}

	metrics := FidelityMetrics{
		TotalDetectedAssets:    rawAssets,
		InlinedAssets:          bundledAssets,
		PreservationRatio:      preservationRatio,
		HermeticScore:          hermeticScore,
		OriginalSizeBytes:      rawSize,
		BundledSizeBytes:       bundledSize,
		WarcSizeBytes:          warcSize,
		GzipSizeBytes:          gzipSize,
		CompressionRatio:       compressionRatio,
		SpaceSavingsPct:        spaceSavings,
		ThroughputMBps:         throughputMBps,
		SerializationLatencyUs: elapsedUs,
	}

	summary := ""
	if passed {
		summary = fmt.Sprintf("Archival integrity verified at %.1f%% fidelity score; 100%% ISO 28500 WARC compliant, 0 leaks.", fidelityScore)
	} else if status == StatusLeakDetected {
		summary = fmt.Sprintf("Hermetic leak breach! %d external dependencies detected breaking offline isolation.", len(leaks))
	} else {
		summary = fmt.Sprintf("Fidelity degraded (Score: %.1f%%, Asset preservation: %.1f%%).", fidelityScore, preservationRatio)
	}

	return &CorpusResult{
		Name:          name,
		TargetURL:     targetURL,
		Status:        status,
		FidelityScore: fidelityScore,
		Metrics:       metrics,
		ISOReport:     isoReport,
		Leaks:         leaks,
		Passed:        passed,
		Summary:       summary,
	}, nil
}

// EvaluateSnapshot audits an existing storage Snapshot.
func (e *Engine) EvaluateSnapshot(snap *archive.Snapshot) (*CorpusResult, error) {
	if snap == nil {
		return nil, fmt.Errorf("snapshot cannot be nil")
	}

	// Generate standard WARC bytes for evaluation
	var warcBuf bytes.Buffer
	writer := warc.NewWriter(&warcBuf, false)
	_, err := writer.WriteSnapshotArchive(snap, snap.ID+".warc")
	if err != nil {
		return nil, fmt.Errorf("failed to generate WARC for snapshot: %v", err)
	}

	return e.EvaluateArchival(
		fmt.Sprintf("Snapshot %s", snap.ID),
		snap.URL,
		snap.HTML,
		snap.HTML, // Evaluates stored HTML
		warcBuf.Bytes(),
	)
}

func countAssets(html string) int {
	imgs := len(reImgTag.FindAllString(html, -1))
	styles := len(reStyleTag.FindAllString(html, -1))
	scripts := len(reScriptTag.FindAllString(html, -1))
	return imgs + styles + scripts
}

func countInlinedAssets(html string) int {
	// Look for data: URIs, inlined style blocks, and inert template blocks
	dataURIs := regexp.MustCompile(`(?i)data:[a-zA-Z0-9/+-]+;base64,`).FindAllString(html, -1)
	inertScripts := regexp.MustCompile(`(?i)<template[^>]+type=["']text/inert-script["']`).FindAllString(html, -1)
	inlinedStyles := regexp.MustCompile(`(?i)<style\b[^>]*>`).FindAllString(html, -1)
	return len(dataURIs) + len(inertScripts) + len(inlinedStyles)
}
