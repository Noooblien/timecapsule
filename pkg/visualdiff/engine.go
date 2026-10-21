package visualdiff

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// Engine performs hierarchical visual and structural DOM comparisons.
type Engine struct {
	config VisualDiffConfig
}

// NewEngine creates a visual diff engine with the specified configuration.
func NewEngine(cfg VisualDiffConfig) *Engine {
	return &Engine{config: cfg}
}

// CompareSnapshots analyzes structural and visual mutations between two snapshots.
func (e *Engine) CompareSnapshots(oldSnap, newSnap *archive.Snapshot) *VisualDiffResult {
	start := time.Now()

	oldRoot := ParseHTML(oldSnap.HTML, e.config)
	newRoot := ParseHTML(newSnap.HTML, e.config)

	oldNodeMap := make(map[string]*DOMNode)
	newNodeMap := make(map[string]*DOMNode)
	flattenTree(oldRoot, oldNodeMap)
	flattenTree(newRoot, newNodeMap)

	var mutations []DOMMutation
	impactedMap := make(map[SemanticBlockType]bool)

	addedCount := 0
	removedCount := 0
	modifiedCount := 0
	matchedCount := 0

	// 1. Detect Removed & Modified Nodes
	for path, oldNode := range oldNodeMap {
		if path == "document" {
			continue
		}
		newNode, exists := newNodeMap[path]
		if !exists {
			// Node was removed
			removedCount++
			impactedMap[oldNode.SemanticBlock] = true
			mutations = append(mutations, DOMMutation{
				Type:          MutationNodeRemoved,
				Severity:      classifyElementSeverity(oldNode),
				Path:          path,
				Tag:           oldNode.Tag,
				OldValue:      summarizeNode(oldNode),
				SemanticBlock: oldNode.SemanticBlock,
				Explanation:   fmt.Sprintf("Element <%s> deleted from %s", oldNode.Tag, oldNode.SemanticBlock),
			})
			continue
		}

		matchedCount++

		// Check if subtree hashes match exactly
		if oldNode.SubtreeHash == newNode.SubtreeHash {
			continue
		}

		// Text modification check
		if oldNode.TextContent != newNode.TextContent && (oldNode.TextContent != "" || newNode.TextContent != "") {
			textDelta := computeTextDeltaRatio(oldNode.TextContent, newNode.TextContent)
			if textDelta >= e.config.MinTextDelta {
				modifiedCount++
				impactedMap[newNode.SemanticBlock] = true
				mutations = append(mutations, DOMMutation{
					Type:          MutationTextModified,
					Severity:      classifyTextSeverity(newNode.SemanticBlock, textDelta),
					Path:          path,
					Tag:           newNode.Tag,
					OldValue:      truncateText(oldNode.TextContent, 60),
					NewValue:      truncateText(newNode.TextContent, 60),
					SemanticBlock: newNode.SemanticBlock,
					Explanation:   fmt.Sprintf("Text mutated in %s: %q -> %q", newNode.SemanticBlock, truncateText(oldNode.TextContent, 25), truncateText(newNode.TextContent, 25)),
				})
			}
		}

		// Attribute modification check
		attrDiffs := diffAttributes(oldNode.Attributes, newNode.Attributes, e.config.IgnoreClasses)
		if len(attrDiffs) > 0 {
			modifiedCount++
			impactedMap[newNode.SemanticBlock] = true
			for _, ad := range attrDiffs {
				mutations = append(mutations, DOMMutation{
					Type:          MutationAttrModified,
					Severity:      SeverityMinor,
					Path:          path,
					Tag:           newNode.Tag,
					OldValue:      ad.OldVal,
					NewValue:      ad.NewVal,
					SemanticBlock: newNode.SemanticBlock,
					Explanation:   fmt.Sprintf("Attribute @%s updated on <%s>: %q -> %q", ad.Key, newNode.Tag, ad.OldVal, ad.NewVal),
				})
			}
		}
	}

	// 2. Detect Added Nodes
	for path, newNode := range newNodeMap {
		if path == "document" {
			continue
		}
		if _, exists := oldNodeMap[path]; !exists {
			addedCount++
			impactedMap[newNode.SemanticBlock] = true
			mutations = append(mutations, DOMMutation{
				Type:          MutationNodeAdded,
				Severity:      classifyElementSeverity(newNode),
				Path:          path,
				Tag:           newNode.Tag,
				NewValue:      summarizeNode(newNode),
				SemanticBlock: newNode.SemanticBlock,
				Explanation:   fmt.Sprintf("New element <%s> added to %s", newNode.Tag, newNode.SemanticBlock),
			})
		}
	}

	// Calculate metrics
	totalNodesOld := len(oldNodeMap) - 1 // minus document root
	totalNodesNew := len(newNodeMap) - 1
	if totalNodesOld < 0 {
		totalNodesOld = 0
	}
	if totalNodesNew < 0 {
		totalNodesNew = 0
	}

	var structuralSimilarity float64
	var visualDriftScore float64

	maxNodes := math.Max(float64(totalNodesOld), float64(totalNodesNew))
	if maxNodes == 0 {
		structuralSimilarity = 1.0
		visualDriftScore = 0.0
	} else {
		// Similarity based on unmutated matched nodes
		unmutatedMatches := matchedCount - modifiedCount
		if unmutatedMatches < 0 {
			unmutatedMatches = 0
		}
		structuralSimilarity = float64(unmutatedMatches) / maxNodes
		if structuralSimilarity > 1.0 {
			structuralSimilarity = 1.0
		}

		// Drift score = (added + removed + 0.5*modified) / maxNodes * 100
		driftUnits := float64(addedCount+removedCount) + 0.5*float64(modifiedCount)
		visualDriftScore = math.Min(100.0, (driftUnits/maxNodes)*100.0)
	}

	severity := SeverityCosmetic
	switch {
	case visualDriftScore >= 50.0:
		severity = SeverityBreaking
	case visualDriftScore >= 20.0:
		severity = SeveritySubstantial
	case visualDriftScore >= 5.0:
		severity = SeverityMinor
	default:
		severity = SeverityCosmetic
	}

	var impactedBlocks []SemanticBlockType
	for b := range impactedMap {
		impactedBlocks = append(impactedBlocks, b)
	}

	return &VisualDiffResult{
		OldSnapshotID:        oldSnap.ID,
		NewSnapshotID:        newSnap.ID,
		URL:                  newSnap.URL,
		OldTimestamp:         oldSnap.Timestamp,
		NewTimestamp:         newSnap.Timestamp,
		TotalNodesOld:        totalNodesOld,
		TotalNodesNew:        totalNodesNew,
		AddedNodes:           addedCount,
		RemovedNodes:         removedCount,
		ModifiedNodes:        modifiedCount,
		StructuralSimilarity: structuralSimilarity,
		VisualDriftScore:     visualDriftScore,
		Severity:             severity,
		ImpactedBlocks:       impactedBlocks,
		Mutations:            mutations,
		DurationMs:           time.Since(start).Milliseconds(),
	}
}

