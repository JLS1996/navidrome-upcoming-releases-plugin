package radar

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const artistID = "5182c1d9-c7d2-4dad-afa0-ccfeada921a8"
const releaseID = "11111111-2222-3333-4444-555555555555"

var testNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func cfg() Config { c := DefaultConfig(); c.Username = "alice"; return c }
func ready() State {
	s := State{}
	s.Reconcile([]Artist{{ID: "ar-1", Name: "Artist", MBID: artistID}}, cfg(), "scope", testNow)
	return s
}
func page(groups []Release, count, offset int) Response {
	b, _ := json.Marshal(map[string]any{"release-group-count": count, "release-group-offset": offset, "release-groups": groups})
	return Response{Status: 200, Body: b}
}
func TestPartialDatesAndWindows(t *testing.T) {
	for _, tc := range []struct {
		date           string
		valid, visible bool
	}{{"2026-12", true, true}, {"2026", true, true}, {"2026-11-19", true, true}, {"2026-02-30", false, false}, {"2026-13", false, false}, {"", false, false}, {"2027-12", true, false}, {"2025-01-01", true, false}} {
		_, _, ok := DateInterval(tc.date)
		if ok != tc.valid {
			t.Errorf("valid %q", tc.date)
		}
		if got := Visible(Release{Date: tc.date, Type: "Album"}, cfg(), testNow); got != tc.visible {
			t.Errorf("visible %q=%v", tc.date, got)
		}
	}
	if !strings.Contains(DateLabel("2026-12"), "day unknown") {
		t.Fatal("partial date precision lost")
	}
}
func TestPaginationCommitAndDateChanges(t *testing.T) {
	s := ready()
	s.Entries[0].Releases = []Release{{ID: releaseID, Title: "Old", Date: "2026-11-20", Type: "Album"}}
	calls := 0
	fetch := func(u string) (Response, error) {
		calls++
		if calls == 1 {
			return page([]Release{{ID: releaseID, Title: "New", Date: "2026-12-01", Type: "Album"}}, 2, 0), nil
		}
		if !strings.Contains(u, "offset=1") {
			t.Fatal(u)
		}
		return page([]Release{{ID: releaseID, Title: "New", Date: "2026-12-01", Type: "Album"}}, 2, 1), nil
	}
	s.Step(fetch, cfg(), testNow)
	if s.Entries[0].Releases[0].Title != "Old" {
		t.Fatal("partial results replaced valid snapshot")
	}
	// Simulate a reload between pages.
	b, _ := json.Marshal(s)
	json.Unmarshal(b, &s)
	s.Step(fetch, cfg(), testNow.Add(3*time.Second))
	e := s.Entries[0]
	if e.Phase != "done" || len(e.Releases) != 1 || e.Releases[0].Date != "2026-12-01" {
		t.Fatalf("%+v", e)
	}
}
func TestFailurePreservesResultsAndHonorsGlobalRateLimit(t *testing.T) {
	s := ready()
	s.Entries[0].Releases = []Release{{ID: releaseID, Title: "Keep"}}
	s.Entries = append(s.Entries, s.Entries[0])
	s.Entries[1].Artist.ID = "other"
	calls := 0
	fetch := func(string) (Response, error) {
		calls++
		return Response{Status: 429, Headers: map[string]string{"retry-after": "120"}}, nil
	}
	s.Step(fetch, cfg(), testNow)
	s.Step(fetch, cfg(), testNow.Add(60*time.Second))
	if calls != 1 || s.NextHTTP != testNow.Unix()+120 || s.Entries[0].Releases[0].Title != "Keep" {
		t.Fatalf("calls=%d state=%+v", calls, s)
	}
	if RetryDelay(map[string]string{"Retry-After": testNow.Add(time.Minute).Format(time.RFC1123)}, testNow) < 60 {
		t.Fatal("HTTP-date ignored")
	}
}
func TestMalformedPagesAndTransportErrors(t *testing.T) {
	for _, response := range []Response{{Status: 200, Body: []byte(`{}`)}, page(nil, 2, 0), page([]Release{{ID: "bad"}}, 1, 0), {Status: 200, Body: []byte(`<html>bad gateway</html>`)}} {
		s := ready()
		s.Entries[0].Releases = []Release{{ID: releaseID, Title: "Keep"}}
		s.Step(func(string) (Response, error) { return response, nil }, cfg(), testNow)
		if s.Entries[0].Problem == "" || s.Entries[0].Releases[0].Title != "Keep" {
			t.Fatal(s)
		}
	}
	s := ready()
	s.Step(func(string) (Response, error) { return Response{}, fmt.Errorf("offline") }, cfg(), testNow)
	if s.MusicBrainzFailures != 1 || s.Entries[0].Failures != 0 {
		t.Fatal(s)
	}
}
func TestAmbiguousMatchAndOverrides(t *testing.T) {
	e := Entry{Artist: Artist{Name: "Same Name"}, Phase: "match"}
	body := fmt.Sprintf(`{"count":2,"artists":[{"id":%q,"name":"Same Name","score":100},{"id":%q,"name":"Same Name","score":100}]}`, artistID, releaseID)
	if err := e.match([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if e.Phase != "unresolved" {
		t.Fatal(e)
	}
	c := cfg()
	c.Overrides["one"] = artistID
	s := State{}
	s.Reconcile([]Artist{{ID: "one", Name: "Same Name"}}, c, "scope", testNow)
	if s.Entries[0].ResolvedMBID != artistID || s.Entries[0].Phase != "releases" {
		t.Fatal(s)
	}
}
func TestScopeAndRemoval(t *testing.T) {
	s := ready()
	s.Entries[0].Releases = []Release{{ID: releaseID}}
	s.Reconcile([]Artist{{ID: "ar-1", Name: "Artist", MBID: artistID}}, cfg(), "different-user-or-library", testNow)
	if len(s.Entries[0].Releases) > 0 {
		t.Fatal("cross-scope data retained")
	}
	s.Reconcile(nil, cfg(), s.Scope, testNow)
	if len(s.Entries) != 0 {
		t.Fatal("removed artist retained")
	}
}
func TestHTMLInjectionAndOfflineOutput(t *testing.T) {
	s := ready()
	s.Entries[0].Artist.Name = `<script>window.pwned=1</script>`
	s.Entries[0].Releases = []Release{{ID: releaseID, Title: `</h2><img src=x onerror="window.pwned=1">`, Date: "2026-12", Type: "Album"}}
	b, err := Render(s, cfg(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	if strings.Contains(html, `<img src=x`) || strings.Contains(html, `<script>window.pwned`) || !strings.Contains(html, "&lt;img") {
		t.Fatal("unsafe HTML output")
	}
	if strings.Contains(html, "fetch(") || strings.Contains(html, "<script src=") {
		t.Fatal("dashboard requires network")
	}
}
func TestAtomicOutputAndCorruptState(t *testing.T) {
	dir := t.TempDir()
	s := ready()
	if err := Save(dir, s, cfg(), testNow); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil || len(loaded.Entries) != 1 {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "upcoming-releases.html"))
	if len(b) < 100 {
		t.Fatal("no dashboard")
	}
	if err := AtomicWrite(dir, "../escape", nil); err == nil {
		t.Fatal("path traversal allowed")
	}
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("broken"), 0600)
	if _, err := Load(dir); err == nil {
		t.Fatal("corruption silently ignored")
	}
}
func TestSubsonicParsingAndFilters(t *testing.T) {
	a, err := ParseArtists(`{"subsonic-response":{"status":"ok","artists":{"index":[{"artist":[{"id":"a","name":"Music","musicBrainzId":"x"}]}]}}}`)
	if err != nil || len(a) != 1 || a[0].MBID != "x" {
		t.Fatal(a, err)
	}
	if _, err = ParseArtists(`{"subsonic-response":{"status":"failed","error":{"message":"permission denied"}}}`); err == nil {
		t.Fatal("ignored permission error")
	}
	c := cfg()
	for _, r := range []Release{{Type: "Single", Date: "2026-12-01"}, {Type: "Album", Date: "2026-12-01", Secondary: []string{"Live"}}} {
		if Visible(r, c, testNow) {
			t.Fatal("default filters ignored")
		}
	}
}
func TestDailyDiscoveryKeepsUnfinishedPagination(t *testing.T) {
	s := ready()
	s.Entries[0].Offset = 100
	s.Entries[0].Pending = []Release{{ID: releaseID}}
	s.Reconcile([]Artist{s.Entries[0].Artist}, cfg(), s.Scope, testNow.Add(24*time.Hour))
	if s.Entries[0].Offset != 100 || len(s.Entries[0].Pending) != 1 {
		t.Fatal("daily refresh discarded progress")
	}
}
func TestMissingStateCanBeSavedBeforeConfiguration(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = Save(dir, s, DefaultConfig(), testNow); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(dir); err != nil {
		t.Fatal(err)
	}
}
func TestStorageWriteNeverFollowsTemporarySymlinks(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "precious.flac")
	os.WriteFile(music, []byte("keep audio untouched"), 0600)
	tmp := filepath.Join(dir, "upcoming-releases.html.tmp")
	if err := os.Symlink(music, tmp); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(dir, "upcoming-releases.html", []byte("<html>report</html>")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(music)
	if string(b) != "keep audio untouched" {
		t.Fatal("export followed a symlink into audio")
	}
	info, _ := os.Stat(filepath.Join(dir, "upcoming-releases.html"))
	if info.Mode().Perm()&0022 != 0 {
		t.Fatalf("export mode %o", info.Mode().Perm())
	}
}
func TestCurrentLibraryPermissionsAreIntersected(t *testing.T) {
	raw := `{"subsonic-response":{"status":"ok","musicFolders":{"musicFolder":[{"id":2}]}}}`
	ids, err := AccessibleLibraries(raw, []int{1, 2, 2, 3})
	if err != nil || len(ids) != 1 || ids[0] != 2 {
		t.Fatal(ids, err)
	}
	ids, err = AccessibleLibraries(raw, []int{1})
	if err != nil || len(ids) != 0 {
		t.Fatal("revoked library remained accessible")
	}
	for _, body := range []string{`null`, `{}`, `{"subsonic-response":{"status":"failed"}}`} {
		if _, err = AccessibleLibraries(body, []int{1}); err == nil {
			t.Fatal("permission verification failed open")
		}
	}
}
func TestCorruptStateCannotSilentlyResetAcknowledgements(t *testing.T) {
	dir := t.TempDir()
	for _, body := range []string{`null`, `{}`, `{"schema":0}`, `{"schema":1,"entries":[{"phase":"unknown"}]}`} {
		os.WriteFile(filepath.Join(dir, "state.json"), []byte(body), 0600)
		if _, err := Load(dir); err == nil {
			t.Fatalf("accepted invalid state %s", body)
		}
	}
	for _, body := range []string{`null`, `{}`, `{"schema":1,"remote":{"bad":null}}`, `{"schema":1,"remote":{"bad":{"phase":"unknown"}}}`} {
		os.WriteFile(filepath.Join(dir, "integrations.json"), []byte(body), 0600)
		if _, err := LoadDeliveries(dir); err == nil {
			t.Fatalf("accepted invalid delivery state %s", body)
		}
	}
}
func TestInvalidCursorDoesNotPanic(t *testing.T) {
	s := ready()
	s.Cursor = -999
	if !s.Step(func(string) (Response, error) { return page(nil, 0, 0), nil }, cfg(), testNow) || s.Entries[0].Phase != "done" {
		t.Fatal(s)
	}
}
func TestVerifiedEmptyLibraryIsNotAnAuthorizationFailure(t *testing.T) {
	raw := `{"subsonic-response":{"status":"failed","error":{"code":70,"message":"Library not found or empty"}}}`
	artists, err := ParseArtists(raw)
	if err != nil || len(artists) != 0 {
		t.Fatal(artists, err)
	}
	if _, err = ParseArtists(`{"subsonic-response":{"status":"failed","error":{"code":70,"message":"Library 2 not found or not accessible"}}}`); err == nil {
		t.Fatal("permission error became an empty success")
	}
}
func TestUsernameComesOnlyFromGrantedAccess(t *testing.T) {
	for _, tc := range []struct {
		name    string
		granted []string
		want    string
		fail    bool
	}{
		{"single", []string{"reader"}, "reader", false},
		{"duplicates", []string{"reader", "reader"}, "reader", false},
		{"empty grant ignored", []string{"", "reader"}, "reader", false},
		{"no grants", nil, "", true},
		{"ambiguous", []string{"admin", "reader"}, "", true},
		{"current grant", []string{"admin"}, "admin", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveUsername(tc.granted)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("selection %q, error %v", got, err)
			}
		})
	}
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatal("configuration must validate before grants are resolved", err)
	}
}

