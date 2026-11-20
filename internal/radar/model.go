package radar

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const Version = "0.2.2"

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidMBID(s string) bool { return uuidRE.MatchString(s) }

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MBID string `json:"musicBrainzId"`
}
type Release struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Date      string   `json:"first-release-date"`
	Type      string   `json:"primary-type"`
	Secondary []string `json:"secondary-types"`
}
type Entry struct {
	Artist       Artist    `json:"artist"`
	ResolvedMBID string    `json:"resolvedMbid,omitempty"`
	Phase        string    `json:"phase"`
	Offset       int       `json:"offset"`
	Pending      []Release `json:"pending,omitempty"`
	Releases     []Release `json:"releases,omitempty"`
	CheckedAt    string    `json:"checkedAt,omitempty"`
	Problem      string    `json:"problem,omitempty"`
	RetryAt      int64     `json:"retryAt,omitempty"`
	Failures     int       `json:"failures,omitempty"`
}
type State struct {
	IntegrationProblem string  `json:"integrationProblem,omitempty"`
	RefreshToken       string  `json:"refreshToken"`
	RenderedAt         int64   `json:"renderedAt"`
	ViewConfig         string  `json:"viewConfig"`
	Schema             int     `json:"schema"`
	DiscoveredAt       int64   `json:"discoveredAt"`
	Scope              string  `json:"scope"`
	Entries            []Entry `json:"entries"`
	Cursor             int     `json:"cursor"`
	NextHTTP           int64   `json:"nextHttp"`
	Problem            string  `json:"problem,omitempty"`
}
type Config struct {
	CalendarNamespace string
	Username          string
	AheadDays         int
	PastDays          int
	IncludeSingles    bool
	IncludeSecondary  bool
	AutoMatch         bool
	Overrides         map[string]string
	Exclusions        map[string]bool
}

func DefaultConfig() Config {
	return Config{CalendarNamespace: "release-radar", AheadDays: 90, PastDays: 30, AutoMatch: true, Overrides: map[string]string{}, Exclusions: map[string]bool{}}
}
func (c Config) Validate() error {
	if c.Username == "" {
		return fmt.Errorf("set username to a user assigned to this plugin")
	}
	if c.AheadDays < 1 || c.AheadDays > 730 || c.PastDays < 0 || c.PastDays > 365 {
		return fmt.Errorf("invalid date window")
	}
	for id, mbid := range c.Overrides {
		if id == "" || !ValidMBID(mbid) {
			return fmt.Errorf("invalid artist override: use Navidrome artist IDs mapped to MusicBrainz UUIDs")
		}
	}
	return nil
}
func (c Config) Fingerprint() string {
	b, _ := json.Marshal(struct {
		User       string
		Overrides  map[string]string
		Exclusions map[string]bool
		Auto       bool
	}{c.Username, c.Overrides, c.Exclusions, c.AutoMatch})
	return string(b)
}
func (s *State) Reconcile(artists []Artist, c Config, scope string, now time.Time) {
	old := map[string]Entry{}
	// Never retain another user's or another library selection's results.
	if s.Scope == scope {
		for _, e := range s.Entries {
			old[e.Artist.ID] = e
		}
	}
	entries := []Entry{}
	seen := map[string]bool{}
	for _, a := range artists {
		if a.ID == "" || seen[a.ID] || c.Exclusions[a.ID] {
			continue
		}
		seen[a.ID] = true
		mbid := a.MBID
		if override := c.Overrides[a.ID]; override != "" {
			mbid = override
		}
		if !ValidMBID(mbid) {
			mbid = ""
		}
		e, ok := old[a.ID]
		if !ok || a.Name != e.Artist.Name || mbid != e.Artist.MBID {
			e = Entry{}
		}
		// Do not restart unfinished pagination during the daily library refresh.
		if ok && a.Name == e.Artist.Name && mbid == e.Artist.MBID && (e.Phase == "match" || e.Phase == "releases") {
			a.MBID = mbid
			e.Artist = a
			entries = append(entries, e)
			continue
		}
		a.MBID = mbid
		e.Artist = a
		if mbid != "" {
			e.ResolvedMBID = mbid
		}
		if e.ResolvedMBID != "" {
			e.Phase = "releases"
		} else if c.AutoMatch {
			e.Phase = "match"
		} else {
			e.Phase = "unresolved"
			e.Problem = "No MusicBrainz ID; add a manual override."
		}
		e.Offset = 0
		e.Pending = nil
		e.Failures = 0
		e.RetryAt = 0
		entries = append(entries, e)
	}
	s.Schema = 1
	s.Scope = scope
	s.Entries = entries
	s.Cursor = 0
	s.DiscoveredAt = now.Unix()
	s.Problem = ""
}
func ParseArtists(raw string) ([]Artist, error) {
	var envelope struct {
		Response struct {
			Status string `json:"status"`
			Error  struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
			Artists struct {
				Index []struct {
					Artist []Artist `json:"artist"`
				} `json:"index"`
			} `json:"artists"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, err
	}
	if envelope.Response.Status != "ok" {
		// Navidrome uses this exact code/message for an empty, already-verified library.
		if envelope.Response.Error.Code == 70 && envelope.Response.Error.Message == "Library not found or empty" {
			return []Artist{}, nil
		}
		return nil, fmt.Errorf("Navidrome: %s", envelope.Response.Error.Message)
	}
	out := []Artist{}
	for _, idx := range envelope.Response.Artists.Index {
		out = append(out, idx.Artist...)
	}
	return out, nil
}

// DateInterval preserves partial dates, and rejects malformed dates instead of guessing.
func DateInterval(s string) (time.Time, time.Time, bool) {
	var layout string
	switch len(s) {
	case 4:
		layout = "2006"
	case 7:
		layout = "2006-01"
	case 10:
		layout = "2006-01-02"
	default:
		return time.Time{}, time.Time{}, false
	}
	start, err := time.Parse(layout, s)
	if err != nil || start.Year() < 1000 {
		return time.Time{}, time.Time{}, false
	}
	end := start
	switch len(s) {
	case 4:
		end = start.AddDate(1, 0, 0).AddDate(0, 0, -1)
	case 7:
		end = start.AddDate(0, 1, 0).AddDate(0, 0, -1)
	}
	return start, end, true
}
func DateLabel(s string) string {
	t, _, ok := DateInterval(s)
	if !ok {
		return "Date unknown"
	}
	switch len(s) {
	case 4:
		return t.Format("2006") + " (day unknown)"
	case 7:
		return t.Format("January 2006") + " (day unknown)"
	default:
		return t.Format("2 January 2006")
	}
}
func Visible(r Release, c Config, now time.Time) bool {
	if r.Type != "Album" && r.Type != "EP" && !(c.IncludeSingles && r.Type == "Single") {
		return false
	}
	if !c.IncludeSecondary && len(r.Secondary) > 0 {
		return false
	}
	start, end, ok := DateInterval(r.Date)
	if !ok {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return !end.Before(today.AddDate(0, 0, -c.PastDays)) && !start.After(today.AddDate(0, 0, c.AheadDays))
}
func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func (c Config) EventNamespace() string { return c.CalendarNamespace + "\x00" + c.Username }
