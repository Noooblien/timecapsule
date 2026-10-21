# TimeCapsule ⏳

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/Noooblien/timecapsule)](https://goreportcard.com/report/github.com/Noooblien/timecapsule)
[![CI](https://github.com/Noooblien/timecapsule/actions/workflows/ci.yml/badge.svg)](https://github.com/Noooblien/timecapsule/actions)

**Personal Wayback Machine & Self-Hosted Web Archival Engine in Go.**

Zero external dependencies. Blazing fast. Single binary. Generates standard ISO 28500 WARC files, offline reader Markdown, verbatim HTML snapshots, provides an instant Wayback Machine web replay UI, and includes visual temporal diffing & continuous page watching.

---

## Features

- ⚡ **Zero Dependencies**: Pure Go standard library. No Python virtualenvs, no headless Chrome bloat, no heavy databases.
- 📦 **Verbatim Single-File DOM Serializer**: Inline remote stylesheets, nested CSS assets, and images into Base64 Data URIs, neutralize active scripts into inert payloads, and produce 100% hermetic offline HTML bundles.
- 📦 **ISO 28500 WARC Standard**: Emits archival-grade multi-record WARC suites (`warcinfo`, `request`, `response`, `metadata`) with ISO 28500 Annex D GZIP concatenation.
- 🔄 **WARC Import & Export**: Import external `.warc` and `.warc.gz` dumps directly into your Wayback archive or export snapshots with cryptographic digest integrity.
- 📑 **Standard CDXJ 1.1 Indexing & SURT**: Generates universal Capture Index Format (CDXJ) records with Sort-friendly URI Technique (SURT) keys compatible with OutbackCDX and pywb.
- 🌐 **Wayback CDX Server API**: Built-in `/cdx` and `/api/cdx` query endpoints supporting JSON and CDXJ outputs for Wayback Machine browser extensions and tools.
- 📖 **Offline Reader & Markdown Engine**: Noise-free readability parser stripping navigation, sidebars, ads, and footers; extracts author, publication date, and description metadata; generates structured Markdown with tables, code fences, blockquotes, and YAML frontmatter; generates distraction-free responsive Reader View HTML.
- 📄 **Clean Reader Markdown**: Automatically strips scripts, ads, and noise, outputting beautiful clean Markdown ready for offline reading and AI/LLM ingestion.
- ⏳ **Wayback Time-Travel Replay**: Built-in HTTP replay server recreating the Wayback Machine URL structure (`/web/<timestamp>/<url>`) with retro archive banners.
- ⇄ **Temporal Snapshot Diffing**: Compare two captures across time with LCS (Longest Common Subsequence) diffing, similarity metrics, and visual color highlights.
- 👁️ **Automated Page Watcher**: Continuously monitor any web page for changes, auto-archiving new revisions and printing real-time diffs.
- ⏳ **RFC 7089 Memento Framework**: Time-travel datetime negotiation via `Accept-Datetime`, TimeGate nearest-neighbor resolution, and canonical `application/link-format` or JSON TimeMaps.
- 🛡️ **Cryptographic Proof of Archival & Merkle Authenticity**: 5-leaf SHA-256 Merkle tree verification, Ed25519 digital signature chain of custody, isolated leaf-level forensic tamper detection, and JSON/Markdown attestation receipts for legal audit trails.
- 🔍 **Full-Text Search**: Search through historical pages by title, URL, or article content.
- 🖥️ **Modern Dark-Mode Dashboard**: Lightweight built-in web interface for preserving, diffing, and exploring historical snapshots.

---

## Quickstart

### Installation

```bash
go install github.com/Noooblien/timecapsule/cmd/timecapsule@latest
```

Or build from source:

```bash
git clone https://github.com/Noooblien/timecapsule.git
cd timecapsule
go build -o timecapsule ./cmd/timecapsule
```

---

## Usage

### 1. Preserve a Web Page
```bash
timecapsule save https://news.ycombinator.com
```
```text
⏳ Capturing snapshot of https://news.ycombinator.com...
✅ Preserved successfully!
  ID:        tc_20261018000400_a3f19e8c
  Title:     Hacker News
  Timestamp: 2026-10-18T00:04:00Z (Wayback: 20261018000400)
  Size:      18,452 bytes (14 assets)
  Hash:      a3f19e8c9284f...
  Replay:    /web/20261018000400/https://news.ycombinator.com
```

### 2. Compare Historical Snapshots (Diff)
```bash
timecapsule diff tc_20261018000400_a3f19e8c tc_20261018004500_b9e28f11
```
```text
--- tc_20261018000400_a3f19e8c (2026-10-18 00:04:00 UTC)
+++ tc_20261018004500_b9e28f11 (2026-10-18 00:45:00 UTC)
Similarity: 94.2% (+5 / -3)

  # Hacker News
- 1. Project Alpha launches beta (120 points)
+ 1. Project Alpha launches beta (310 points)
+ 2. New AI model release (85 points)
```

### 3. Temporal Visual & Hierarchical DOM Tree Diff (`vdiff`)
Analyze structural mutations, text drift, and component additions/deletions across semantic blocks (`PRICING_TABLE`, `HERO_BANNER`, `NAVIGATION`, `CTA`, `FOOTER`):
```bash
# Run 4-scenario visual diff benchmark demo
timecapsule vdiff --demo

# Generate Mermaid flowchart diagram of impacted DOM subtrees
timecapsule vdiff --demo --mermaid

# Compare two snapshots and generate an interactive dual-pane HTML diff
timecapsule vdiff snap_old snap_new --html diff_visual.html

# Output GitHub Markdown summary report for pull request gates
timecapsule vdiff snap_old snap_new --markdown
```

### 4. Continuous Page Watcher & Website Drift Monitor
Monitor any live URL or registry of web pages for updates with automated drift classification, noise filtering, webhook alerts, and auto-archival:
```bash
# Run multi-target simulated drift & mutation benchmark demo
timecapsule watch --demo

# Register target for continuous monitoring with noise filtering and webhook dispatch
timecapsule watch --add https://news.ycombinator.com --name "Hacker News" --interval 5m --ignore "ad-banner,timestamp" --webhook "https://alerts.example.com/drift"

# Display status scorecard of all registered watch targets and recent drift events
timecapsule watch --list

# Execute on-demand polling cycle across all active targets
timecapsule watch --poll

# Generate Mermaid timeline of detected website mutations
timecapsule watch --demo --mermaid
```

### 5. Distraction-Free Offline Reader & Markdown Conversion
Convert any live URL or existing snapshot into clean Markdown with metadata frontmatter or standalone distraction-free Reader View HTML:
```bash
# Convert live URL to clean Markdown with YAML frontmatter
timecapsule reader https://news.ycombinator.com --frontmatter

# Convert snapshot to a Markdown file
timecapsule reader tc_20261018000400_a3f19e8c --out article.md --frontmatter

# Output standalone distraction-free Reader View HTML
timecapsule reader https://blog.example.com/post --html --out reader.html
```

### 6. Create Hermetic Single-File DOM Bundle
Inline all remote assets (stylesheets, nested CSS fonts/images, images) and neutralize active scripts into a self-contained, offline-viewable HTML file:
```bash
# Bundle directly from a live URL
timecapsule bundle https://news.ycombinator.com --out hn.single.html

# Or bundle from a previously captured snapshot ID
timecapsule bundle tc_20261018000400_a3f19e8c --out snapshot.single.html
```

### 7. Export ISO 28500 WARC Archive
Export any snapshot into standard ISO 28500 multi-record WARC file (with optional Annex D GZIP compression):
```bash
timecapsule export tc_20261018000400_a3f19e8c --out my_archive.warc.gz --gzip
```

### 8. Import External WARC Dumps
Import existing WARC archives from the Internet Archive, Common Crawl, or wget crawls directly into TimeCapsule:
```bash
timecapsule import external_crawl.warc.gz
```

### 9. Generate Standard CDXJ Index
Output standard CDXJ 1.1 capture index entries with SURT keys:
```bash
timecapsule cdx https://news.ycombinator.com
```

### 10. Run Archival Fidelity & ISO 28500 Benchmark
Empirically assert hermetic isolation, asset preservation ratio, ISO 28500 WARC standard compliance, and compression efficiency across test corpuses:
```bash
# Run 4-corpus empirical benchmark suite
timecapsule bench --demo

# Generate GitHub Markdown summary table for CI/CD gates
timecapsule bench --demo --markdown
```

### 11. Verify Archival Isolation & Privacy Leak Audit
Audit any snapshot, WARC file, or standalone HTML file for un-neutralized active scripts, remote network leaks, or corrupt WARC headers:
```bash
# Verify a captured snapshot
timecapsule verify tc_20261018000400_a3f19e8c --strict

# Audit an external WARC file for ISO 28500 compliance
timecapsule verify crawl.warc.gz

# Audit an offline HTML file for remote network leaks
timecapsule verify page.single.html
```

### 12. Launch the Wayback Replay Web Server
```bash
timecapsule serve --port 8080
```
Open `http://localhost:8080` in your browser to browse your personal archive, replay historical pages, or visually inspect side-by-side diffs at:
- Replay: `http://localhost:8080/web/<timestamp>/<url>`
- Reader View: `http://localhost:8080/reader/<id>`
- Visual Dual-Pane Diff: `http://localhost:8080/vdiff/<old_id>/<new_id>`
- Visual Diff API: `http://localhost:8080/api/diff/visual?old=<old_id>&new=<new_id>`
- Text Diff: `http://localhost:8080/diff/<old_id>/<new_id>`
- CDX API: `http://localhost:8080/cdx?url=https://news.ycombinator.com&output=json`

### 13. List Archived Snapshots
```bash
timecapsule list
```

### 14. Search Historical Pages
```bash
timecapsule search "artificial intelligence"
```

### 15. View Reader Markdown in Terminal
```bash
timecapsule view tc_20261018000400_a3f19e8c
```

### 16. Cryptographic Proof of Archival & Merkle Authenticity (`timecapsule attest`)
Compute cryptographic Merkle trees across all snapshot facets (HTTP metadata, verbatim HTML DOM, reader markdown, archive manifest, asset manifest), sign snapshots with Ed25519 legal authority keys, and verify custody chains:
```bash
# Run 4-scenario forensic attestation and tamper detection demo suite
timecapsule attest --demo

# Generate Mermaid Merkle authenticity tree diagram
timecapsule attest --demo --mermaid

# Attest a specific archived snapshot and save JSON receipt
timecapsule attest --id tc_20261018000400_a3f19e8c --receipt receipt.json

# Verify cryptographic receipt with tamper check
timecapsule attest --verify receipt.json --strict
```

### 17. RFC 7089 Memento TimeGate & TimeMap Engine (`timecapsule memento`)
Full compliance with the IETF RFC 7089 web archival time-travel standard. Enables datetime negotiation via `Accept-Datetime`, TimeGate nearest-neighbor resolution, and canonical `application/link-format` or JSON TimeMaps:
```bash
# Run 4-scenario temporal negotiation and TimeGate interpolation demo
timecapsule memento --demo

# Generate Mermaid sequence diagram of RFC 7089 datetime negotiation
timecapsule memento --demo --mermaid

# Negotiate closest memento to target date via TimeGate
timecapsule memento timegate https://news.ycombinator.com -time "2024-02-15T18:30:00Z"

# Retrieve complete chronological TimeMap in RFC 7089 link-format or JSON
timecapsule memento timemap https://news.ycombinator.com -format link
timecapsule memento timemap https://news.ycombinator.com -format json

# HTTP Server Endpoints:
# GET http://localhost:8080/timegate/{url...}   (Accept-Datetime: Tue, 20 Mar 2026 18:00:00 GMT -> 302 Found)
# GET http://localhost:8080/timemap/link/{url...} (application/link-format)
# GET http://localhost:8080/timemap/json/{url...} (application/json)
```

---

## Empirical Benchmarks & Archival Fidelity

Zero marketing hype. Measured directly by the TimeCapsule verification engine (`go test -v ./...` and `timecapsule bench --demo`):

| Corpus | Hermetic Score | Asset Retention | ISO 28500 WARC | Space Savings | Throughput | Verdict |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Clean Documentation Page** | `100.0%` | `0/0 (100.0%)` | ✅ Valid | `17.8% (1.2x)` | `5.5 MB/s` | `PASS (92.1/100)` |
| **Modern Media Web App (Hermetic Bundle)** | `100.0%` | `3/3 (100.0%)` | ✅ Valid | `15.6% (1.2x)` | `6.1 MB/s` | `PASS (91.9/100)` |
| **Hostile Tracking & Network Leak Trap** | `65.0%` | `0/2 (0.0%)` | ✅ Valid | `16.8% (1.2x)` | `1.6 MB/s` | `LEAK BREACH (48.0/100)` |
| **High-Volume Multi-Record WARC Stream** | `100.0%` | `0/0 (100.0%)` | ✅ Valid | `89.5% (9.6x)` | `20.9 MB/s` | `PASS (100.0/100)` |
| **Cryptographic Provenance & Merkle Tree** | `100.0%` | `5/5 (100.0%)` | ✅ Valid | `0% Overhead` | `14.8 MB/s` | `PASS (100.0/100)` |

---

## Storage Directory Layout

By default, TimeCapsule stores archives in `~/.timecapsule`:

```
~/.timecapsule/
├── index.json               # Fast search & metadata catalog
└── capsules/
    └── tc_20261018000400_a3f19e8c/
        ├── index.html       # Verbatim captured HTML
        ├── content.md       # Clean extracted reader markdown
        └── archive.warc     # Standard ISO 28500 WARC file
```

---

## Testing

Run the test suite:

```bash
go test -v ./tests
```

---

## License

[MIT](LICENSE) © 2026 [Rahul Singh Maraskole](https://github.com/Noooblien)
