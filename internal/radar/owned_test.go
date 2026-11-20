package radar

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func albumPage(albums []LibraryAlbum) string {
	b, _ := json.Marshal(map[string]any{"subsonic-response": map[string]any{"status": "ok", "albumList2": map[string]any{"album": albums}}})
	return string(b)
}
func TestOwnedMatchingIsExactAndTypeAware(t *testing.T) {
	artist := Artist{ID: "artist-1", Name: "Example Artist"}
	r := Release{ID: releaseID, Title: "A Title", Type: "Album"}
	a := LibraryAlbum{ID: "local-1", Name: " a  TITLE ", ArtistID: "artist-1", Artist: "Different display name"}
	if !ownedAlbumMatches(a, artist, r) {
		t.Fatal("exact normalized artist-ID/title match failed")
	}
	a.ArtistID = "other"
	if ownedAlbumMatches(a, artist, r) {
		t.Fatal("different artist hidden")
	}
	a.ArtistID = ""
	a.Artist = "example ARTIST"
	if !ownedAlbumMatches(a, artist, r) {
		t.Fatal("name fallback failed")
	}
	a.Name = "A Title (Deluxe)"
	if ownedAlbumMatches(a, artist, r) {
		t.Fatal("fuzzy edition match")
	}
	a.Name = r.Title
	a.ReleaseTypes = []string{"album", "live"}
	if ownedAlbumMatches(a, artist, r) {
		t.Fatal("live/studio mismatch")
	}
	r.Secondary = []string{"Live"}
	if !ownedAlbumMatches(a, artist, r) {
		t.Fatal("tagged live match failed")
	}
	r.Type = "Single"
	if ownedAlbumMatches(a, artist, r) {
		t.Fatal("single/album mismatch")
	}
}
func TestOwnedScanCommitsOnlyCompletePagesAndCanRefresh(t *testing.T) {
	s := State{}
	if !s.OwnedScanDue(testNow, false) {
		t.Fatal("initial scan not queued")
	}
	albums := []LibraryAlbum{}
	for i := 0; i < 500; i++ {
		albums = append(albums, LibraryAlbum{ID: fmt.Sprint(i), Name: "Title"})
	}
	complete, err := s.OwnedPage(albumPage(albums), testNow)
	if err != nil || complete || s.OwnedReady {
		t.Fatal("partial page committed", err)
	}
	b, _ := json.Marshal(s)
	json.Unmarshal(b, &s)
	complete, err = s.OwnedPage(albumPage(nil), testNow)
	if err != nil || !complete || !s.OwnedReady || len(s.OwnedAlbums) != 500 {
		t.Fatal("final page did not commit", err)
	}
	if s.OwnedScanDue(testNow.Add(time.Minute), false) {
		t.Fatal("inventory scanned too often")
	}
	if !s.OwnedScanDue(testNow.Add(time.Minute), true) {
		t.Fatal("manual refresh ignored")
	}
	if _, err = s.OwnedPage(`{"subsonic-response":{"status":"failed"}}`, testNow); err == nil || len(s.OwnedAlbums) != 500 {
		t.Fatal("failure discarded snapshot")
	}
	s.OwnedPage(albumPage(nil), testNow)
	if len(s.OwnedAlbums) != 0 {
		t.Fatal("removed local albums retained")
	}
}
func TestOwnedFilteringCoversHTMLCalendarAndDeliveryEvents(t *testing.T) {
	s := published()
	c := cfg()
	c.ExcludeOwned = true
	if len(Events(s, c, c.EventNamespace(), testNow, true)) != 0 {
		t.Fatal("unverified inventory used")
	}
	c.OwnershipReady = true
	c.OwnedAlbums = []LibraryAlbum{{ID: "local-1", ArtistID: s.Entries[0].Artist.ID, Name: s.Entries[0].Releases[0].Title}}
	if len(Events(s, c, c.EventNamespace(), testNow, false)) != 0 {
		t.Fatal("owned release reached notifications/calendar")
	}
	b, _ := Render(s, c, testNow)
	if strings.Contains(string(b), `<article class="release"`) {
		t.Fatal("owned release in HTML")
	}
	c.ExcludeOwned = false
	if len(Events(s, c, c.EventNamespace(), testNow, true)) != 1 {
		t.Fatal("disabled filter still active")
	}
}
