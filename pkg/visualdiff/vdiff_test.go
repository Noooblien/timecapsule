package visualdiff

import (
	"sync"
	"testing"
	"time"
)

func TestDOMParser_CleanDocument(t *testing.T) {
	html := `<html><body><header id="top-nav"><nav><a href="/">Home</a></nav></header><main><section id="pricing" class="tier"><p>$10</p></section></main></body></html>`
	root := ParseHTML(html, DefaultVisualDiffConfig())

	if root == nil {
		t.Fatalf("expected non-nil root")
	}

	nodeMap := make(map[string]*DOMNode)
	flattenTree(root, nodeMap)

	if len(nodeMap) == 0 {
		t.Fatalf("expected parsed nodes, got 0")
	}

	pricingNode, exists := nodeMap["body > main:nth-child(2) > section#pricing"]
	if !exists {
		t.Fatalf("expected path 'body > main:nth-child(2) > section#pricing' to exist in nodeMap")
	}

	if pricingNode.SemanticBlock != BlockPricing {
		t.Errorf("expected BlockPricing, got %s", pricingNode.SemanticBlock)
	}
}

func TestVisualDiff_IdenticalDOM(t *testing.T) {
	engine := NewEngine(DefaultVisualDiffConfig())
	snapA := makeDemoSnapshot("snap_a", "https://example.com", demoHTMLCleanV1, time.Now().Add(-time.Hour))
	snapB := makeDemoSnapshot("snap_b", "https://example.com", demoHTMLCleanV1, time.Now())

	res := engine.CompareSnapshots(snapA, snapB)

	if res.VisualDriftScore != 0.0 {
		t.Errorf("expected 0.0 visual drift score, got %.2f", res.VisualDriftScore)
	}
	if res.StructuralSimilarity != 1.0 {
		t.Errorf("expected 1.0 structural similarity, got %.2f", res.StructuralSimilarity)
	}
	if len(res.Mutations) != 0 {
		t.Errorf("expected 0 mutations, got %d", len(res.Mutations))
	}
	if res.Severity != SeverityCosmetic {
		t.Errorf("expected SeverityCosmetic, got %s", res.Severity)
	}
}

func TestVisualDiff_PricingOverhaul(t *testing.T) {
	engine := NewEngine(DefaultVisualDiffConfig())
	snapA := makeDemoSnapshot("snap_p1", "https://saas.io/pricing", demoHTMLPricingV1, time.Now().Add(-time.Hour))
	snapB := makeDemoSnapshot("snap_p2", "https://saas.io/pricing", demoHTMLPricingV2, time.Now())

	res := engine.CompareSnapshots(snapA, snapB)

	if res.VisualDriftScore <= 0.0 {
		t.Errorf("expected positive visual drift score, got %.2f", res.VisualDriftScore)
	}
	if len(res.Mutations) == 0 {
		t.Fatalf("expected mutations for pricing overhaul, got 0")
	}

	// Verify pricing block was detected as impacted
	foundPricing := false
	for _, b := range res.ImpactedBlocks {
		if b == BlockPricing {
			foundPricing = true
			break
		}
	}
	if !foundPricing {
		t.Errorf("expected BlockPricing in ImpactedBlocks, got %v", res.ImpactedBlocks)
	}

	// Verify HTML render
	htmlOut := RenderVisualDiffHTML(res, snapA, snapB)
	if len(htmlOut) == 0 || !contains(htmlOut, "Temporal Visual & DOM Tree Diff") {
		t.Errorf("rendered HTML missing expected elements")
	}
}

func TestVisualDiff_HeroRedesign(t *testing.T) {
	engine := NewEngine(DefaultVisualDiffConfig())
	snapA := makeDemoSnapshot("snap_h1", "https://startup.com", demoHTMLHeroV1, time.Now().Add(-time.Hour))
	snapB := makeDemoSnapshot("snap_h2", "https://startup.com", demoHTMLHeroV2, time.Now())

	res := engine.CompareSnapshots(snapA, snapB)

	if len(res.Mutations) == 0 {
		t.Fatalf("expected mutations in hero redesign")
	}

	scorecard := RenderVisualScorecard(res)
	if !contains(scorecard, "TIMECAPSULE TEMPORAL VISUAL & DOM DIFF SCORECARD") {
		t.Errorf("scorecard missing header")
	}

	mermaid := RenderMermaidDOMTree(res)
	if !contains(mermaid, "flowchart TD") {
		t.Errorf("mermaid output missing flowchart TD")
	}

	markdown := RenderMarkdownReport(res)
	if !contains(markdown, "TimeCapsule Visual & DOM Diff Report") {
		t.Errorf("markdown report missing expected heading")
	}
}

func TestVisualDiff_Concurrency(t *testing.T) {
	engine := NewEngine(DefaultVisualDiffConfig())
	snapA := makeDemoSnapshot("snap_p1", "https://saas.io/pricing", demoHTMLPricingV1, time.Now().Add(-time.Hour))
	snapB := makeDemoSnapshot("snap_p2", "https://saas.io/pricing", demoHTMLPricingV2, time.Now())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := engine.CompareSnapshots(snapA, snapB)
			if res == nil || len(res.Mutations) == 0 {
				t.Errorf("concurrent run failed")
			}
		}()
	}
	wg.Wait()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
