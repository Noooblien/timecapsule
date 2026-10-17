package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

const version = "1.0.0"

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
  list              List all archived web page snapshots
  search <query>    Full-text search through preserved pages
  view <id>         Display the clean extracted markdown for a snapshot
  serve             Launch the local Wayback Machine web dashboard and replay server
  version           Show TimeCapsule version

Examples:
  timecapsule save https://news.ycombinator.com
  timecapsule serve --port 8080
  timecapsule search "artificial intelligence"
`, version)
}
