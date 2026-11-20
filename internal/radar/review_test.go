package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseSelectorsCoverMusicBrainzTypes(t *testing.T) {
	c := DefaultConfig()
	b, _ := Render(published(), c, testNow)
	for _, option := range append(append([]ReleaseTypeOption{}, PrimaryReleaseTypes...), SecondaryReleaseTypes...) {
		if !strings.Contains(string(b), `<option value="`+option.Value+`">`) {
			t.Fatal("HTML filter missing type", option.Value)
		}
	}
	for _, primary := range PrimaryReleaseTypes {
		r := Release{ID: releaseID, Type: primary.Value, Date: "2026-12-04"}
		c.PrimaryTypes = []string{primary.Value}
		if !Visible(r, c, testNow) {
			t.Fatal("primary selection failed", primary.Value)
		}
		for _, secondary := range SecondaryReleaseTypes {
			r.Secondary = []string{secondary.Value}
			c.SecondaryTypes = nil
			if Visible(r, c, testNow) {
				t.Fatal("secondary selection ignored", secondary.Value)
			}
			c.SecondaryTypes = []string{secondary.Value}
			if !Visible(r, c, testNow) {
				t.Fatal("secondary selection failed", secondary.Value)
			}
			c.PrimaryTypes = nil
			if Visible(r, c, testNow) {
				t.Fatal("secondary bypassed format selection")
			}
			c.PrimaryTypes = []string{primary.Value}
		}
	}
	c = DefaultConfig()
	c.IncludeStandard = false
	c.SecondaryTypes = []string{"Live"}
	r := Release{ID: releaseID, Type: "Album", Date: "2026-12-04"}
	if Visible(r, c, testNow) {
		t.Fatal("standard release shown when disabled")
	}
	r.Secondary = []string{"Compilation", "Live"}
	if !Visible(r, c, testNow) {
		t.Fatal("any selected secondary tag should match")
	}
	c.PrimaryTypes = []string{"Other"}
	c.IncludeStandard = true
	r.Type = ""
	r.Secondary = nil
	if !Visible(r, c, testNow) {
		t.Fatal("unspecified primary type must be selectable")
	}
	// Local timezone must not shift the UTC window by a day.
	c = DefaultConfig()
	c.PastDays = 0
	r.Type = "Album"
	r.Date = "2026-11-19"
	if Visible(r, c, time.Date(2026, 11, 19, 23, 30, 0, 0, time.FixedZone("local", -5*3600))) {
		t.Fatal("window used local date instead of UTC")
	}
}

func TestSharedReleaseHasConsistentCreditsAndNoDuplicates(t *testing.T) {
	s := published()
	other := s.Entries[0]
	other.Artist.ID = "other"
	other.Artist.Name = "Another artist"
	s.Entries = append(s.Entries, other)
	events := Events(s, cfg(), cfg().EventNamespace(), testNow, true)
	if len(events) != 1 || !strings.Contains(events[0].Summary, "Another artist / Artist") {
		t.Fatal("shared event credits", events)
	}
	b, _ := Render(s, cfg(), testNow)
	if strings.Count(string(b), `<article class="release"`) != 1 || !strings.Contains(string(b), "Another artist / Artist") {
		t.Fatal("report/calendar deduplication differs")
	}
	// A newer source snapshot wins, even if it moves the date outside the window.
	s.Entries[1].Releases = append([]Release(nil), other.Releases...)
	s.Entries[1].Releases[0].Date = "2028-01-01"
	s.Entries[1].CheckedAt = "2026-11-20T00:00:00Z"
	if len(Events(s, cfg(), cfg().EventNamespace(), testNow, true)) != 0 {
		t.Fatal("old duplicate overrode newer metadata")
	}
}

