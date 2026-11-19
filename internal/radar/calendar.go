package radar

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type CalendarEvent struct {
	Key         string `json:"key"`
	UID         string `json:"uid"`
	Summary     string `json:"summary"`
	Date        string `json:"date"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (e CalendarEvent) Fingerprint() string {
	return digest(e.Summary + "\x00" + e.Date + "\x00" + e.Description + "\x00" + e.URL)
}
func (r Release) ValidID() bool { return ValidMBID(r.ID) }
func (r Release) SourceURL() string {
	if r.ValidID() {
		return "https://musicbrainz.org/release-group/" + r.ID
	}
	return ""
}
func Events(s State, c Config, namespace string, now time.Time, exactOnly bool) []CalendarEvent {
	result := []CalendarEvent{}
	seen := map[string]bool{}
	for _, entry := range s.Entries {
		for _, r := range entry.Releases {
			if !Visible(r, c, now) || !r.ValidID() || (exactOnly && len(r.Date) != 10) {
				continue
			}
			key := r.ID // Release-group identity, not title/date.
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, CalendarEvent{Key: key, UID: "rr-" + digest(namespace+"\x00"+key) + "@release-radar", Summary: entry.Artist.Name + " — " + r.Title, Date: r.Date, Description: r.Type + " release. Source dates may change. " + DateLabel(r.Date), URL: r.SourceURL()})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UID < result[j].UID })
	return result
}
func icsEscape(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", ",", "\\,", ";", "\\;").Replace(s)
}

// Fold by UTF-8 octets without breaking a code point; continuation starts with a space.
func foldICS(s string) string {
	var out strings.Builder
	limit := 75
	for len(s) > limit {
		n := limit
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		out.WriteString(s[:n])
		out.WriteString("\r\n ")
		s = s[n:]
		limit = 74
	}
	out.WriteString(s)
	out.WriteString("\r\n")
	return out.String()
}
func EventICS(e CalendarEvent, now time.Time) string {
	t, _, valid := DateInterval(e.Date)
	if !valid || len(e.Date) != 10 {
		return ""
	}
	lines := []string{"BEGIN:VEVENT", "UID:" + icsEscape(e.UID), "DTSTAMP:" + now.UTC().Format("20060102T150405Z"), "DTSTART;VALUE=DATE:" + t.Format("20060102"), "DTEND;VALUE=DATE:" + t.AddDate(0, 0, 1).Format("20060102"), "SUMMARY:" + icsEscape(e.Summary), "DESCRIPTION:" + icsEscape(e.Description), "URL:" + icsEscape(e.URL), "TRANSP:TRANSPARENT", "END:VEVENT"}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(foldICS(l))
	}
	return b.String()
}
func CalendarICS(events []CalendarEvent, now time.Time) []byte {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Release Radar//Music releases//EN\r\nCALSCALE:GREGORIAN\r\nX-WR-CALNAME:Music releases\r\n")
	for _, e := range events {
		b.WriteString(EventICS(e, now))
	}
	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String())
}
func ownedICS(body []byte, uid string) bool {
	s := strings.ReplaceAll(strings.ReplaceAll(string(body), "\r\n ", ""), "\r\n\t", "")
	return strings.Contains(s, "PRODID:-//Release Radar//") && strings.Contains(s, "\r\nUID:"+uid+"\r\n")
}
func FullCycle(s State) bool {
	for _, e := range s.Entries {
		if e.Phase == "match" || e.Phase == "releases" {
			return false
		}
	}
	return true
}
func CalendarStats(s State, c Config, namespace string, now time.Time) string {
	all := len(Events(s, c, namespace, now, false))
	exact := len(Events(s, c, namespace, now, true))
	return fmt.Sprintf("%d dated events; %d releases with uncertain dates excluded from calendar export", exact, all-exact)
}