func TestTransientOutageRetriesGloballyAndRecovers(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			s := ready()
			s.Entries[0].Offset = 1
			s.Entries[0].Pending = []Release{{ID: releaseID, Title: "Pending"}}
			s.Entries[0].Releases = []Release{{ID: releaseID, Title: "Keep"}}
			other := s.Entries[0]
			other.Artist.ID = "other"
			s.Entries = append(s.Entries, other)
			at := testNow
			for attempt := 1; attempt <= 9; attempt++ {
				calls := 0
				fetch := func(string) (Response, error) { calls++; return Response{Status: status}, nil }
				if !s.Step(fetch, cfg(), at) || calls != 1 {
					t.Fatal("expected one request")
				}
				wait := int64(30) << min(attempt-1, 6)
				if s.NextHTTP != at.Unix()+wait || s.Cursor != 0 || s.Entries[0].Phase != "releases" || s.Entries[0].Failures != 0 || s.Entries[1].Problem != "" {
					t.Fatalf("backoff or queue lost: %+v", s)
				}
				if s.Step(fetch, cfg(), at.Add(time.Duration(wait-1)*time.Second)) || calls != 1 {
					t.Fatal("requested during global cooldown")
				}
				// A restart must preserve the queue-wide failure streak and its deadline.
				b, _ := json.Marshal(s)
				if err := json.Unmarshal(b, &s); err != nil {
					t.Fatal(err)
				}
				at = time.Unix(s.NextHTTP, 0)
			}
			b, _ := Render(s, cfg(), at)
			if !strings.Contains(string(b), "automatic retry after") || strings.Contains(string(b), "artists need attention") {
				t.Fatal("service outage presented as artist problem")
			}
			s.Step(func(u string) (Response, error) {
				if !strings.Contains(u, "offset=1") {
					t.Fatal("pagination restarted")
				}
				return page([]Release{{ID: artistID, Title: "Recovered"}}, 2, 1), nil
			}, cfg(), at)
			if s.MusicBrainzFailures != 0 || s.MusicBrainzProblem != "" || s.Entries[0].Phase != "done" || len(s.Entries[0].Releases) != 2 {
				t.Fatal("outage did not recover")
			}
		})
	}
}
func TestOldTransientErrorsResumeWithoutDiscardingSnapshots(t *testing.T) {
	s := ready()
	s.Schema = 1
	s.Entries[0].Phase = "error"
	s.Entries[0].Failures = 5
	s.Entries[0].RetryAt = testNow.Unix() + 300
	s.Entries[0].Problem = "MusicBrainz returned HTTP 503; will retry on the next library refresh"
	s.Entries[0].Offset = 100
	s.Entries[0].Releases = []Release{{ID: releaseID, Title: "Keep"}}
	b, _ := json.Marshal(s)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	e := restored.Entries[0]
	if e.Phase != "releases" || e.Failures != 0 || e.RetryAt != 0 || e.Problem != "" || e.Offset != 100 || len(e.Releases) != 1 {
		t.Fatal("old transient failure did not migrate")
	}
}

func TestEmptyReportHasValidDownloadAndTextFallback(t *testing.T) {
	b, err := Render(State{Schema: 1}, cfg(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	document := string(b)
	const prefix = "data:text/calendar;charset=utf-8;base64,"
	_, payload, ok := strings.Cut(document, prefix)
	if !ok {
		t.Fatal("missing embedded calendar")
	}
	encoded, _, _ := strings.Cut(payload, `"`)
	calendar, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(calendar), "BEGIN:VCALENDAR\r\n") || !strings.HasSuffix(string(calendar), "END:VCALENDAR\r\n") || strings.Contains(string(calendar), "BEGIN:VEVENT") {
		t.Fatal("invalid empty calendar")
	}
	if !strings.Contains(document, "Importing this calendar will add nothing.") || !strings.Contains(document, html.EscapeString(string(calendar))) || !strings.Contains(document, "Show calendar text") {
		t.Fatal("empty calendar has no text fallback")
	}
}
