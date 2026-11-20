package radar

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Explicitly opt in: normal tests and CI do not depend on the public API.
func TestLiveNorthlaneLookup(t *testing.T) {
	if os.Getenv("RADAR_LIVE_MUSICBRAINZ") != "1" {
		t.Skip("set RADAR_LIVE_MUSICBRAINZ=1 for a public API check")
	}
	c := DefaultConfig()
	c.Username = "live-check"
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	s := State{}
	s.Reconcile([]Artist{{ID: "public-live-check", Name: "Northlane", MBID: "f4e44995-1890-42a0-a6c1-9185b71655fa"}}, c, "live-check", now)
	client := http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	fetch := func(endpoint string) (Response, error) {
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return Response{}, err
		}
		req.Header.Set("User-Agent", MusicBrainzUserAgent())
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return Response{}, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		headers := map[string]string{}
		for key := range resp.Header {
			headers[key] = resp.Header.Get(key)
		}
		return Response{Status: resp.StatusCode, Headers: headers, Body: body}, err
	}
	if !s.Step(fetch, c, now) || s.Entries[0].Phase != "done" {
		t.Fatalf("live lookup did not complete: %s %s", s.MusicBrainzProblem, s.Entries[0].Problem)
	}
	events := Events(s, c, c.EventNamespace(), now, true)
	found := false
	for _, e := range events {
		if strings.Contains(e.Summary, "Anemoia") {
			found = true
			t.Logf("Northlane: %s (%s), %s", e.Summary, e.Date, e.URL)
		}
	}
	if !found {
		t.Fatal("MusicBrainz no longer places Anemoia in the check's November 2026 window; inspect public source metadata")
	}
	report, err := Render(s, c, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "Anemoia") || !strings.Contains(string(CalendarICS(events, now)), "Anemoia") {
		t.Fatal("live album missing from HTML/calendar exports")
	}
}

func TestLiveNorthlaneCoverArt(t *testing.T) {
	if os.Getenv("RADAR_LIVE_ARTWORK") != "1" {
		t.Skip("set RADAR_LIVE_ARTWORK=1 for the public artwork check")
	}
	id := "30a1f7ef-86cc-4411-8567-6362169cc36e"
	a := ArtworkState{Schema: 1, Records: map[string]*ArtworkRecord{id: {}}}
	client := http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(r HTTPRequest) (Response, error) {
		req, err := http.NewRequest(r.Method, r.URL, nil)
		if err != nil {
			return Response{}, err
		}
		for k, v := range r.Headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return Response{}, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtworkBytes+1))
		headers := map[string]string{}
		for k := range resp.Header {
			headers[k] = resp.Header.Get(k)
		}
		return Response{Status: resp.StatusCode, Headers: headers, Body: body}, err
	}
	for i := 0; i < 7; i++ {
		a.Step(send, time.Now().UTC())
		r := a.Records[id]
		if r.Data != "" {
			t.Logf("Fetched and validated an embedded Northlane thumbnail (%d encoded bytes)", len(r.Data))
			return
		}
		if r.CheckedAt != 0 {
			t.Skip("the public archive has no cover for this release")
		}
		if a.Problem != "" {
			t.Fatal(a.Problem)
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("artwork redirect chain did not complete")
}
