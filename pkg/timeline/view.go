package timeline

import (
	"fmt"
	"html"
	"strings"
)

// RenderWaybackBanner generates a responsive, rich Wayback Machine top navigation toolbar
// with previous/next snapshot links, timeline position indicator, diff toggles, and metadata.
func RenderWaybackBanner(nav *NavContext) string {
	if nav == nil {
		return ""
	}

	snapTime := nav.Current.Timestamp.Format("Jan 02, 2006 15:04:05 UTC")
	hashShort := nav.Current.ContentHash
	if len(hashShort) > 8 {
		hashShort = hashShort[:8]
	}

	// First / Prev buttons
	firstBtn := `<span class="tb-btn disabled">&laquo; First</span>`
	if nav.First != nil {
		firstBtn = fmt.Sprintf(`<a class="tb-btn" href="%s" title="First capture: %s">&laquo; First</a>`,
			nav.First.ReplayURL, nav.First.Timestamp.Format("2006-01-02 15:04"))
	}

	prevBtn := `<span class="tb-btn disabled">&lsaquo; Prev</span>`
	if nav.Prev != nil {
		prevBtn = fmt.Sprintf(`<a class="tb-btn" href="%s" title="Previous capture: %s">&lsaquo; Prev</a>`,
			nav.Prev.ReplayURL, nav.Prev.Timestamp.Format("2006-01-02 15:04"))
	}

	nextBtn := `<span class="tb-btn disabled">Next &rsaquo;</span>`
	if nav.Next != nil {
		nextBtn = fmt.Sprintf(`<a class="tb-btn" href="%s" title="Next capture: %s">Next &rsaquo;</a>`,
			nav.Next.ReplayURL, nav.Next.Timestamp.Format("2006-01-02 15:04"))
	}

	lastBtn := `<span class="tb-btn disabled">Last &raquo;</span>`
	if nav.Last != nil {
		lastBtn = fmt.Sprintf(`<a class="tb-btn" href="%s" title="Latest capture: %s">Last &raquo;</a>`,
			nav.Last.ReplayURL, nav.Last.Timestamp.Format("2006-01-02 15:04"))
	}

	// Diff links if prev exists
	diffLinks := ""
	if nav.Prev != nil {
		diffLinks = fmt.Sprintf(`
			<a class="tb-action" href="%s" target="_blank" title="Side-by-side line diff with previous snapshot">Diff ⇄</a>
			<a class="tb-action" href="%s" target="_blank" title="Visual DOM & structural diff">Visual Diff 🎨</a>`,
			nav.Current.DiffURL, nav.Current.VDiffURL)
	}

	timelineURL := fmt.Sprintf("/timeline/%s", nav.Current.ReplayURL[strings.Index(nav.Current.ReplayURL, "/http")+1:])

	return fmt.Sprintf(`<!-- TimeCapsule Wayback Banner -->
<div id="timecapsule-replay-banner" style="all:initial;">
<style>
#tc-banner-wrap {
	position: fixed; top: 0; left: 0; right: 0; height: 48px;
	background: #161b22; border-bottom: 2px solid #238636;
	color: #c9d1d9; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
	font-size: 13px; display: flex; align-items: center; justify-content: space-between;
	padding: 0 16px; z-index: 2147483647; box-shadow: 0 4px 16px rgba(0,0,0,0.6);
	box-sizing: border-box; line-height: normal;
}
#tc-banner-wrap a { color: #58a6ff; text-decoration: none; }
#tc-banner-wrap a:hover { text-decoration: underline; }
.tb-section { display: flex; align-items: center; gap: 8px; }
.tb-brand { font-weight: 700; color: #f0f6fc; display: flex; align-items: center; gap: 6px; }
.tb-brand a { color: #f0f6fc; }
.tb-badge {
	background: #21262d; border: 1px solid #30363d; color: #8b949e;
	padding: 2px 7px; border-radius: 10px; font-size: 11px; font-weight: 600;
}
.tb-badge.highlight { color: #58a6ff; border-color: #1f6feb; background: rgba(31,111,235,0.15); }
.tb-btn {
	background: #21262d; border: 1px solid #30363d; color: #c9d1d9 !important;
	padding: 4px 9px; border-radius: 5px; font-size: 12px; font-weight: 600;
	text-decoration: none !important; cursor: pointer; transition: all 0.15s;
}
.tb-btn:hover { background: #30363d; color: #fff !important; }
.tb-btn.disabled { opacity: 0.4; cursor: not-allowed; pointer-events: none; }
.tb-action {
	background: #1f6feb22; border: 1px solid #1f6feb55; color: #58a6ff !important;
	padding: 4px 8px; border-radius: 5px; font-size: 12px; text-decoration: none !important;
}
.tb-action:hover { background: #1f6feb44; }
.tb-exit {
	background: #238636; color: #fff !important; padding: 4px 12px;
	border-radius: 5px; font-weight: 600; font-size: 12px; text-decoration: none !important;
}
.tb-exit:hover { background: #2ea043; }
</style>
<div id="tc-banner-wrap">
	<div class="tb-section">
		<div class="tb-brand">
			<a href="/" title="TimeCapsule Dashboard">⏳ TimeCapsule Archive</a>
		</div>
		<a href="%s" class="tb-badge highlight" title="View interactive timeline for this URL">Capture %d of %d 📅</a>
		<div style="color:#e6edf3;font-weight:600;">%s</div>
		<span class="tb-badge" title="SHA-256 Digest">#%s</span>
	</div>

	<div class="tb-section">
		%s
		%s
		%s
		%s
	</div>

	<div class="tb-section">
		%s
		<a class="tb-action" href="%s" target="_blank">Reader 📖</a>
		<a class="tb-action" href="%s" target="_blank">Bundle 📦</a>
		<a class="tb-action" href="%s" target="_blank">WARC 📦</a>
		<a class="tb-btn" href="%s" target="_blank" title="Live Website">Live URL ↗</a>
		<a class="tb-exit" href="/">← Exit</a>
	</div>
</div>
<div style="height:48px;display:block;"></div>
</div>
<!-- /TimeCapsule Wayback Banner -->`,
		timelineURL, nav.Position, nav.Total,
		html.EscapeString(snapTime),
		html.EscapeString(hashShort),
		firstBtn, prevBtn, nextBtn, lastBtn,
		diffLinks,
		nav.Current.ReaderURL, nav.Current.BundleURL, nav.Current.WARCURL,
		html.EscapeString(nav.Current.ReplayURL[strings.Index(nav.Current.ReplayURL, "/http")+1:]),
	)
}

