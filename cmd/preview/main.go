// preview generates deterministic, explicitly labelled sample data. It makes no network requests.
package main

import (
	"flag"
	"os"
	"release-radar/internal/radar"
	"time"
)

func main() {
	out := flag.String("out", "dist/preview.html", "output path")
	flag.Parse()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	c := radar.DefaultConfig()
	c.Username = "demo"
	c.IncludeSingles = true
	s := radar.State{Schema: 1, Problem: "DEMO DATA — this preview is not a live release announcement feed.", Entries: []radar.Entry{
		{Artist: radar.Artist{ID: "sample-1", Name: "Sample artist · Northbound"}, Phase: "done", CheckedAt: now.Format(time.RFC3339), Releases: []radar.Release{{ID: "11111111-1111-4111-8111-111111111111", Title: "A Place in the Morning (sample)", Date: "2026-12-04", Type: "Album"}, {ID: "22222222-2222-4222-8222-222222222222", Title: "Slow Light (sample)", Date: "2026-11-06", Type: "EP"}}},
		{Artist: radar.Artist{ID: "sample-2", Name: "Sample artist · Glass Orchard"}, Phase: "done", CheckedAt: now.Format(time.RFC3339), Releases: []radar.Release{{ID: "33333333-3333-4333-8333-333333333333", Title: "Signals / After Hours (sample)", Date: "2026-12", Type: "Single"}}},
		{Artist: radar.Artist{ID: "sample-3", Name: "Ambiguous sample artist"}, Phase: "unresolved", Problem: "Multiple artists share this name. Set an artist ID override."},
	}}
	b, err := radar.Render(s, c, now)
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, b, 0644); err != nil {
		panic(err)
	}
}
