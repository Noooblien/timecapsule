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
	"github.com/Noooblien/timecapsule/pkg/bench"
	"github.com/Noooblien/timecapsule/pkg/bundle"
	"github.com/Noooblien/timecapsule/pkg/diff"
	"github.com/Noooblien/timecapsule/pkg/reader"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/visualdiff"
	"github.com/Noooblien/timecapsule/pkg/warc"
	"github.com/Noooblien/timecapsule/pkg/watcher"
)

const version = "1.5.0"

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

	case "vdiff":
		vdiffCmd := flag.NewFlagSet("vdiff", flag.ExitOnError)
		dataDir := vdiffCmd.String("dir", defaultDataDir, "Storage directory")
		demo := vdiffCmd.Bool("demo", false, "Run 4-scenario temporal visual diff demo suite")
		mermaid := vdiffCmd.Bool("mermaid", false, "Render Mermaid DOM mutation subtree diagram")
		markdown := vdiffCmd.Bool("markdown", false, "Render Markdown summary report")
		htmlOut := vdiffCmd.String("html", "", "Output dual-pane interactive visual diff HTML file")
		_ = vdiffCmd.Parse(os.Args[2:])

		if *demo {
			out, err := visualdiff.RunDemo(*mermaid, *markdown)
			if err != nil {
				fmt.Printf("Visual diff demo failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(out)
			return
		}

		args := vdiffCmd.Args()
		if len(args) < 2 {
			fmt.Println("Error: Two snapshot IDs required. Usage: timecapsule vdiff <old_id> <new_id> [--demo] [--mermaid] [--markdown] [--html <out.html>]")
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

		engine := visualdiff.NewEngine(visualdiff.DefaultVisualDiffConfig())
		res := engine.CompareSnapshots(oldSnap, newSnap)

		if *htmlOut != "" {
			htmlContent := visualdiff.RenderVisualDiffHTML(res, oldSnap, newSnap)
			if err := os.WriteFile(*htmlOut, []byte(htmlContent), 0644); err != nil {
				fmt.Printf("Failed to write HTML diff file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✅ Dual-pane visual diff HTML written to %s (%d bytes)\n", *htmlOut, len(htmlContent))
			return
		}

		if *markdown {
			fmt.Println(visualdiff.RenderMarkdownReport(res))
		} else {
			fmt.Print(visualdiff.RenderVisualScorecard(res))
		}

		if *mermaid {
			fmt.Println(visualdiff.RenderMermaidDOMTree(res))
		}

	case "watch":
		watchCmd := flag.NewFlagSet("watch", flag.ExitOnError)
		dataDir := watchCmd.String("dir", defaultDataDir, "Storage directory")
		demo := watchCmd.Bool("demo", false, "Run multi-scenario continuous watcher simulation demo")
		mermaid := watchCmd.Bool("mermaid", false, "Render Mermaid timeline of detected drift events")
		markdown := watchCmd.Bool("markdown", false, "Render Markdown summary report")
		addURL := watchCmd.String("add", "", "Register a URL for continuous temporal monitoring")
		name := watchCmd.String("name", "", "Human-readable label for the watch target")
		webhook := watchCmd.String("webhook", "", "Optional webhook URL for drift notifications")
		threshold := watchCmd.Float64("threshold", 0.05, "Drift sensitivity threshold (default 0.05 = 5%)")
		ignore := watchCmd.String("ignore", "", "Comma-separated CSS classes/IDs to ignore (e.g. ad-banner,timestamp)")
		list := watchCmd.Bool("list", false, "Display status scorecard of all registered watch targets")
		poll := watchCmd.Bool("poll", false, "Execute on-demand check cycle across all targets")
		interval := watchCmd.Duration("interval", 5*time.Minute, "Check interval (e.g. 30s, 5m, 1h)")
		timeout := watchCmd.Duration("timeout", 15*time.Second, "Fetch timeout")
		_ = watchCmd.Parse(os.Args[2:])

		if *demo {
			out, err := watcher.RunWatcherDemo(*mermaid, *markdown)
			if err != nil {
				fmt.Printf("Demo failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(out)
			return
		}

		store, err := storage.NewStore(*dataDir)
		if err != nil {
			fmt.Printf("Storage error: %v\n", err)
			os.Exit(1)
		}

		archiver := archive.NewArchiver(*timeout)
		engine, err := watcher.NewEngine(store, archiver, *dataDir)
		if err != nil {
			fmt.Printf("Watcher engine error: %v\n", err)
			os.Exit(1)
		}

		if *addURL != "" {
			var ignoreSelectors []string
			if *ignore != "" {
				for _, s := range strings.Split(*ignore, ",") {
					if tr := strings.TrimSpace(s); tr != "" {
						ignoreSelectors = append(ignoreSelectors, tr)
					}
				}
			}
			t, err := engine.AddTarget(*addURL, *name, (*interval).String(), *webhook, *threshold, ignoreSelectors, nil)
			if err != nil {
				fmt.Printf("Failed to add target: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✨ Watch target registered successfully:\n")
			fmt.Printf("   ID:       %s\n", t.ID)
			fmt.Printf("   Name:     %s\n", t.Name)
			fmt.Printf("   URL:      %s\n", t.URL)
			fmt.Printf("   Interval: %s\n", t.IntervalStr)
			if t.WebhookURL != "" {
				fmt.Printf("   Webhook:  %s\n", t.WebhookURL)
			}
			return
		}

		if *list {
			targets := engine.ListTargets()
			events := engine.GetEvents("")
			if *markdown {
				fmt.Println(watcher.RenderMarkdownReport(targets, events))
			} else if *mermaid {
				fmt.Println(watcher.RenderMermaidTimeline(events))
			} else {
				fmt.Print(watcher.RenderWatcherScorecard(targets, events))
			}
			return
		}

		if *poll {
			fmt.Println("🔍 Executing on-demand polling cycle across active targets...")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			events := engine.PollAll(ctx)
			targets := engine.ListTargets()
			fmt.Printf("✅ Polling completed: %d targets checked, %d mutations detected.\n", len(targets), len(events))
			fmt.Print(watcher.RenderWatcherScorecard(targets, engine.GetEvents("")))
			return
		}

		// Positional argument fallback: single URL live watching
		args := watchCmd.Args()
		if len(args) < 1 {
			targets := engine.ListTargets()
			if len(targets) > 0 {
				fmt.Print(watcher.RenderWatcherScorecard(targets, engine.GetEvents("")))
				return
			}
			fmt.Println("Usage: timecapsule watch <url> [--interval 30s] | --demo | --list | --add <url>")
			os.Exit(1)
		}

		targetURL := args[0]
		target, err := engine.AddTarget(targetURL, targetURL, (*interval).String(), *webhook, *threshold, nil, nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("👁️  Watching %s every %v for content changes (Ctrl+C to stop)...\n", targetURL, *interval)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		checkFunc := func() {
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()

			ev, err := engine.PollTarget(ctx, target.ID)
			if err != nil {
				fmt.Printf("[%s] ⚠️  Check failed: %v\n", time.Now().Format("15:04:05"), err)
				return
			}
			if ev == nil {
				fmt.Printf("[%s] 💤 No drift detected (hash: %s)\n", time.Now().Format("15:04:05"), target.LastHash[:8])
			} else {
				fmt.Printf("\n🔔 [%s] %s Drift detected! (%s, delta: %.1f%%)\n",
					time.Now().Format("15:04:05"), ev.Severity, ev.Summary, ev.DeltaRatio*100)
				fmt.Printf("   Old Snapshot: %s | New Snapshot: %s (+%d / -%d lines)\n\n",
					ev.OldSnapshotID[:8], ev.NewSnapshotID[:8], ev.LinesAdded, ev.LinesDeleted)
			}
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

	case "reader":
		readerCmd := flag.NewFlagSet("reader", flag.ExitOnError)
		dataDir := readerCmd.String("dir", defaultDataDir, "Storage directory")
		outFile := readerCmd.String("out", "", "Output Markdown file path (prints to stdout if omitted)")
		frontmatter := readerCmd.Bool("frontmatter", false, "Include YAML frontmatter (title, author, date, word_count)")
		htmlOut := readerCmd.Bool("html", false, "Output standalone distraction-free Reader View HTML instead of Markdown")
		timeout := readerCmd.Duration("timeout", 15*time.Second, "Fetch timeout if target is a live URL")
		_ = readerCmd.Parse(os.Args[2:])

		args := readerCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: Snapshot ID or URL required. Usage: timecapsule reader <id|url> [--out <file.md>] [--frontmatter] [--html]")
			os.Exit(1)
		}
		target := args[0]
		store, _ := storage.NewStore(*dataDir)
		var targetURL, rawHTML, title string

		if store != nil {
			if snap, ok := store.Get(target); ok {
				targetURL = snap.URL
				rawHTML = snap.HTML
				title = snap.Title
			}
		}

		if rawHTML == "" {
			targetURL = target
			if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
				targetURL = "https://" + targetURL
			}
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			defer cancel()
			archiver := archive.NewArchiver(*timeout)
			snap, err := archiver.Capture(ctx, targetURL)
			if err != nil {
				fmt.Printf("❌ Failed to fetch URL %s: %v\n", targetURL, err)
				os.Exit(1)
			}
			rawHTML = snap.HTML
			title = snap.Title
		}

		opts := reader.DefaultOptions(targetURL)
		opts.IncludeFrontmatter = *frontmatter
		article := reader.ExtractArticle(rawHTML, opts)
		if article.Title == "Untitled Document" && title != "" {
			article.Title = title
		}

		var output string
		if *htmlOut {
			output = reader.RenderReaderHTML(article)
		} else if *frontmatter {
			output = article.ToMarkdownWithFrontmatter()
		} else {
			output = article.Content
		}

		if *outFile != "" {
			if err := os.WriteFile(*outFile, []byte(output), 0644); err != nil {
				fmt.Printf("❌ Failed to write file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✅ Reader content written to %s (%d words, ~%d min read)\n", *outFile, article.WordCount, article.ReadingTimeMin)
		} else {
			fmt.Println(output)
		}

	case "bundle":
		bundleCmd := flag.NewFlagSet("bundle", flag.ExitOnError)
		dataDir := bundleCmd.String("dir", defaultDataDir, "Storage directory")
		outFile := bundleCmd.String("out", "", "Output single-file HTML path (default: <title/host>.single.html)")
		safe := bundleCmd.Bool("safe", true, "Neutralize scripts for clean, safe offline reading")
		timeout := bundleCmd.Duration("timeout", 15*time.Second, "Fetch timeout for sub-resources")
		_ = bundleCmd.Parse(os.Args[2:])

		args := bundleCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: Snapshot ID or URL required. Usage: timecapsule bundle <id|url> [--out <file.html>] [--safe]")
			os.Exit(1)
		}
		target := args[0]

		cfg := bundle.DefaultBundleConfig()
		cfg.NeutralizeScripts = *safe
		cfg.Timeout = *timeout
		bundler := bundle.NewBundler(cfg)

		store, _ := storage.NewStore(*dataDir)
		var targetURL, rawHTML string

		// Check if target is a known snapshot ID
		if store != nil {
			if snap, ok := store.Get(target); ok {
				targetURL = snap.URL
				rawHTML = snap.HTML
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeout*2)
		defer cancel()

		if rawHTML == "" {
			// Target is a URL: fetch raw HTML first
			targetURL = target
			if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
				targetURL = "https://" + targetURL
			}
			archiver := archive.NewArchiver(*timeout)
			snap, err := archiver.Capture(ctx, targetURL)
			if err != nil {
				fmt.Printf("❌ Failed to capture %s: %v\n", targetURL, err)
				os.Exit(1)
			}
			rawHTML = snap.HTML
		}

		fmt.Printf("⏳ Serializing verbatim DOM and inlining assets for %s...\n", targetURL)
		res, err := bundler.BundleHTML(ctx, targetURL, rawHTML)
		if err != nil {
			fmt.Printf("❌ Failed to bundle DOM: %v\n", err)
			os.Exit(1)
		}

		outPath := *outFile
		if outPath == "" {
			safeName := strings.ReplaceAll(strings.ReplaceAll(targetURL, "https://", ""), "http://", "")
			safeName = strings.ReplaceAll(safeName, "/", "_")
			if len(safeName) > 40 {
				safeName = safeName[:40]
			}
			outPath = fmt.Sprintf("%s.single.html", safeName)
		}

		if err := os.WriteFile(outPath, []byte(res.BundledHTML), 0644); err != nil {
			fmt.Printf("❌ Failed to write single-file bundle: %v\n", err)
			os.Exit(1)
		}

		fmt.Print(bundle.RenderBundleScorecard(res))
		fmt.Printf("\n✅ Single-file hermetic HTML written to %s (%d bytes)\n\n", outPath, res.BundledSizeBytes)

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

	case "bench":
		benchCmd := flag.NewFlagSet("bench", flag.ExitOnError)
		demo := benchCmd.Bool("demo", false, "Run comprehensive 4-corpus archival benchmark suite")
		strict := benchCmd.Bool("strict", false, "Exit with code 1 if benchmark suite fails")
		markdown := benchCmd.Bool("markdown", false, "Output GitHub Markdown summary table")
		jsonOutput := benchCmd.Bool("json", false, "Output structured JSON report")
		_ = benchCmd.Parse(os.Args[2:])

		if *demo || len(os.Args) == 2 {
			suite := bench.GenerateDemoSuite()
			if *markdown {
				fmt.Println(bench.RenderMarkdown(suite))
			} else if *jsonOutput {
				js, _ := bench.RenderJSON(suite)
				fmt.Println(js)
			} else {
				fmt.Print(bench.RenderSuiteScorecard(suite))
			}
			if *strict && !suite.OverallPassed {
				os.Exit(1)
			}
			return
		}

	case "verify":
		verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
		dataDir := verifyCmd.String("dir", defaultDataDir, "Storage directory")
		strict := verifyCmd.Bool("strict", false, "Exit with code 1 if verification detects leaks or invalid ISO WARC")
		_ = verifyCmd.Parse(os.Args[2:])

		args := verifyCmd.Args()
		if len(args) < 1 {
			fmt.Println("Error: Target required. Usage: timecapsule verify <snapshot_id|file.warc|file.html> [--strict]")
			os.Exit(1)
		}
		target := args[0]

		engine := bench.NewEngine()
		store, _ := storage.NewStore(*dataDir)

		// 1. Try resolving as snapshot ID
		if store != nil {
			if snap, ok := store.Get(target); ok {
				res, err := engine.EvaluateSnapshot(snap)
				if err != nil {
					fmt.Printf("Verification error: %v\n", err)
					os.Exit(1)
				}
				fmt.Print(bench.RenderCorpusScorecard(res))
				if *strict && !res.Passed {
					os.Exit(1)
				}
				return
			}
		}

		// 2. Try resolving as local file
		fileBytes, err := os.ReadFile(target)
		if err != nil {
			fmt.Printf("Error reading target '%s': %v\n", target, err)
			os.Exit(1)
		}

		if strings.HasSuffix(target, ".warc") || strings.HasSuffix(target, ".warc.gz") {
			iso := bench.ValidateWARC(fileBytes)
			fmt.Printf("\n=== ISO 28500 WARC Conformance Audit for %s ===\n", target)
			fmt.Printf("Valid Format:      %v\n", iso.ValidFormat)
			fmt.Printf("WARC Info Record:  %v\n", iso.HasWarcInfo)
			fmt.Printf("Response Record:   %v\n", iso.HasResponse)
			fmt.Printf("Digest Integrity:  %v\n", iso.DigestMatch)
			fmt.Printf("Total Records:     %d\n", iso.RecordCount)
			if len(iso.Errors) > 0 {
				fmt.Printf("Errors:\n")
				for _, e := range iso.Errors {
					fmt.Printf("  • %s\n", e)
				}
			}
			if *strict && (!iso.ValidFormat || !iso.DigestMatch) {
				os.Exit(1)
			}
		} else {
			// Audit as HTML
			leaks, score := bench.AuditHTML(string(fileBytes))
			fmt.Printf("\n=== Hermetic Isolation & Privacy Leak Audit for %s ===\n", target)
			fmt.Printf("Hermetic Score: %.1f / 100.0\n", score)
			fmt.Printf("Detected Leaks: %d\n", len(leaks))
			for _, l := range leaks {
				fmt.Printf("  [%s] %s: %s (%s)\n", l.Severity, l.ElementType, l.URL, l.Description)
			}
			if *strict && score < 85.0 {
				os.Exit(1)
			}
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
  reader <target>   Convert webpage/snapshot into clean reader Markdown or standalone HTML
  bundle <target>   Serialize verbatim DOM into a self-contained single-file HTML bundle
  verify <id|file>  Verify archival fidelity, ISO 28500 WARC compliance, and leak isolation
  bench             Run empirical archival fidelity and compression benchmark suite
  diff <old> <new>  Compare two historical snapshots with visual line diff
  vdiff <old> <new> Temporal visual & DOM tree diff with dual-pane HTML, Mermaid & scorecards
  watch [options]   Continuous website drift watcher, change alerts & automated snapshots
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
  timecapsule reader https://news.ycombinator.com --frontmatter
  timecapsule bundle https://news.ycombinator.com --out hn.single.html
  timecapsule watch --demo
  timecapsule watch --add https://github.com/trending --interval 10m
  timecapsule watch --list
  timecapsule export tc_123 --out archive.warc.gz --gzip
  timecapsule import external_archive.warc.gz
  timecapsule cdx https://news.ycombinator.com
  timecapsule diff snap_123 snap_456
  timecapsule vdiff --demo --mermaid
  timecapsule vdiff snap_123 snap_456 --html diff.html
  timecapsule watch https://github.com/trending --interval 1m
  timecapsule serve --port 8080
  timecapsule search "artificial intelligence"
`, version)
}
