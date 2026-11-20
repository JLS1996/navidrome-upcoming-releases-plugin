package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func creditPage(kind string, records []directCredit) string {
	container, key := "albumList2", "album"
	if kind == "tracks" {
		container, key = "searchResult3", "song"
	}
	b, _ := json.Marshal(map[string]any{"subsonic-response": map[string]any{"status": "ok", container: map[string]any{key: records}}})
	return string(b)
}
func TestDirectCreditsRequireWholeStandaloneNames(t *testing.T) {
	var inventory CreditInventory
	inventory.ScanDue(testNow, false)
	_, err := inventory.Page(creditPage("albums", []directCredit{
		{ID: "collaboration", Artist: "Fixture Lead & Fixture Guest", ArtistID: "guest"},
		{ID: "solo", Artist: "Album Only", ArtistID: "album-only"},
		{ID: "same-name", Artist: "Namesake", ArtistID: "other-person"},
		{ID: "legacy", Artist: " Legacy  Name "},
	}), testNow)
	if err != nil {
		t.Fatal(err)
	}
	done, err := inventory.Page(creditPage("tracks", []directCredit{{ID: "song", Artist: "fixture LEAD", ArtistID: "lead"}, {ID: "featured", Artist: "Fixture Lead feat. Featured Guest", ArtistID: "featured"}}), testNow)
	if err != nil || !done {
		t.Fatal(done, err)
	}
	c := DefaultConfig()
	c.ExcludeUncredited = true
	c.DirectReady = inventory.Ready
	c.DirectCredits = inventory.Credits
	for _, tc := range []struct {
		artist Artist
		want   bool
	}{
		{Artist{ID: "lead", Name: "Fixture Lead"}, true},
		{Artist{ID: "album-only", Name: "album ONLY"}, true},
		{Artist{ID: "guest", Name: "Fixture Guest"}, false},
		{Artist{ID: "featured", Name: "Featured Guest"}, false},
		{Artist{ID: "similar", Name: "Guest Extended"}, false},
		{Artist{ID: "namesake", Name: "Namesake"}, false},
		{Artist{ID: "legacy", Name: "legacy name"}, true},
	} {
		if got := IncludedArtist(tc.artist, c); got != tc.want {
			t.Errorf("%s included=%t want=%t", tc.artist.Name, got, tc.want)
		}
	}
	c.ExcludeUncredited = false
	if !IncludedArtist(Artist{Name: "Fixture Guest"}, c) {
		t.Fatal("filter must be opt-in")
	}
}

func TestDirectScanPaginationPersistenceAndAtomicSnapshot(t *testing.T) {
	s := State{Schema: 1}
	s.Direct = CreditInventory{Ready: true, At: testNow.Add(-time.Hour).Unix(), Credits: map[string]bool{creditKey("Previous", "previous"): true}}
	if !s.Direct.ScanDue(testNow, false) {
		t.Fatal("stale scan not started")
	}
	albums := make([]directCredit, 500)
	for i := range albums {
		albums[i] = directCredit{ID: "album", Artist: "New Artist", ArtistID: "new"}
	}
	if done, err := s.Direct.Page(creditPage("albums", albums), testNow); err != nil || done || s.Direct.Offset != 500 {
		t.Fatal(done, err, s.Direct)
	}
	dir := t.TempDir()
	b, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(dir, "state.json"), b, 0600)
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	s = loaded
	if s.Direct.Offset != 500 || !s.Direct.Credits[creditKey("Previous", "previous")] {
		t.Fatal("scan progress/snapshot lost")
	}
	if _, err := s.Direct.Page(`{"subsonic-response":{"status":"ok"}}`, testNow); err == nil {
		t.Fatal("missing album result accepted")
	}
	if _, err := s.Direct.Page(creditPage("albums", nil), testNow); err != nil {
		t.Fatal(err)
	}
	if s.Direct.Phase != "tracks" || s.Direct.Offset != 0 {
		t.Fatal("track phase did not reset offset")
	}
	if _, err := s.Direct.Page(creditPage("tracks", []directCredit{{ID: "no-credit"}}), testNow); err == nil {
		t.Fatal("missing artist credit accepted")
	}
	s.Direct.Restart(testNow)
	if !s.Direct.Credits[creditKey("Previous", "previous")] || s.Direct.RetryAt != testNow.Unix()+60 || len(s.Direct.Pending) != 0 {
		t.Fatal("failed scan replaced snapshot")
	}
	s.Direct.Page(creditPage("albums", nil), testNow)
	if done, err := s.Direct.Page(creditPage("tracks", []directCredit{{ID: "song", Artist: "New Artist", ArtistID: "new"}}), testNow); err != nil || !done {
		t.Fatal(done, err)
	}
	if s.Direct.Credits[creditKey("Previous", "previous")] || !s.Direct.Credits[creditKey("New Artist", "new")] {
		t.Fatal("complete snapshot not replaced")
	}
	if s.Direct.ScanDue(testNow.Add(time.Minute), false) || !s.Direct.ScanDue(testNow.Add(time.Minute), true) {
		t.Fatal("refresh timing incorrect")
	}
	s.Direct.Offset = 2000001
	if s.Direct.Validate() == nil {
		t.Fatal("invalid persistent offset accepted")
	}
	s.Direct.Offset = 100000
	s.Direct.Phase = "albums"
	if _, err := s.Direct.Page(creditPage("albums", []directCredit{{ID: "too-many", Artist: "New Artist"}}), testNow); err == nil {
		t.Fatal("scan limit ignored")
	}
}

func TestDirectFilteringSkipsLookupsIssuesAndExportsAndRestoresCache(t *testing.T) {
	s := published()
	a := s.Entries[0].Artist
	c := cfg()
	c.ExcludeUncredited = true
	pending := Entry{Artist: Artist{ID: "guest", Name: "Fixture Guest"}, Phase: "match", Problem: "Match needs attention"}
	s.Entries = append(s.Entries, pending)
	if FullCycle(s, c) {
		t.Fatal("incomplete inventory marked complete")
	}
	c.DirectReady = true
	c.DirectCredits = map[string]bool{creditKey(a.Name, a.ID): true}
	if !FullCycle(s, c) {
		t.Fatal("excluded artist blocks completion")
	}
	calls := 0
	if s.Step(func(string) (Response, error) { calls++; return Response{}, nil }, c, testNow) || calls != 0 {
		t.Fatal("excluded artist requested MusicBrainz")
	}
	body, _ := Render(s, c, testNow)
	if strings.Contains(string(body), "Fixture Guest") || len(Events(s, c, c.EventNamespace(), testNow, false)) != 1 {
		t.Fatal("excluded artist leaked into report or eligible event lost")
	}
	c.DirectCredits = nil
	body, _ = Render(s, c, testNow)
	if strings.Contains(string(body), `<article class="release"`) || len(Events(s, c, c.EventNamespace(), testNow, false)) != 0 {
		t.Fatal("excluded releases exported")
	}
	d := DeliveryState{}
	ic := icfg()
	ic.NtfyTopic = "fixture-topic"
	d.Step(s, c, ic, func(HTTPRequest) (Response, error) { t.Fatal("excluded release notified"); return Response{}, nil }, testNow)
	if !d.NtfyReady {
		t.Fatal("excluded pending artist prevented notification baseline")
	}
	c.ExcludeUncredited = false
	if len(Events(s, c, c.EventNamespace(), testNow, false)) != 1 || s.Entries[0].Releases[0].Title == "" {
		t.Fatal("toggle failed to restore cached releases")
	}
	if FullCycle(s, c) {
		t.Fatal("restored pending artist ignored")
	}
}
