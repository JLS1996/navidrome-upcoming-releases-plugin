package radar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func published() State {
	s := ready()
	s.Entries[0].Phase = "done"
	s.Entries[0].Releases = []Release{{ID: releaseID, Title: "New album", Date: "2026-12-04", Type: "Album"}}
	return s
}
func icfg() IntegrationConfig {
	return IntegrationConfig{Namespace: cfg().EventNamespace(), NtfyURL: "https://ntfy.sh", CalendarProvider: "none"}
}
func TestICSIdentityDatesEscapingAndFolding(t *testing.T) {
	s := published()
	s.Entries[0].Releases = append(s.Entries[0].Releases, Release{ID: artistID, Title: "Unknown day", Date: "2026-12", Type: "Album"})
	events := Events(s, cfg(), cfg().EventNamespace(), testNow, true)
	if len(events) != 1 {
		t.Fatal("partial date exported as exact")
	}
	originalUID := events[0].UID
	s.Entries[0].Releases[0].Date = "2026-12-05"
	if Events(s, cfg(), cfg().EventNamespace(), testNow, true)[0].UID != originalUID {
		t.Fatal("date change created a new UID")
	}
	e := events[0]
	e.Summary = strings.Repeat("音楽", 50) + ";x,\r\nBEGIN:VEVENT"
	e.Date = "2026-12-31"
	b := CalendarICS([]CalendarEvent{e}, testNow)
	if !bytes.Contains(b, []byte("DTEND;VALUE=DATE:20270101\r\n")) {
		t.Fatal("all-day end must be exclusive")
	}
	if bytes.Count(b, []byte("\r\nBEGIN:VEVENT\r\n")) != 1 {
		t.Fatal("ICS injection")
	}
	for _, line := range strings.Split(string(b), "\r\n") {
		if len(line) > 75 || !utf8.ValidString(line) {
			t.Fatal("invalid line folding", len(line))
		}
	}
	if bytes.Contains(b, []byte("METHOD:")) {
		t.Fatal("CalDAV calendar objects must not contain METHOD")
	}
	if !ownedICS(b, e.UID) || ownedICS(b, "foreign") {
		t.Fatal("ownership check")
	}
	if EventICS(CalendarEvent{Date: "2026-12"}, testNow) != "" {
		t.Fatal("partial event emitted")
	}
}
func TestNtfyBaselineChangesAndPersistentAcknowledgement(t *testing.T) {
	s := published()
	ic := icfg()
	ic.NtfyTopic = "private-test-topic"
	ic.NtfyToken = "secret-token"
	d := DeliveryState{}
	calls := 0
	send := func(r HTTPRequest) (Response, error) {
		calls++
		if r.Method != "POST" || r.Headers["Authorization"] != "Bearer secret-token" {
			t.Fatal(r)
		}
		var body map[string]any
		if json.Unmarshal(r.Body, &body) != nil || body["topic"] != ic.NtfyTopic {
			t.Fatal(string(r.Body))
		}
		return Response{Status: 200}, nil
	}
	if !d.Step(s, cfg(), ic, send, testNow) || calls != 0 {
		t.Fatal("initial backlog was published")
	}
	s.Entries[0].Releases[0].Date = "2026-12-06"
	d.Step(s, cfg(), ic, send, testNow.Add(3*time.Second))
	if calls != 1 {
		t.Fatal("date update not notified")
	}
	dir := t.TempDir()
	if err := SaveDeliveries(dir, d); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDeliveries(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Step(s, cfg(), ic, send, testNow.Add(10*time.Second))
	if calls != 1 {
		t.Fatal("duplicate after restart")
	}
	html, _ := Render(s, cfg(), testNow)
	if bytes.Contains(html, []byte(ic.NtfyToken)) {
		t.Fatal("token leaked into HTML")
	}
}
func TestNtfyRetryAndNoSecretInStatus(t *testing.T) {
	ic := icfg()
	ic.NtfyTopic = "test"
	ic.NotifyInitial = true
	s := published()
	d := DeliveryState{}
	calls := 0
	send := func(HTTPRequest) (Response, error) {
		calls++
		return Response{Status: 429, Headers: map[string]string{"Retry-After": "300"}, Body: []byte("secret-password")}, nil
	}
	d.Step(s, cfg(), ic, send, testNow)
	d.Step(s, cfg(), ic, send, testNow.Add(30*time.Second))
	if calls != 1 || len(d.NtfySeen) != 0 || strings.Contains(d.Status(), "secret-password") {
		t.Fatal(d)
	}
	d.Step(s, cfg(), ic, func(HTTPRequest) (Response, error) { return Response{}, fmt.Errorf("Bearer secret-password") }, testNow.Add(301*time.Second))
	if strings.Contains(d.Status(), "secret-password") {
		t.Fatal("transport error leaked credentials")
	}
}
func TestCalDAVCreateUpdateDeleteAndPreconditions(t *testing.T) {
	ic := icfg()
	ic.CalendarProvider = "caldav"
	ic.CalDAVURL = "https://example.com/calendar/"
	ic.CalDAVUser = "alice"
	ic.CalDAVPassword = "app-password"
	d := DeliveryState{}
	s := published()
	now := testNow
	ops := []HTTPRequest{}
	send := func(r HTTPRequest) (Response, error) {
		ops = append(ops, r)
		if r.Headers["Authorization"] == "" {
			t.Fatal("no CalDAV authentication")
		}
		switch r.Method {
		case "GET":
			return Response{Status: 404}, nil
		case "PUT":
			return Response{Status: 201, Headers: map[string]string{"ETag": `"v1"`}}, nil
		case "DELETE":
			return Response{Status: 204}, nil
		}
		t.Fatal(r.Method)
		return Response{}, nil
	}
	for i := 0; i < 2; i++ {
		d.Step(s, cfg(), ic, send, now)
		now = now.Add(3 * time.Second)
	}
	if len(ops) != 2 || ops[1].Method != "PUT" || ops[1].Headers["If-None-Match"] != "*" {
		t.Fatal(ops)
	}
	s.Entries[0].Releases[0].Date = "2026-12-07"
	d.Step(s, cfg(), ic, send, now)
	now = now.Add(3 * time.Second)
	if ops[2].Headers["If-Match"] != `"v1"` || !bytes.Contains(ops[2].Body, []byte("DTSTART;VALUE=DATE:20261207")) {
		t.Fatal("date update did not use same protected event")
	}
	if ops[1].URL != ops[2].URL {
		t.Fatal("event URL changed with date")
	}
	s.Entries[0].Releases = nil
	d.Step(s, cfg(), ic, send, now)
	if ops[3].Method != "DELETE" || ops[3].Headers["If-Match"] != `"v1"` || len(d.Remote) != 0 {
		t.Fatal(ops)
	}
}
func TestCalDAVRejectsForeignEventAndRetriesConflict(t *testing.T) {
	ic := icfg()
	ic.CalendarProvider = "caldav"
	ic.CalDAVURL = "https://example.com/calendar/"
	d := DeliveryState{}
	s := published()
	calls := 0
	d.Step(s, cfg(), ic, func(r HTTPRequest) (Response, error) {
		calls++
		return Response{Status: 200, Body: []byte("foreign calendar data"), Headers: map[string]string{"ETag": "foreign"}}, nil
	}, testNow)
	if calls != 1 || !strings.Contains(d.Status(), "ownership conflict") {
		t.Fatal(d)
	}
	d = DeliveryState{}
	step := 0
	send := func(r HTTPRequest) (Response, error) {
		step++
		switch step {
		case 1:
			return Response{Status: 404}, nil
		case 2:
			return Response{Status: 412}, nil
		case 3:
			if r.Method != "GET" {
				t.Fatal("did not re-read after conflict")
			}
			return Response{Status: 200, Body: CalendarICS(Events(s, cfg(), ic.Namespace, testNow, true), testNow), Headers: map[string]string{"etag": `"fresh"`}}, nil
		default:
			return Response{Status: 204, Headers: map[string]string{"etag": `"new"`}}, nil
		}
	}
	for i := 0; i < 4; i++ {
		d.Step(s, cfg(), ic, send, testNow.Add(time.Duration(i*3)*time.Second))
	}
	for _, r := range d.Remote {
		if r.Hash == "" || r.ETag != `"new"` {
			t.Fatal(r)
		}
	}
}
func TestGoogleRefreshCreatePatchAndOwnership(t *testing.T) {
	ic := icfg()
	ic.CalendarProvider = "google"
	ic.GoogleCalendarID = "calendar@example.com"
	ic.GoogleClientID = "client"
	ic.GoogleClientSecret = "secret"
	ic.GoogleRefreshToken = "refresh"
	d := DeliveryState{}
	s := published()
	ops := []HTTPRequest{}
	send := func(r HTTPRequest) (Response, error) {
		ops = append(ops, r)
		switch len(ops) {
		case 1:
			if !bytes.Contains(r.Body, []byte("grant_type=refresh_token")) {
				t.Fatal("no refresh flow")
			}
			return Response{Status: 200, Body: []byte(`{"access_token":"access","expires_in":3600}`)}, nil
		case 2:
			return Response{Status: 404}, nil
		case 3:
			if r.Method != "POST" || r.Headers["Authorization"] != "Bearer access" {
				t.Fatal(r)
			}
			var created map[string]any
			json.Unmarshal(r.Body, &created)
			body, _ := json.Marshal(map[string]any{"etag": "v1", "id": created["id"]})
			return Response{Status: 200, Body: body}, nil
		default:
			id := r.URL[strings.LastIndex(r.URL, "/")+1:]
			body, _ := json.Marshal(map[string]string{"etag": "v2", "id": id})
			return Response{Status: 200, Body: body}, nil
		}
	}
	for i := 0; i < 3; i++ {
		d.Step(s, cfg(), ic, send, testNow.Add(time.Duration(3*i)*time.Second))
	}
	s.Entries[0].Releases[0].Date = "2026-12-08"
	d.Step(s, cfg(), ic, send, testNow.Add(10*time.Second))
	if len(ops) != 4 || ops[3].Method != "PATCH" || ops[3].Headers["If-Match"] != "v1" {
		t.Fatal("Google update did not patch existing protected event")
	}
	var created, updated map[string]any
	json.Unmarshal(ops[2].Body, &created)
	json.Unmarshal(ops[3].Body, &updated)
	if updated["start"].(map[string]any)["date"] != "2026-12-08" || updated["end"].(map[string]any)["date"] != "2026-12-09" {
		t.Fatal(updated)
	}
	if !strings.HasSuffix(ops[3].URL, "/"+created["id"].(string)) {
		t.Fatal("Google identity changed")
	}
	d = DeliveryState{AccessToken: "access", TokenExpiry: testNow.Add(time.Hour).Unix(), GoogleAuth: d.GoogleAuth}
	// First step resets calendar target but must retain auth; GET foreign event must never PATCH.
	d.Step(s, cfg(), ic, func(r HTTPRequest) (Response, error) {
		if r.Method != "GET" {
			t.Fatal(r)
		}
		return Response{Status: 200, Body: []byte(`{"etag":"x","extendedProperties":{"private":{"releaseRadarUID":"foreign"}}}`)}, nil
	}, testNow)
	if !strings.Contains(d.Status(), "ownership conflict") {
		t.Fatal(d)
	}
}
func TestCalendarDeletionWaitsForCompleteDiscovery(t *testing.T) {
	ic := icfg()
	ic.CalendarProvider = "caldav"
	ic.CalDAVURL = "https://example.com/calendar"
	s := published()
	d := DeliveryState{}
	send := func(r HTTPRequest) (Response, error) {
		if r.Method == "GET" {
			return Response{Status: 404}, nil
		}
		return Response{Status: 201, Headers: map[string]string{"etag": "x"}}, nil
	}
	d.Step(s, cfg(), ic, send, testNow)
	d.Step(s, cfg(), ic, send, testNow.Add(3*time.Second))
	s.Entries[0].Phase = "releases"
	s.Entries[0].Releases = nil
	calls := 0
	d.Step(s, cfg(), ic, func(HTTPRequest) (Response, error) { calls++; return Response{}, nil }, testNow.Add(6*time.Second))
	if calls != 0 {
		t.Fatal("deleted events while source discovery incomplete")
	}
}
func TestIntegrationConfigurationAndFiles(t *testing.T) {
	ic := icfg()
	ic.NtfyTopic = "test"
	if err := ic.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"http://ntfy.sh", "https://user:pass@example.com", "https://ntfy.sh/?token=secret"} {
		ic.NtfyURL = bad
		if ic.Validate() == nil {
			t.Fatal(bad)
		}
	}
	dir := t.TempDir()
	s := published()
	if err := Save(dir, s, cfg(), testNow); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "music-releases.ics"))
	if err != nil || !bytes.Contains(b, []byte("BEGIN:VEVENT")) {
		t.Fatal(err)
	}
	if _, err := LoadDeliveries(dir); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "integrations.json"), []byte("broken"), 0600)
	if _, err := LoadDeliveries(dir); err == nil {
		t.Fatal("corrupt integration state ignored")
	}
}
func TestInvalidCalendarConfigurationDoesNotBlockNtfy(t *testing.T) {
	ic := icfg()
	ic.NtfyTopic = "test"
	ic.NotifyInitial = true
	ic.CalendarProvider = "google"
	active, problem := ic.ValidParts()
	if active.CalendarProvider != "none" || active.NtfyTopic != "test" || !strings.Contains(problem, "calendar:") {
		t.Fatal(active, problem)
	}
	d := DeliveryState{}
	calls := 0
	d.Step(published(), cfg(), active, func(HTTPRequest) (Response, error) { calls++; return Response{Status: 200}, nil }, testNow)
	if calls != 1 {
		t.Fatal("valid notifications were blocked by calendar configuration")
	}
}
func TestUnconfirmedGoogleWriteIsNotAcknowledged(t *testing.T) {
	ic := icfg()
	ic.CalendarProvider = "google"
	ic.GoogleCalendarID = "calendar@example.com"
	ic.GoogleClientID = "client"
	ic.GoogleClientSecret = "secret"
	ic.GoogleRefreshToken = "refresh"
	s := published()
	d := DeliveryState{}
	calls := 0
	send := func(r HTTPRequest) (Response, error) {
		calls++
		switch calls {
		case 1:
			return Response{Status: 200, Body: []byte(`{"access_token":"access","expires_in":3600}`)}, nil
		case 2:
			return Response{Status: 404}, nil
		default:
			return Response{Status: 200, Body: []byte(`<html>sign in</html>`)}, nil
		}
	}
	for i := 0; i < 3; i++ {
		d.Step(s, cfg(), ic, send, testNow.Add(time.Duration(i*3)*time.Second))
	}
	for _, r := range d.Remote {
		if r.Hash != "" || r.Phase != "get" {
			t.Fatal("malformed success was acknowledged", r)
		}
	}
	if !strings.Contains(d.Status(), "not confirmed") {
		t.Fatal(d.Status())
	}
}
func TestEquivalentNtfyDestinationKeepsAcknowledgements(t *testing.T) {
	ic := icfg()
	ic.NtfyTopic = "topic"
	d := DeliveryState{}
	s := published()
	d.Step(s, cfg(), ic, func(HTTPRequest) (Response, error) {
		t.Fatal("initial backlog should be suppressed")
		return Response{}, nil
	}, testNow)
	ic.NtfyURL += "/"
	ic.NotifyInitial = true
	d.Step(s, cfg(), ic, func(HTTPRequest) (Response, error) {
		t.Fatal("trailing slash caused duplicate notification")
		return Response{}, nil
	}, testNow.Add(time.Minute))
}
