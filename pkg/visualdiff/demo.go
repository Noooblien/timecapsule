package visualdiff

import (
	"fmt"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// RunDemo executes visual diff comparisons across 4 production scenarios.
func RunDemo(mermaid, markdown bool) (string, error) {
	engine := NewEngine(DefaultVisualDiffConfig())

	scenarios := []struct {
		Name     string
		OldSnap  *archive.Snapshot
		NewSnap  *archive.Snapshot
		Expected DriftSeverity
	}{
		{
			Name:     "Scenario 1: Clean Baseline (No Drift)",
			OldSnap:  makeDemoSnapshot("snap_clean_v1", "https://example.com/blog", demoHTMLCleanV1, time.Now().Add(-24*time.Hour)),
			NewSnap:  makeDemoSnapshot("snap_clean_v2", "https://example.com/blog", demoHTMLCleanV1, time.Now()),
			Expected: SeverityCosmetic,
		},
		{
			Name:     "Scenario 2: SaaS Pricing Page Overhaul",
			OldSnap:  makeDemoSnapshot("snap_pricing_v1", "https://saas.io/pricing", demoHTMLPricingV1, time.Now().Add(-48*time.Hour)),
			NewSnap:  makeDemoSnapshot("snap_pricing_v2", "https://saas.io/pricing", demoHTMLPricingV2, time.Now()),
			Expected: SeveritySubstantial,
		},
		{
			Name:     "Scenario 3: Hero Banner & CTA Redesign",
			OldSnap:  makeDemoSnapshot("snap_hero_v1", "https://startup.com", demoHTMLHeroV1, time.Now().Add(-72*time.Hour)),
			NewSnap:  makeDemoSnapshot("snap_hero_v2", "https://startup.com", demoHTMLHeroV2, time.Now()),
			Expected: SeverityMinor,
		},
		{
			Name:     "Scenario 4: Navigation Restructuring & Footer Drift",
			OldSnap:  makeDemoSnapshot("snap_nav_v1", "https://platform.dev", demoHTMLNavV1, time.Now().Add(-96*time.Hour)),
			NewSnap:  makeDemoSnapshot("snap_nav_v2", "https://platform.dev", demoHTMLNavV2, time.Now()),
			Expected: SeverityMinor,
		},
	}

	var out strings.Builder
	for i, sc := range scenarios {
		res := engine.CompareSnapshots(sc.OldSnap, sc.NewSnap)

		if markdown {
			out.WriteString(fmt.Sprintf("### %s\n\n", sc.Name))
			out.WriteString(RenderMarkdownReport(res))
			out.WriteString("\n\n")
		} else {
			out.WriteString(fmt.Sprintf("%s=== [Demo %d/4] %s ===%s\n", ansiBold, i+1, sc.Name, ansiReset))
			out.WriteString(RenderVisualScorecard(res))
			out.WriteString("\n")
		}

		if mermaid {
			out.WriteString(RenderMermaidDOMTree(res))
			out.WriteString("\n\n")
		}
	}

	return out.String(), nil
}

func makeDemoSnapshot(id, urlStr, htmlContent string, ts time.Time) *archive.Snapshot {
	return &archive.Snapshot{
		ID:          id,
		URL:         urlStr,
		Timestamp:   ts,
		WaybackTS:   archive.FormatWaybackTS(ts),
		Title:       archive.ExtractTitle(htmlContent, urlStr),
		HTML:        htmlContent,
		ContentHash: archive.ComputeHash([]byte(htmlContent)),
		SizeBytes:   int64(len(htmlContent)),
	}
}

const demoHTMLCleanV1 = `<!DOCTYPE html>
<html>
<head><title>Engineering Blog</title></head>
<body>
  <header><h1>Tech Insights</h1><nav><a href="/">Home</a><a href="/about">About</a></nav></header>
  <main>
    <article>
      <h2>Zero-Downtime Migration</h2>
      <p>Continuous database migrations using shadow tables and change data capture.</p>
    </article>
  </main>
  <footer><p>© 2026 Tech Insights</p></footer>
</body>
</html>`

const demoHTMLPricingV1 = `<!DOCTYPE html>
<html>
<head><title>CloudScale Pricing</title></head>
<body>
  <header><nav><a href="/">Home</a></nav></header>
  <main>
    <section id="pricing" class="pricing-grid">
      <div id="plan-starter" class="tier-card">
        <h3>Starter</h3>
        <p class="price">$19/mo</p>
        <p>Up to 10k monthly active users.</p>
        <button class="cta-btn">Select Starter</button>
      </div>
      <div id="plan-pro" class="tier-card">
        <h3>Pro</h3>
        <p class="price">$49/mo</p>
        <p>Unlimited users and dedicated VPC.</p>
        <button class="cta-btn">Select Pro</button>
      </div>
    </section>
  </main>
</body>
</html>`

const demoHTMLPricingV2 = `<!DOCTYPE html>
<html>
<head><title>CloudScale Pricing</title></head>
<body>
  <header><nav><a href="/">Home</a></nav></header>
  <main>
    <section id="pricing" class="pricing-grid">
      <div id="plan-starter" class="tier-card">
        <h3>Starter</h3>
        <p class="price">$29/mo</p>
        <p>Up to 10k monthly active users.</p>
        <button class="cta-btn">Select Starter</button>
      </div>
      <div id="plan-pro" class="tier-card">
        <h3>Pro</h3>
        <p class="price">$79/mo</p>
        <p>Unlimited users and dedicated VPC.</p>
        <button class="cta-btn">Select Pro</button>
      </div>
      <div id="plan-enterprise" class="tier-card">
        <h3>Enterprise</h3>
        <p class="price">Custom</p>
        <p>SLA guarantees and 24/7 support.</p>
        <button class="cta-btn">Contact Sales</button>
      </div>
    </section>
  </main>
</body>
</html>`

const demoHTMLHeroV1 = `<!DOCTYPE html>
<html>
<head><title>Autonomous Agent Studio</title></head>
<body>
  <section id="hero" class="hero-section">
    <h1>Build Agents in Seconds</h1>
    <p>Zero configuration local runtime with unified tool protocols.</p>
    <a href="/download" class="cta-button">Download Free</a>
  </section>
</body>
</html>`

const demoHTMLHeroV2 = `<!DOCTYPE html>
<html>
<head><title>Autonomous Agent Studio</title></head>
<body>
  <section id="hero" class="hero-section">
    <h1>Autonomous Agent Operating System</h1>
    <p>Enterprise orchestration with deterministic flight recording.</p>
    <a href="/trial" class="cta-button">Start Free 14-Day Trial</a>
    <a href="/demo" class="secondary-btn">Watch Interactive Demo</a>
  </section>
</body>
</html>`

const demoHTMLNavV1 = `<!DOCTYPE html>
<html>
<head><title>Platform Portal</title></head>
<body>
  <header>
    <nav class="nav-menu">
      <a href="/docs">Documentation</a>
      <a href="/pricing">Pricing</a>
      <a href="/login">Login</a>
    </nav>
  </header>
  <footer><p>© 2025 Old Brand</p></footer>
</body>
</html>`

const demoHTMLNavV2 = `<!DOCTYPE html>
<html>
<head><title>Platform Portal</title></head>
<body>
  <header>
    <nav class="nav-menu">
      <a href="/docs">Documentation</a>
      <a href="/pricing">Pricing</a>
      <a href="/changelog">Changelog</a>
      <a href="/login">Sign In</a>
    </nav>
  </header>
  <footer><p>© 2026 New Brand Corp</p></footer>
</body>
</html>`
