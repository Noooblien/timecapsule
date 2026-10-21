# TimeCapsule Architecture ⏳

**TimeCapsule** is a zero-dependency, single-binary, self-hosted web archiving engine and Wayback Machine written in Go.

```
                  ┌──────────────────────────────────────────────┐
                  │                 TimeCapsule                  │
                  │   (CLI / Daemon / Embedded HTTP Server)      │
                  └──────────────────────┬───────────────────────┘
                                         │
                 ┌───────────────────────┴───────────────────────┐
                 ▼                                               ▼
     ┌───────────────────────┐                       ┌───────────────────────┐
     │      pkg/archive      │                       │      pkg/storage      │
     │ ───────────────────── │                       │ ───────────────────── │
     │ • HTTP Fetch & DOM    │                       │ • Atomic Filesystem   │
     │ • Title & Asset Parse │                       │ • JSON Metadata Index │
     │ • Reader Markdown Gen │                       │ • ISO 28500 WARC Store│
     │ • ISO 28500 WARC Form │                       │ • Full-Text Search    │
     └───────────┬───────────┘                       └───────────┬───────────┘
                 │                                               │
                 └───────────────────────┬───────────────────────┘
                                         ▼
                             ┌───────────────────────┐
                             │      pkg/server       │
                             │ ───────────────────── │
                             │ • Dashboard Web UI    │
                             │ • Wayback Replay Mode │
                             │ • Archive Banner Inject│
                             │ • REST API Endpoints  │
                             └───────────────────────┘
```

## Core Modules

### 1. `pkg/archive`
- **Snapshot Representation**: Captures canonical URL, UTC timestamp, Wayback format (`YYYYMMDDhhmmss`), raw HTML, clean Reader Markdown, HTTP headers, asset counts, and SHA-256 integrity hash.
- **Content Distillation**: Automatically extracts page titles and converts article bodies into clean Markdown for offline reading and AI ingestion.
- **WARC 1.0 Exporter**: Generates standard ISO 28500 WARC response records for universal archival interoperability.

### 2. `pkg/storage`
- **Zero-Corruption Layout**: Each snapshot is safely written into `capsules/<snapshot_id>/`:
  - `index.html`: Raw verbatim captured HTML.
  - `content.md`: Extracted reader markdown.
  - `archive.warc`: ISO 28500 compliant WARC record.
- **Fast In-Memory Index**: Backed by `index.json`, providing instant URL and full-text keyword searches across historical archives.
- **Time-Travel Resolver**: `FindByWayback(url, timestamp)` looks up the nearest historical version of any URL.

### 3. `pkg/server`
- **Wayback Replay Engine (`/web/{timestamp}/{url}`)**: Replays captured web pages exactly as they looked on that date, injecting an archival banner with original timestamp and live link.
- **Web Dashboard (`/`)**: Dark-mode dashboard to preserve new URLs, search history, and view/download raw HTML, markdown, and WARC records.
- **REST APIs**: Automated endpoints for capturing (`/api/capture?url=...`), listing (`/api/snapshots`), and visual diff analysis (`/api/diff/visual`).
- **Interactive Dual-Pane Diff (`/vdiff/{old_id}/{new_id}`)**: Interactive visual side-by-side comparison with iframe sandboxes and mutation tables.

### 4. `pkg/visualdiff`
- **Hierarchical DOM Parser (`dom_parser.go`)**: Pure Go recursive tokenizer that parses HTML into structural trees with CSS selector paths, semantic block recognition (`PRICING_TABLE`, `HERO_BANNER`, `NAVIGATION`), and subtree hashing.
- **Temporal Visual Diff Engine (`engine.go`)**: Detects node additions, deletions, text mutations, attribute updates, and structural layout shifts with similarity and drift scores.
- **Scorecards & Interactive Views (`scorecard.go`, `render_html.go`)**: Generates ANSI terminal scorecards, Mermaid DOM mutation subtree diagrams, and standalone dual-pane HTML comparisons.
