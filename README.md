# TimeCapsule ⏳

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/Noooblien/timecapsule)](https://goreportcard.com/report/github.com/Noooblien/timecapsule)
[![CI](https://github.com/Noooblien/timecapsule/actions/workflows/ci.yml/badge.svg)](https://github.com/Noooblien/timecapsule/actions)

**Personal Wayback Machine & Self-Hosted Web Archival Engine in Go.**

Zero external dependencies. Blazing fast. Single binary. Generates standard ISO 28500 WARC files, offline reader Markdown, verbatim HTML snapshots, provides an instant Wayback Machine web replay UI, and includes visual temporal diffing & continuous page watching.

---

## Features

- ⚡ **Zero Dependencies**: Pure Go standard library. No Python virtualenvs, no headless Chrome bloat, no heavy databases.
- 📦 **ISO 28500 WARC Standard**: Emits archival-grade WARC response records compatible with the Internet Archive, Wayback Machine, and WACZ tools.
- 📄 **Clean Reader Markdown**: Automatically strips scripts, ads, and noise, outputting beautiful clean Markdown ready for offline reading and AI/LLM ingestion.
- ⏳ **Wayback Time-Travel Replay**: Built-in HTTP replay server recreating the Wayback Machine URL structure (`/web/<timestamp>/<url>`) with retro archive banners.
- ⇄ **Temporal Snapshot Diffing**: Compare two captures across time with LCS (Longest Common Subsequence) diffing, similarity metrics, and visual color highlights.
- 👁️ **Automated Page Watcher**: Continuously monitor any web page for changes, auto-archiving new revisions and printing real-time diffs.
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

### 3. Continuous Page Watcher
Monitor any live URL for updates and automatically archive new versions as soon as changes occur:
```bash
timecapsule watch https://news.ycombinator.com --interval 1m
```

### 4. Launch the Wayback Replay Web Server
```bash
timecapsule serve --port 8080
```
Open `http://localhost:8080` in your browser to browse your personal archive, replay historical pages, or visually inspect side-by-side diffs at:
- Replay: `http://localhost:8080/web/<timestamp>/<url>`
- Diff: `http://localhost:8080/diff/<old_id>/<new_id>`

### 5. List Archived Snapshots
```bash
timecapsule list
```

### 6. Search Historical Pages
```bash
timecapsule search "artificial intelligence"
```

### 7. View Reader Markdown in Terminal
```bash
timecapsule view tc_20261018000400_a3f19e8c
```

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