func TestPageRejectsCatalogTruncationAndClearsRecoveredWarning(t *testing.T) {
	for _, r := range []Response{page([]Release{{ID: releaseID}}, 0, 0), page(nil, 50001, 0)} {
		s := ready()
		s.Entries[0].Releases = []Release{{ID: artistID, Title: "Keep"}}
		s.Step(func(string) (Response, error) { return r, nil }, cfg(), testNow)
		if s.Entries[0].Problem == "" || s.Entries[0].Releases[0].Title != "Keep" {
			t.Fatal("inconsistent page replaced cache")
		}
	}
	s := ready()
	s.Entries[0].Problem = "previous invalid response"
	s.Step(func(string) (Response, error) { return page([]Release{{ID: releaseID}}, 2, 0), nil }, cfg(), testNow)
	if s.Entries[0].Problem != "" || s.Entries[0].Phase != "releases" {
		t.Fatal("successful partial page retained an old warning")
	}
}

func TestCalendarUpdateRecoversWhenRemoteEventWasDeleted(t *testing.T) {
	for _, provider := range []string{"caldav", "google"} {
		t.Run(provider, func(t *testing.T) {
			s := published()
			c := cfg()
			ic := icfg()
			ic.CalendarProvider = provider
			ic.CalDAVURL = "https://example.com/calendar/"
			ic.GoogleCalendarID = "calendar@example.com"
			uid := Events(s, c, ic.Namespace, testNow, true)[0].UID
			desired := Events(s, c, ic.Namespace, testNow, true)[0]
			d := DeliveryState{Remote: map[string]*RemoteEvent{uid: {Present: true, Phase: "write", ETag: "old"}}, AccessToken: "fixture", TokenExpiry: testNow.Add(time.Hour).Unix()}
			d.init()
			sendMissing := func(HTTPRequest) (Response, error) { return Response{Status: 404}, nil }
			if provider == "caldav" {
				d.caldavOp(uid, desired, true, d.Remote[uid], ic, sendMissing, testNow)
			} else {
				d.googleOp(uid, desired, true, d.Remote[uid], ic, sendMissing, testNow)
			}
			if d.Remote[uid].Phase != "get" {
				t.Fatal("missing remote event did not trigger re-read")
			}
			// Re-read confirms absence, allowing a protected recreation.
			if provider == "caldav" {
				d.caldavOp(uid, desired, true, d.Remote[uid], ic, sendMissing, testNow)
			} else {
				d.googleOp(uid, desired, true, d.Remote[uid], ic, func(HTTPRequest) (Response, error) { return Response{Status: 410}, nil }, testNow)
			}
			if d.Remote[uid].Present || d.Remote[uid].Phase != "write" {
				t.Fatal("missing event was not prepared for recreation")
			}
			confirm := func(r HTTPRequest) (Response, error) {
				if provider == "caldav" {
					if r.Method != "PUT" || r.Headers["If-None-Match"] != "*" {
						t.Fatal("unprotected CalDAV recreation")
					}
					return Response{Status: 201, Headers: map[string]string{"ETag": "new"}}, nil
				}
				if r.Method != "POST" {
					t.Fatal("Google recreation was not an insert")
				}
				b, _ := json.Marshal(map[string]string{"etag": "new", "id": googleID(uid)})
				return Response{Status: 200, Body: b}, nil
			}
			if provider == "caldav" {
				d.caldavOp(uid, desired, true, d.Remote[uid], ic, confirm, testNow)
			} else {
				d.googleOp(uid, desired, true, d.Remote[uid], ic, confirm, testNow)
			}
			if !d.Remote[uid].Present || d.Remote[uid].Hash != desired.Fingerprint() {
				t.Fatal("recreated event not acknowledged")
			}
		})
	}
}

func TestInvalidPersistedRetryMetadataIsRejected(t *testing.T) {
	for _, change := range []func(*State){
		func(s *State) { s.MusicBrainzFailures = -1 },
		func(s *State) { s.MusicBrainzFailures = 8 },
		func(s *State) { s.NextHTTP = -1 },
		func(s *State) { s.Entries[0].Failures = -1 },
		func(s *State) { s.Entries[0].Failures = 9 },
		func(s *State) { s.Entries[0].RetryAt = -1 },
	} {
		s := ready()
		s.Schema = 1
		change(&s)
		b, _ := json.Marshal(s)
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "state.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatal("invalid retry metadata accepted")
		}
	}
}
