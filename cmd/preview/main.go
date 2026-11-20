// preview generates deterministic, explicitly labelled sample data. It makes no network requests.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"github.com/JLS1996/navidrome-upcoming-releases-plugin/internal/radar"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"time"
)

func main() {
	theme := flag.String("theme", "dark", "dashboard theme: bright, dark or black")
	art := flag.Bool("covers", false, "include original synthetic demo artwork")
	out := flag.String("out", "dist/preview.html", "output path")
	flag.Parse()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	c := radar.DefaultConfig()
	c.Theme = *theme
	c.Username = "demo"
	c.PrimaryTypes = append(c.PrimaryTypes, "Single")
	c.SecondaryTypes = []string{"Live"}
	s := radar.State{Schema: 1, Problem: "DEMO DATA — this preview is not a live release announcement feed.", Entries: []radar.Entry{
		{Artist: radar.Artist{ID: "sample-1", Name: "Sample artist · Northbound"}, Phase: "done", CheckedAt: now.Format(time.RFC3339), Releases: []radar.Release{{ID: "11111111-1111-4111-8111-111111111111", Title: "A Place in the Morning (sample)", Date: "2026-12-04", Type: "Album"}, {ID: "22222222-2222-4222-8222-222222222222", Title: "Slow Light (sample)", Date: "2026-11-06", Type: "EP", Secondary: []string{"Live"}}}},
		{Artist: radar.Artist{ID: "sample-2", Name: "Sample artist · Glass Orchard"}, Phase: "done", CheckedAt: now.Format(time.RFC3339), Releases: []radar.Release{{ID: "33333333-3333-4333-8333-333333333333", Title: "Signals / After Hours (sample)", Date: "2026-12", Type: "Single"}}},
		{Artist: radar.Artist{ID: "sample-3", Name: "Ambiguous sample artist"}, Phase: "unresolved", Problem: "Multiple artists share this name. Set an artist ID override."},
	}}
	if *art {
		c.CoverArt = true
		c.Covers = map[string]string{}
		ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"}
		for i, id := range ids {
			if i == 1 {
				continue
			}
			im := image.NewRGBA(image.Rect(0, 0, 250, 250))
			for y := 0; y < 250; y++ {
				for x := 0; x < 250; x++ {
					wave := (math.Sin(float64(x+y)/28+float64(i)) + 1) / 2
					r, g, b := uint8(35+wave*85), uint8(65+float64(y)/2), uint8(90+float64(x)/3)
					if i == 1 {
						r, g, b = g, uint8(40+wave*100), r
					}
					if i == 2 {
						r, g, b = b, r, g
					}
					if (x-125)*(x-125)+(y-105)*(y-105) < 3600 {
						r, g, b = uint8(170+wave*60), uint8(180+wave*50), 110
					}
					im.Set(x, y, color.RGBA{r, g, b, 255})
				}
			}
			var data bytes.Buffer
			png.Encode(&data, im)
			c.Covers[id] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
		}
	}
	b, err := radar.Render(s, c, now)
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, b, 0644); err != nil {
		panic(err)
	}
}
