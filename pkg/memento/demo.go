package memento

import (
	"fmt"
	"time"
)

// DemoScenario represents a simulated client requesting a temporal snapshot.
type DemoScenario struct {
	Name          string
	Description   string
	RequestedDate string
	OriginalURL   string
}

// DemoSummary aggregates results of running the Memento demonstration suite.
type DemoSummary struct {
	TotalScenarios int
	Successful     int
	Resolutions    []*TemporalResolution
	LinkHeaders    []string
	Scenarios      []DemoScenario
}

// RunMementoDemo executes 4 temporal negotiation test scenarios across a realistic timeline.
func RunMementoDemo() *DemoSummary {
	baseURL := "https://archive.timecapsule.dev"
	originalURL := "https://techcrunch.com/artificial-intelligence"

	// Timeline of 4 historical captures
	t1, _ := time.Parse(time.RFC3339, "2023-03-14T17:00:00Z") // GPT-4 launch
	t2, _ := time.Parse(time.RFC3339, "2024-02-15T18:30:00Z") // Sora announcement
	t3, _ := time.Parse(time.RFC3339, "2025-01-20T12:00:00Z") // DeepSeek-R1 release
	t4, _ := time.Parse(time.RFC3339, "2026-06-10T15:00:00Z") // Contemporary snapshot

	mementos := []*Memento{
		{
			URI:         fmt.Sprintf("%s/web/20230314170000/%s", baseURL, originalURL),
			Datetime:    t1,
			OriginalURL: originalURL,
			MimeType:    "text/html; charset=utf-8",
			StatusCode:  200,
			Digest:      "sha256:7b5f0a21...",
		},
		{
			URI:         fmt.Sprintf("%s/web/20240215183000/%s", baseURL, originalURL),
			Datetime:    t2,
			OriginalURL: originalURL,
			MimeType:    "text/html; charset=utf-8",
			StatusCode:  200,
			Digest:      "sha256:8c1d3e42...",
		},
		{
			URI:         fmt.Sprintf("%s/web/20250120120000/%s", baseURL, originalURL),
			Datetime:    t3,
			OriginalURL: originalURL,
			MimeType:    "text/html; charset=utf-8",
			StatusCode:  200,
			Digest:      "sha256:9e4f5a63...",
		},
		{
			URI:         fmt.Sprintf("%s/web/20260610150000/%s", baseURL, originalURL),
			Datetime:    t4,
			OriginalURL: originalURL,
			MimeType:    "text/html; charset=utf-8",
			StatusCode:  200,
			Digest:      "sha256:0a5b6c74...",
		},
	}

	tg := NewTimeGate(originalURL, baseURL, mementos)

	scenarios := []DemoScenario{
		{
			Name:          "Scenario 1: Exact Hit on Major Historical Event",
			Description:   "Client sends Accept-Datetime matching the exact second of a historical capture (2024-02-15).",
			RequestedDate: "Thu, 15 Feb 2024 18:30:00 GMT",
			OriginalURL:   originalURL,
		},
		{
			Name:          "Scenario 2: Nearest-Neighbor Temporal Interpolation",
			Description:   "Client queries mid-2024 (2024-08-01); TimeGate smoothly interpolates and selects closest neighbor.",
			RequestedDate: "Thu, 01 Aug 2024 09:00:00 GMT",
			OriginalURL:   originalURL,
		},
		{
			Name:          "Scenario 3: Pre-Archive Boundary Clamp",
			Description:   "Client queries a 2021 date before the earliest archive; TimeGate safely clamps to the first memento.",
			RequestedDate: "Mon, 15 Nov 2021 14:00:00 GMT",
			OriginalURL:   originalURL,
		},
		{
			Name:          "Scenario 4: Future Query Post-Archive Clamp",
			Description:   "Client queries a future 2027 date; TimeGate clamps to the latest available capture.",
			RequestedDate: "Fri, 01 Jan 2027 00:00:00 GMT",
			OriginalURL:   originalURL,
		},
	}

	summary := &DemoSummary{
		TotalScenarios: len(scenarios),
		Scenarios:      scenarios,
	}

	for _, sc := range scenarios {
		reqTime, _ := ParseHTTPDate(sc.RequestedDate)
		res, err := tg.Resolve(reqTime)
		if err == nil {
			summary.Successful++
			summary.Resolutions = append(summary.Resolutions, res)
			summary.LinkHeaders = append(summary.LinkHeaders, tg.BuildLinkHeader(res))
		}
	}

	return summary
}