// RenderTimelinePage produces a dedicated interactive timeline dashboard for a URL,
// showing year-by-year histograms, chronological capture points, and quick jump buttons.
func RenderTimelinePage(tl *URLTimeline) string {
	if tl == nil {
		return ""
	}

	yearBlocks := ""
	for _, y := range tl.Years {
		monthSpans := ""
		monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		for mIdx, cnt := range y.Months {
			cls := "month-col"
			if cnt > 0 {
				cls += " active"
			}
			barH := cnt * 12 + 4
			if barH > 60 {
				barH = 60
			}
			monthSpans += fmt.Sprintf(`<div class="%s"><div class="m-bar" style="height:%dpx;"></div><span class="m-lbl">%s</span><span class="m-cnt">%d</span></div>`,
				cls, barH, monthNames[mIdx], cnt)
		}

		yearBlocks += fmt.Sprintf(`
		<div class="year-card">
			<div class="year-header">
				<span class="y-title">%d</span>
				<span class="y-total">%d captures</span>
			</div>
			<div class="months-grid">
				%s
			</div>
		</div>`, y.Year, y.Total, monthSpans)
	}

	captureRows := ""
	for i, c := range tl.Captures {
		diffBtn := ""
		if c.DiffURL != "" {
			diffBtn = fmt.Sprintf(`<a class="btn-sm" href="%s" target="_blank">Diff ⇄</a> <a class="btn-sm" href="%s" target="_blank">V-Diff 🎨</a>`, c.DiffURL, c.VDiffURL)
		}
		captureRows += fmt.Sprintf(`
		<tr class="cap-row">
			<td class="idx-col">#%d</td>
			<td class="time-col"><a href="%s" class="time-link">%s</a></td>
			<td><span class="badge-status">%d</span></td>
			<td>%s</td>
			<td>%d KB</td>
			<td class="actions-col">
				<a class="btn-sm btn-primary" href="%s">Replay ⏳</a>
				%s
				<a class="btn-sm" href="%s" target="_blank">Reader 📖</a>
				<a class="btn-sm" href="%s" target="_blank">Bundle 📦</a>
				<a class="btn-sm" href="%s">WARC 📦</a>
			</td>
		</tr>`,
			i+1, c.ReplayURL, c.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			c.StatusCode, html.EscapeString(c.ContentType), c.SizeBytes/1024,
			c.ReplayURL, diffBtn, c.ReaderURL, c.BundleURL, c.WARCURL)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Timeline: %s &bull; TimeCapsule</title>
<style>
:root { --bg: #0d1117; --card: #161b22; --border: #30363d; --text: #c9d1d9; --accent: #58a6ff; --green: #238636; }
body { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
.container { max-width: 1060px; margin: 30px auto; padding: 0 20px; }
header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border); padding-bottom: 16px; margin-bottom: 24px; }
h1 { margin: 0; font-size: 22px; color: #f0f6fc; display: flex; align-items: center; gap: 10px; }
.url-sub { font-size: 14px; color: #8b949e; word-break: break-all; margin-top: 6px; }
.stats-bar { display: flex; gap: 20px; margin-bottom: 24px; }
.stat-box { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 12px 20px; flex: 1; }
.stat-box .val { font-size: 20px; font-weight: 700; color: #f0f6fc; }
.stat-box .lbl { font-size: 12px; color: #8b949e; margin-top: 2px; }
.year-card { background: var(--card); border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
.year-header { display: flex; justify-content: space-between; margin-bottom: 12px; font-size: 15px; font-weight: 600; color: #f0f6fc; }
.months-grid { display: grid; grid-template-columns: repeat(12, 1fr); gap: 6px; }
.month-col { display: flex; flex-direction: column; align-items: center; justify-content: flex-end; height: 80px; background: #21262d; border-radius: 4px; padding: 4px; font-size: 11px; }
.month-col.active { background: rgba(31,111,235,0.12); border: 1px solid #1f6feb; }
.m-bar { width: 14px; background: #58a6ff; border-radius: 2px; margin-bottom: 4px; min-height: 2px; }
.m-lbl { color: #8b949e; }
.m-cnt { font-weight: 600; font-size: 10px; color: #f0f6fc; }
.table-wrap { background: var(--card); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; margin-top: 24px; }
table { width: 100%%; border-collapse: collapse; font-size: 13px; text-align: left; }
th { background: #21262d; padding: 12px 14px; font-weight: 600; color: #8b949e; border-bottom: 1px solid var(--border); }
td { padding: 12px 14px; border-bottom: 1px solid #21262d; }
.cap-row:hover td { background: #1f242c; }
.idx-col { color: #8b949e; font-weight: 600; width: 40px; }
.time-link { color: #58a6ff; text-decoration: none; font-weight: 600; }
.time-link:hover { text-decoration: underline; }
.badge-status { background: #23863622; color: #7ee787; border: 1px solid #238636; padding: 2px 6px; border-radius: 4px; font-size: 11px; font-weight: 600; }
.btn-sm { padding: 4px 8px; border-radius: 4px; text-decoration: none; font-size: 12px; background: #21262d; color: #c9d1d9; border: 1px solid #30363d; display: inline-block; }
.btn-sm:hover { background: #30363d; color: #fff; }
.btn-primary { background: #238636; color: #fff; border-color: #2ea043; }
.btn-primary:hover { background: #2ea043; }
.nav-back { color: #58a6ff; text-decoration: none; font-size: 14px; }
</style>
</head>
<body>
<div class="container">
	<header>
		<div>
			<h1>⏳ Temporal Timeline</h1>
			<div class="url-sub">%s</div>
		</div>
		<div>
			<a class="nav-back" href="/">&larr; Back to Dashboard</a>
		</div>
	</header>

	<div class="stats-bar">
		<div class="stat-box">
			<div class="val">%d</div>
			<div class="lbl">Total Historical Captures</div>
		</div>
		<div class="stat-box">
			<div class="val">%s</div>
			<div class="lbl">Earliest Capture</div>
		</div>
		<div class="stat-box">
			<div class="val">%s</div>
			<div class="lbl">Latest Capture</div>
		</div>
	</div>

	%s

	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th>#</th>
					<th>Capture Timestamp</th>
					<th>Status</th>
					<th>MIME Type</th>
					<th>Size</th>
					<th>Replay & Analysis</th>
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
		html.EscapeString(tl.URL),
		html.EscapeString(tl.URL),
		tl.TotalCaptures,
		tl.FirstCapture.Format("Jan 02, 2006 15:04 UTC"),
		tl.LastCapture.Format("Jan 02, 2006 15:04 UTC"),
		yearBlocks,
		captureRows,
	)
}
