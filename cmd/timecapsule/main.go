package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/diff"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

const version = "1.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, ".timecapsule")

	cmd := os.Args[1]

	switch cmd {
	case "save":
		saveCmd := flag.NewFlagSet("save", flag.ExitOnError)
		dataDir := saveCmd.String("dir", defaultDataDir, "Storage directory")
		timeout := saveCmd.Duration("timeout", 15*time.Second, "Fetch timeout")
		_ = saveCmd.Parse(os.Args[2:])

		args := saveCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: URL required. Usage: timecapsule save <url>")
			os.Exit(1)
		}
		targetURL := args[0]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		archiver := archive.NewArchiver(*timeout)
		fmt.Printf("⏳ Capturing snapshot of %s...\n", targetURL)

		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		snap, err := archiver.Capture(ctx, targetURL)
		if err != nil {
			fmt.Printf("❌ Failed to capture URL: %v\n", err)
			os.Exit(1)
		}

		if err := store.Save(snap); err != nil {
			fmt.Printf("❌ Failed to save snapshot: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Preserved successfully!\n")
		fmt.Printf("  ID:        %s\n", snap.ID)
		fmt.Printf("  Title:     %s\n", snap.Title)
		fmt.Printf("  Timestamp: %s (Wayback: %s)\n", snap.Timestamp.Format(time.RFC3339), snap.WaybackTS)
		fmt.Printf("  Size:      %d bytes (%d assets)\n", snap.SizeBytes, snap.AssetCount)
		fmt.Printf("  Hash:      %s\n", snap.ContentHash)
		fmt.Printf("  Replay:    /web/%s/%s\n", snap.WaybackTS, snap.URL)

	case "diff":
		diffCmd := flag.NewFlagSet("diff", flag.ExitOnError)
		dataDir := diffCmd.String("dir", defaultDataDir, "Storage directory")
		_ = diffCmd.Parse(os.Args[2:])

		args := diffCmd.Args()
		if len(args) < 2 {
			fmt.Println("Error: Two snapshot IDs required. Usage: timecapsule diff <old_id> <new_id>")
			os.Exit(1)
		}
		oldID := args[0]
		newID := args[1]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		oldSnap, ok := store.Get(oldID)
		if !ok {
			fmt.Printf("Error: Snapshot '%s' not found\n", oldID)
			os.Exit(1)
		}
		newSnap, ok := store.Get(newID)
		if !ok {
			fmt.Printf("Error: Snapshot '%s' not found\n", newID)
			os.Exit(1)
		}

		res := diff.CompareSnapshots(oldSnap, newSnap)
		fmt.Print(res.FormatANSI())

	case "watch":
		watchCmd := flag.NewFlagSet("watch", flag.ExitOnError)
		dataDir := watchCmd.String("dir", defaultDataDir, "Storage directory")
		interval := watchCmd.Duration("interval", 30*time.Second, "Check interval (e.g. 10s, 1m)")
		timeout := watchCmd.Duration("timeout", 15*time.Second, "Fetch timeout")
		_ = watchCmd.Parse(os.Args[2:])

		args := watchCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: URL required. Usage: timecapsule watch <url> [--interval 30s]")
			os.Exit(1)
		}
		targetURL := args[0]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		archiver := archive.NewArchiver(*timeout)
		fmt.Printf("👁️  Watching %s every %v for content changes (Ctrl+C to stop)...\n", targetURL, *interval)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		checkFunc := func() {
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()

			snap, err := archiver.Capture(ctx, targetURL)
			if err != nil {
				fmt.Printf("[%s] ⚠️ Check failed: %v\n", time.Now().Format("15:04:05"), err)
				return
			}

			// Check latest snapshot for this URL
			history := store.GetHistory(targetURL)
			if len(history) == 0 {
				_ = store.Save(snap)
				fmt.Printf("[%s] 📥 Initial baseline capture stored (%s - %s)\n",
					time.Now().Format("15:04:05"), snap.ID[:8], snap.Title)
				return
			}

			latest := history[len(history)-1]
			if latest.ContentHash == snap.ContentHash {
				fmt.Printf("[%s] 💤 No changes detected (hash: %s)\n",
					time.Now().Format("15:04:05"), snap.ContentHash[:8])
				return
			}

			// Content changed!
			if err := store.Save(snap); err != nil {
				fmt.Printf("[%s] ❌ Failed to save new snapshot: %v\n", time.Now().Format("15:04:05"), err)
				return
			}

			res := diff.CompareSnapshots(latest, snap)
			fmt.Printf("\n🔔 [%s] Change detected! New snapshot: %s\n", time.Now().Format("15:04:05"), snap.ID)
			fmt.Print(res.FormatANSI())
			fmt.Println()
		}

		// Initial check immediately
		checkFunc()

		for {
			select {
			case <-sigChan:
				fmt.Println("\nStopped watching.")
				return
			case <-ticker.C:
				checkFunc()
			}
		}

	case "list":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		dataDir := listCmd.String("dir", defaultDataDir, "Storage directory")
		_ = listCmd.Parse(os.Args[2:])

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		snapshots := store.List()
		if len(snapshots) == 0 {
			fmt.Println("No snapshots recorded yet. Use 'timecapsule save <url>' to archive your first page.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tTIMESTAMP\tSIZE\tURL\tTITLE")
		for _, s := range snapshots {
			title := s.Title
			if len(title) > 35 {
				title = title[:32] + "..."
			}
			urlStr := s.URL
			if len(urlStr) > 40 {
				urlStr = urlStr[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d B\t%s\t%s\n",
				s.ID, s.Timestamp.Format("2006-01-02 15:04"), s.SizeBytes, urlStr, title)
		}
		w.Flush()

	case "search":
		searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
		dataDir := searchCmd.String("dir", defaultDataDir, "Storage directory")
		_ = searchCmd.Parse(os.Args[2:])

		args := searchCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: Query required. Usage: timecapsule search <query>")
			os.Exit(1)
		}
		query := args[0]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		results := store.Search(query)
		fmt.Printf("Found %d snapshots matching '%s':\n\n", len(results), query)
		for _, s := range results {
			fmt.Printf("• [%s] %s\n  URL: %s\n  Timestamp: %s\n\n",
				s.ID, s.Title, s.URL, s.Timestamp.Format(time.RFC3339))
		}

	case "view":
		viewCmd := flag.NewFlagSet("view", flag.ExitOnError)
		dataDir := viewCmd.String("dir", defaultDataDir, "Storage directory")
		_ = viewCmd.Parse(os.Args[2:])

		args := viewCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: ID required. Usage: timecapsule view <id>")
			os.Exit(1)
		}
		id := args[0]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		snap, ok := store.Get(id)
		if !ok {
			fmt.Printf("Error: Snapshot '%s' not found\n", id)
			os.Exit(1)
		}

		fmt.Println(snap.Markdown)

	case "export":
		exportCmd := flag.NewFlagSet("export", flag.ExitOnError)
		dataDir := exportCmd.String("dir", defaultDataDir, "Storage directory")
		outFile := exportCmd.String("out", "", "Output file path (default: <id>.warc or <id>.warc.gz)")
		useGzip := exportCmd.Bool("gzip", false, "Compress output with GZIP (ISO 28500 Annex D)")
		_ = exportCmd.Parse(os.Args[2:])

		args := exportCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: Snapshot ID required. Usage: timecapsule export <id> [--out <file>] [--gzip]")
			os.Exit(1)
		}
		id := args[0]

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		snap, ok := store.Get(id)
		if !ok {
			fmt.Printf("Error: Snapshot '%s' not found\n", id)
			os.Exit(1)
		}

		targetFile := *outFile
		isGz := *useGzip
		if targetFile == "" {
			targetFile = snap.ID + ".warc"
			if isGz {
				targetFile += ".gz"
			}
		} else if strings.HasSuffix(targetFile, ".gz") {
			isGz = true
		}

		f, err := os.Create(targetFile)
		if err != nil {
			fmt.Printf("Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		writer := warc.NewWriter(f, isGz)
		cdxEntries, err := writer.WriteSnapshotArchive(snap, filepath.Base(targetFile))
		if err != nil {
			fmt.Printf("Error exporting WARC: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Exported ISO 28500 WARC to %s (%d bytes, %d CDX records)\n", targetFile, writer.CurrentOffset(), len(cdxEntries))
		if len(cdxEntries) > 0 {
			fmt.Printf("  CDXJ: %s", warc.FormatCDXJ(cdxEntries[0]))
		}

	case "import":
		importCmd := flag.NewFlagSet("import", flag.ExitOnError)
		dataDir := importCmd.String("dir", defaultDataDir, "Storage directory")
		_ = importCmd.Parse(os.Args[2:])

		args := importCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: WARC file required. Usage: timecapsule import <file.warc[.gz]>")
			os.Exit(1)
		}
		warcPath := args[0]

		f, err := os.Open(warcPath)
		if err != nil {
			fmt.Printf("Error opening WARC file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		reader, err := warc.NewReader(f)
		if err != nil {
			fmt.Printf("Error initializing WARC reader: %v\n", err)
			os.Exit(1)
		}

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		imported := 0
		for {
			rec, err := reader.NextRecord()
			if err != nil {
				break
			}
			if rec.Type == warc.RecordTypeResponse {
				snap, err := warc.ImportToSnapshot(rec)
				if err != nil {
					continue
				}
				if err := store.Save(snap); err == nil {
					imported++
					fmt.Printf("  • Imported %s -> %s (%s)\n", snap.URL, snap.ID, snap.Title)
				}
			}
		}
		fmt.Printf("✅ Successfully imported %d snapshots from %s\n", imported, filepath.Base(warcPath))

	case "cdx":
		cdxCmd := flag.NewFlagSet("cdx", flag.ExitOnError)
		dataDir := cdxCmd.String("dir", defaultDataDir, "Storage directory")
		_ = cdxCmd.Parse(os.Args[2:])

		args := cdxCmd.Args()
		targetURL := ""
		if len(args) > 0 {
			targetURL = args[0]
		}

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		var snapshots []*archive.Snapshot
		if targetURL != "" {
			snapshots = store.GetHistory(targetURL)
		} else {
			snapshots = store.List()
		}

		for _, snap := range snapshots {
			entry := warc.CDXJEntry{
				SURT:        warc.ToSURT(snap.URL),
				Timestamp:   snap.WaybackTS,
				OriginalURL: snap.URL,
				MimeType:    snap.ContentType,
				StatusCode:  snap.StatusCode,
				Digest:      snap.ContentHash,
				Length:      snap.SizeBytes,
				Offset:      0,
				Filename:    fmt.Sprintf("%s.warc", snap.ID),
			}
			fmt.Print(warc.FormatCDXJ(entry))
		}

	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		port := serveCmd.Int("port", 8080, "Port to listen on")
		dataDir := serveCmd.String("dir", defaultDataDir, "Storage directory")
		_ = serveCmd.Parse(os.Args[2:])

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		archiver := archive.NewArchiver(15 * time.Second)
		srv := server.NewServer(store, archiver)

		addr := fmt.Sprintf(":%d", *port)
		fmt.Printf("⏳ TimeCapsule Wayback Server running at http://localhost:%d\n", *port)
		fmt.Printf("📁 Data directory: %s\n", *dataDir)
		fmt.Printf("Press Ctrl+C to stop.\n")

		if err := srv.Start(addr); err != nil {
			fmt.Printf("Server exited: %v\n", err)
		}

	case "version":
		fmt.Printf("timecapsule v%s\n", version)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`⏳ TimeCapsule (v%s)
Personal Wayback Machine & Self-Hosted Web Archival Engine in Go

Usage:
  timecapsule <command> [arguments]

Commands:
  save <url>        Capture and preserve a web page snapshot (HTML, WARC, Markdown)
  diff <old> <new>  Compare two historical snapshots with visual line diff
  watch <url>       Watch a webpage for updates and automatically preserve changes
  export <id>       Export snapshot as multi-record ISO 28500 WARC (optional --gzip)
  import <file>     Import an external ISO 28500 WARC archive into TimeCapsule
  cdx [url]         Generate standard CDXJ 1.1 capture index
  list              List all archived web page snapshots
  search <query>    Full-text search through preserved pages
  view <id>         Display the clean extracted markdown for a snapshot
  serve             Launch the local Wayback Machine web dashboard and replay server
  version           Show TimeCapsule version

Examples:
  timecapsule save https://news.ycombinator.com
  timecapsule export tc_123 --out archive.warc.gz --gzip
  timecapsule import external_archive.warc.gz
  timecapsule cdx https://news.ycombinator.com
  timecapsule diff snap_123 snap_456
  timecapsule watch https://github.com/trending --interval 1m
  timecapsule serve --port 8080
  timecapsule search "artificial intelligence"
`, version)
}
