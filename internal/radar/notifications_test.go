package radar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNtfySwitchAndCategoryChoices(t *testing.T) {
	s := published()
	c := cfg()
	ic := icfg()
	ic.NtfyTopic = "fixture-topic"
	ic.NotifyInitial = true
	d := DeliveryState{}
	calls := 0
	send := func(HTTPRequest) (Response, error) { calls++; return Response{Status: 200}, nil }
	ic.NtfyEnabled = false
	if d.Step(s, c, ic, send, testNow) || calls != 0 {
		t.Fatal("disabled ntfy sent a notification")
	}
	ic.NtfyURL = "invalid"
	if _, problem := ic.ValidParts(); problem != "" {
		t.Fatal("disabled service was validated", problem)
	}
	ic.NtfyEnabled = true
	ic.NtfyURL = "https://ntfy.sh"
	ic.NotifyNew = false
	if !d.Step(s, c, ic, send, testNow) || calls != 0 {
		t.Fatal("disabled new-release category was sent or not acknowledged")
	}
	ic.NotifyNew = true
	d.Step(s, c, ic, send, testNow.Add(3*time.Second))
	if calls != 0 {
		t.Fatal("enabling category replayed skipped backlog")
	}
	s.Entries[0].Releases[0].Date = "2026-12-05"
	ic.NotifyUpdates = false
	d.Step(s, c, ic, send, testNow.Add(6*time.Second))
	if calls != 0 {
		t.Fatal("disabled updates sent")
	}
	ic.NotifyUpdates = true
	d.Step(s, c, ic, send, testNow.Add(9*time.Second))
	if calls != 0 {
		t.Fatal("enabling updates replayed skipped change")
	}
	s.Entries[0].Releases[0].Date = "2026-12-06"
	d.Step(s, c, ic, send, testNow.Add(12*time.Second))
	if calls != 1 {
		t.Fatal("enabled update not sent")
	}
	ic.NtfyEnabled = false
	s.Entries[0].Releases[0].Date = "2026-12-07"
	d.Step(s, c, ic, send, testNow.Add(15*time.Second))
	if calls != 1 {
		t.Fatal("disabled ntfy continued")
	}
	ic.NtfyEnabled = true
	d.Step(s, c, ic, send, testNow.Add(18*time.Second))
	if calls != 2 {
		t.Fatal("re-enabling lost pending update")
	}
}

func TestReleaseDayReminderUsesUTCAndPersistsWithoutDuplicates(t *testing.T) {
	s := published()
	c := cfg()
	ic := icfg()
	ic.NtfyTopic = "fixture-topic"
	ic.NotifyReleaseDay = true
	s.Entries[0].Releases[0].Date = testNow.Format("2006-01-02")
	d := DeliveryState{}
	calls := 0
	send := func(r HTTPRequest) (Response, error) {
		calls++
		if !strings.Contains(string(r.Body), "Music released today") {
			t.Fatal("reminder title missing")
		}
		return Response{Status: 200}, nil
	}
	// The ordinary initial backlog is silent; the independently enabled day reminder follows.
	d.Step(s, c, ic, send, testNow)
	d.Step(s, c, ic, send, testNow.Add(3*time.Second))
	if calls != 1 {
		t.Fatal("release-day reminder missing")
	}
	b, _ := json.Marshal(d)
	var restored DeliveryState
	json.Unmarshal(b, &restored)
	restored.Step(s, c, ic, send, testNow.Add(time.Hour))
	if calls != 1 {
		t.Fatal("reminder duplicated after restart")
	}
	// Moving to tomorrow gives the release a new reminder date, even across local midnight.
	tomorrow := testNow.AddDate(0, 0, 1)
	s.Entries[0].Releases[0].Date = tomorrow.Format("2006-01-02")
	ic.NotifyUpdates = false
	restored.Step(s, c, ic, send, testNow.Add(2*time.Hour))
	if calls != 1 {
		t.Fatal("future release reminded early")
	}
	local := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day()-1, 20, 0, 0, 0, time.FixedZone("fixture", -8*3600))
	restored.Step(s, c, ic, send, local)
	if calls != 2 {
		t.Fatal("UTC reminder day ignored")
	}
	restored.Step(s, c, ic, send, tomorrow.AddDate(0, 0, 1))
	if calls != 2 {
		t.Fatal("late reminder sent")
	}
}

func TestThemesChangePresentationWithoutChangingEventIdentity(t *testing.T) {
	s := published()
	c := cfg()
	original := Events(s, c, c.EventNamespace(), testNow, false)[0]
	for _, theme := range []string{"bright", "dark", "black"} {
		c.Theme = theme
		b, err := Render(s, c, testNow)
		if err != nil || !strings.Contains(string(b), `data-theme="`+theme+`"`) {
			t.Fatal("theme not applied", theme, err)
		}
		got := Events(s, c, c.EventNamespace(), testNow, false)[0]
		if got != original {
			t.Fatal("theme changed calendar data")
		}
	}
	c.Theme = `"><script>alert(1)</script>`
	b, _ := Render(s, c, testNow)
	if !strings.Contains(string(b), `data-theme="dark"`) {
		t.Fatal("unknown theme not constrained")
	}
	legacy := c
	c.CalendarNamespace = "navidrome-upcoming-releases-plugin"
	legacy.CalendarNamespace = "release-radar"
	if c.EventNamespace() != legacy.EventNamespace() {
		t.Fatal("rename changed default calendar namespace")
	}
	old := CalendarICS([]CalendarEvent{original}, testNow)
	old = []byte(strings.ReplaceAll(string(old), "PRODID:-//navidrome-upcoming-releases-plugin//", "PRODID:-//Release Radar//"))
	if !ownedICS(old, original.UID) {
		t.Fatal("legacy CalDAV ownership lost")
	}
}