type attrDelta struct {
	Key    string
	OldVal string
	NewVal string
}

func diffAttributes(oldAttrs, newAttrs map[string]string, ignoreClasses []string) []attrDelta {
	var deltas []attrDelta
	for k, oldV := range oldAttrs {
		if k == "class" && len(ignoreClasses) > 0 {
			continue
		}
		newV, exists := newAttrs[k]
		if !exists {
			deltas = append(deltas, attrDelta{Key: k, OldVal: oldV, NewVal: ""})
		} else if oldV != newV {
			deltas = append(deltas, attrDelta{Key: k, OldVal: oldV, NewVal: newV})
		}
	}
	for k, newV := range newAttrs {
		if _, exists := oldAttrs[k]; !exists {
			deltas = append(deltas, attrDelta{Key: k, OldVal: "", NewVal: newV})
		}
	}
	return deltas
}

func flattenTree(node *DOMNode, out map[string]*DOMNode) {
	out[node.Path] = node
	for _, child := range node.Children {
		flattenTree(child, out)
	}
}

func computeTextDeltaRatio(a, b string) float64 {
	if a == b {
		return 0.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 1.0
	}
	runesA := []rune(a)
	runesB := []rune(b)
	mismatch := 0
	minLen := len(runesA)
	if len(runesB) < minLen {
		minLen = len(runesB)
	}
	for i := 0; i < minLen; i++ {
		if runesA[i] != runesB[i] {
			mismatch++
		}
	}
	if len(runesA) > minLen {
		mismatch += len(runesA) - minLen
	} else if len(runesB) > minLen {
		mismatch += len(runesB) - minLen
	}
	maxLen := len(runesA)
	if len(runesB) > maxLen {
		maxLen = len(runesB)
	}
	return float64(mismatch) / float64(maxLen)
}

func classifyElementSeverity(node *DOMNode) DriftSeverity {
	switch node.SemanticBlock {
	case BlockPricing, BlockHero, BlockCTA:
		return SeveritySubstantial
	case BlockHeader, BlockNavigation:
		return SeverityMinor
	default:
		if node.Tag == "h1" || node.Tag == "h2" {
			return SeverityMinor
		}
		return SeverityCosmetic
	}
}

func classifyTextSeverity(block SemanticBlockType, delta float64) DriftSeverity {
	if block == BlockPricing {
		return SeveritySubstantial
	}
	if delta > 0.5 {
		return SeverityMinor
	}
	return SeverityCosmetic
}

func summarizeNode(node *DOMNode) string {
	if node.TextContent != "" {
		return fmt.Sprintf("<%s>: %s", node.Tag, truncateText(node.TextContent, 40))
	}
	return fmt.Sprintf("<%s>", node.Tag)
}

func truncateText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
