package visualdiff

import (
	"fmt"
	"html"
	"strings"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// RenderVisualDiffHTML renders an interactive side-by-side dual-pane visual diff comparison.
func RenderVisualDiffHTML(res *VisualDiffResult, oldSnap, newSnap *archive.Snapshot) string {
	var blockPills strings.Builder
	for _, b := range res.ImpactedBlocks {
		blockPills.WriteString(fmt.Sprintf(`<span class="pill pill-block">%s</span> `, html.EscapeString(string(b))))
	}

	var mutationRows strings.Builder
	for i, m := range res.Mutations {
		badgeClass := "badge-info"
		switch m.Type {
		case MutationNodeAdded:
			badgeClass = "badge-added"
		case MutationNodeRemoved:
			badgeClass = "badge-removed"
		case MutationTextModified:
			badgeClass = "badge-modified"
		case MutationAttrModified:
			badgeClass = "badge-attr"
		}

		mutationRows.WriteString(fmt.Sprintf(`
      <tr>
        <td style="color:#8b949e;text-align:center;">%d</td>
        <td><span class="badge %s">%s</span></td>
        <td><code>%s</code></td>
        <td style="color:var(--accent);">%s</td>
        <td>%s</td>
      </tr>`,
			i+1, badgeClass, html.EscapeString(string(m.Type)),
			html.EscapeString(m.Path), html.EscapeString(string(m.SemanticBlock)),
			html.EscapeString(m.Explanation),
		))
	}

	severityColor := "#7ee787"
	switch res.Severity {
	case SeverityBreaking:
		severityColor = "#f85149"
	case SeveritySubstantial:
		severityColor = "#d29922"
	case SeverityMinor:
		severityColor = "#58a6ff"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Visual DOM Diff: %s ↔ %s</title>
<style>
  :root {
    --bg: #0d1117;
    --card: #161b22;
    --border: #30363d;
    --text: #c9d1d9;
    --accent: #58a6ff;
    --green: #2ea043;
    --red: #f85149;
    --yellow: #d29922;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 20px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg); color: var(--text); line-height: 1.5;
  }
  .container { max-width: 1400px; margin: 0 auto; }
  header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; border-bottom: 1px solid var(--border); padding-bottom: 16px; }
  h1 { font-size: 22px; margin: 0; display: flex; align-items: center; gap: 8px; }
  .back-link { color: var(--accent); text-decoration: none; font-size: 14px; font-weight: 500; }
  .back-link:hover { text-decoration: underline; }
  
  .hud-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; margin-bottom: 24px; }
  .hud-card { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 16px; }
  .hud-card h4 { margin: 0 0 6px 0; font-size: 13px; color: #8b949e; text-transform: uppercase; }
  .hud-card .metric { font-size: 26px; font-weight: 700; }

  .split-pane-header { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-bottom: 8px; }
  .pane-title { background: var(--card); border: 1px solid var(--border); padding: 12px 16px; border-radius: 6px; font-weight: 600; font-size: 14px; }
  
  .split-view { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; height: 500px; margin-bottom: 24px; }
  .pane-frame { width: 100%%; height: 100%%; border: 1px solid var(--border); border-radius: 6px; background: #fff; }

  .card-table { background: var(--card); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; margin-top: 24px; }
  .card-table h3 { margin: 16px; font-size: 16px; }
  table { width: 100%%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 10px 14px; text-align: left; border-bottom: 1px solid var(--border); }
  th { background: #21262d; color: #8b949e; font-size: 12px; text-transform: uppercase; }
  code { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 12px; background: rgba(110,118,129,0.2); padding: 2px 6px; border-radius: 4px; }

  .badge { display: inline-block; padding: 3px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; }
  .badge-added { background: rgba(46,160,67,0.2); color: #7ee787; border: 1px solid #2ea043; }
  .badge-removed { background: rgba(248,81,73,0.2); color: #ff7b72; border: 1px solid #f85149; }
  .badge-modified { background: rgba(210,153,34,0.2); color: #d29922; border: 1px solid #d29922; }
  .badge-attr { background: rgba(88,166,255,0.2); color: #58a6ff; border: 1px solid #58a6ff; }
  
  .pill { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 12px; font-weight: 600; margin-right: 6px; }
  .pill-block { background: #21262d; border: 1px solid var(--border); color: #c9d1d9; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1><span>👁️</span> Temporal Visual & DOM Tree Diff</h1>
    <a class="back-link" href="/">← Return to Dashboard</a>
  </header>

  <div class="hud-grid">
    <div class="hud-card">
      <h4>Visual Drift Score</h4>
      <div class="metric" style="color:%s;">%.1f%%</div>
      <div style="font-size:12px;color:#8b949e;margin-top:4px;">Severity: <strong>%s</strong></div>
    </div>
    <div class="hud-card">
      <h4>Structural Similarity</h4>
      <div class="metric" style="color:var(--accent);">%.1f%%</div>
      <div style="font-size:12px;color:#8b949e;margin-top:4px;">Nodes: %d (Old) ↔ %d (New)</div>
    </div>
    <div class="hud-card">
      <h4>Mutations Detected</h4>
      <div class="metric" style="color:var(--yellow);">%d</div>
      <div style="font-size:12px;color:#8b949e;margin-top:4px;">+%d added / -%d removed</div>
    </div>
    <div class="hud-card">
      <h4>Impacted Blocks</h4>
      <div style="margin-top:8px;">%s</div>
    </div>
  </div>

  <div class="split-pane-header">
    <div class="pane-title" style="color:#f85149;">
      🔴 Baseline: %s (%s)
    </div>
    <div class="pane-title" style="color:#7ee787;">
      🟢 Target: %s (%s)
    </div>
  </div>

  <div class="split-view">
    <iframe class="pane-frame" src="/web/%s/%s" title="Baseline Replay"></iframe>
    <iframe class="pane-frame" src="/web/%s/%s" title="Target Replay"></iframe>
  </div>

  <div class="card-table">
    <h3>Hierarchical DOM Mutations (%d)</h3>
    <table>
      <thead>
        <tr>
          <th style="width:40px;">#</th>
          <th>Type</th>
          <th>DOM Path</th>
          <th>Semantic Block</th>
          <th>Mutation Summary</th>
        </tr>
      </thead>
      <tbody>
        %s
      </tbody>
    </table>
  </div>
</div>
</body>
</html>`,
		html.EscapeString(safeTruncateID(res.OldSnapshotID, 8)), html.EscapeString(safeTruncateID(res.NewSnapshotID, 8)),
		severityColor, res.VisualDriftScore, html.EscapeString(string(res.Severity)),
		res.StructuralSimilarity*100, res.TotalNodesOld, res.TotalNodesNew,
		len(res.Mutations), res.AddedNodes, res.RemovedNodes,
		blockPills.String(),
		html.EscapeString(oldSnap.ID), oldSnap.Timestamp.Format("Jan 02 15:04:05 UTC"),
		html.EscapeString(newSnap.ID), newSnap.Timestamp.Format("Jan 02 15:04:05 UTC"),
		oldSnap.WaybackTS, oldSnap.URL,
		newSnap.WaybackTS, newSnap.URL,
		len(res.Mutations),
		mutationRows.String(),
	)
}

func safeTruncateID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}
