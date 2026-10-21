package visualdiff

import (
	"time"
)

// SemanticBlockType classifies functional regions of a web page.
type SemanticBlockType string

const (
	BlockHeader     SemanticBlockType = "HEADER"
	BlockNavigation SemanticBlockType = "NAVIGATION"
	BlockHero       SemanticBlockType = "HERO_BANNER"
	BlockMain       SemanticBlockType = "MAIN_CONTENT"
	BlockPricing    SemanticBlockType = "PRICING_TABLE"
	BlockCTA        SemanticBlockType = "CALL_TO_ACTION"
	BlockSidebar    SemanticBlockType = "SIDEBAR"
	BlockFooter     SemanticBlockType = "FOOTER"
	BlockGeneral    SemanticBlockType = "GENERAL_SECTION"
)

// MutationType classifies DOM hierarchy alterations.
type MutationType string

const (
	MutationNodeAdded    MutationType = "NODE_ADDED"
	MutationNodeRemoved  MutationType = "NODE_REMOVED"
	MutationTextModified MutationType = "TEXT_MODIFIED"
	MutationAttrModified MutationType = "ATTR_MODIFIED"
	MutationLayoutShift  MutationType = "LAYOUT_SHIFT"
)

// DriftSeverity rates the overall impact of detected visual changes.
type DriftSeverity string

const (
	SeverityCosmetic    DriftSeverity = "COSMETIC"    // < 5% drift
	SeverityMinor       DriftSeverity = "MINOR"       // 5% - 20% drift
	SeveritySubstantial DriftSeverity = "SUBSTANTIAL" // 20% - 50% drift
	SeverityBreaking    DriftSeverity = "BREAKING"    // > 50% drift
)

// DOMNode represents an element in the parsed HTML tree.
type DOMNode struct {
	ID            string            `json:"id"`
	Tag           string            `json:"tag"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	TextContent   string            `json:"text_content,omitempty"`
	Children      []*DOMNode        `json:"children,omitempty"`
	Depth         int               `json:"depth"`
	Path          string            `json:"path"`
	SemanticBlock SemanticBlockType `json:"semantic_block"`
	SubtreeHash   string            `json:"subtree_hash"`
}

// DOMMutation represents a localized change between two DOM trees.
type DOMMutation struct {
	Type          MutationType      `json:"type"`
	Severity      DriftSeverity     `json:"severity"`
	Path          string            `json:"path"`
	Tag           string            `json:"tag"`
	OldValue      string            `json:"old_value,omitempty"`
	NewValue      string            `json:"new_value,omitempty"`
	SemanticBlock SemanticBlockType `json:"semantic_block"`
	Explanation   string            `json:"explanation"`
}

// VisualDiffConfig customizes the DOM comparison engine.
type VisualDiffConfig struct {
	IgnoreScripts bool     `json:"ignore_scripts"`
	IgnoreStyles  bool     `json:"ignore_styles"`
	IgnoreClasses []string `json:"ignore_classes"`
	MinTextDelta  float64  `json:"min_text_delta"` // Minimum character difference to flag modification
}

// DefaultVisualDiffConfig returns standard settings.
func DefaultVisualDiffConfig() VisualDiffConfig {
	return VisualDiffConfig{
		IgnoreScripts: true,
		IgnoreStyles:  true,
		IgnoreClasses: []string{"active", "hover", "focus"},
		MinTextDelta:  0.05,
	}
}

// VisualDiffResult encapsulates the complete visual & structural diff analysis.
type VisualDiffResult struct {
	OldSnapshotID        string              `json:"old_snapshot_id"`
	NewSnapshotID        string              `json:"new_snapshot_id"`
	URL                  string              `json:"url"`
	OldTimestamp         time.Time           `json:"old_timestamp"`
	NewTimestamp         time.Time           `json:"new_timestamp"`
	TotalNodesOld        int                 `json:"total_nodes_old"`
	TotalNodesNew        int                 `json:"total_nodes_new"`
	AddedNodes           int                 `json:"added_nodes"`
	RemovedNodes         int                 `json:"removed_nodes"`
	ModifiedNodes        int                 `json:"modified_nodes"`
	StructuralSimilarity float64             `json:"structural_similarity"` // 0.0 to 1.0
	VisualDriftScore     float64             `json:"visual_drift_score"`    // 0.0 to 100.0
	Severity             DriftSeverity       `json:"severity"`
	ImpactedBlocks       []SemanticBlockType `json:"impacted_blocks"`
	Mutations            []DOMMutation       `json:"mutations"`
	DurationMs           int64               `json:"duration_ms"`
}
