package radar

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}
type Fetch func(string) (Response, error)

var mbBase = "https://musicbrainz.org/ws/2/"

// Step makes at most one network request. Cursor and pagination survive reloads.
func (s *State) Step(fetch Fetch, c Config, now time.Time) bool {
	if now.Unix() < s.NextHTTP || len(s.Entries) == 0 {
		return false
	}
	if s.Cursor < 0 || s.Cursor >= len(s.Entries) {
		s.Cursor = 0
	}
	for n := 0; n < len(s.Entries); n++ {
		idx := (s.Cursor + n) % len(s.Entries)
		e := &s.Entries[idx]
		if (e.Phase != "match" && e.Phase != "releases") || e.RetryAt > now.Unix() {
			continue
		}
		s.Cursor = (idx + 1) % len(s.Entries)
		s.NextHTTP = now.Unix() + 2
		endpoint := ""
		if e.Phase == "match" {
			// Escape Lucene syntax inside a quoted name; never concatenate raw queries.
			name := strings.ReplaceAll(strings.ReplaceAll(e.Artist.Name, `\`, `\\`), `"`, `\"`)
			endpoint = mbBase + "artist/?" + url.Values{"query": {`artist:"` + name + `"`}, "fmt": {"json"}, "limit": {"5"}}.Encode()
		} else {
			endpoint = mbBase + "release-group?" + url.Values{"artist": {e.ResolvedMBID}, "fmt": {"json"}, "limit": {"100"}, "offset": {strconv.Itoa(e.Offset)}}.Encode()
		}
		resp, err := fetch(endpoint)
		if err != nil {
			s.pauseMusicBrainz("MusicBrainz could not be reached", idx, now, 0)
			return true
		}
		if resp.Status != 200 {
			retry := RetryDelay(resp.Headers, now)
			message := fmt.Sprintf("MusicBrainz returned HTTP %d", resp.Status)
			if transientStatus(resp.Status) {
				s.pauseMusicBrainz(message, idx, now, retry)
			} else {
				s.MusicBrainzFailures, s.MusicBrainzProblem = 0, ""
				s.fail(e, message, now, retry)
			}
			return true
		}
		s.MusicBrainzFailures, s.MusicBrainzProblem = 0, ""
		if e.Phase == "match" {
			err = e.match(resp.Body)
		} else {
			err = e.page(resp.Body, now)
		}
		if err != nil {
			s.fail(e, err.Error(), now, 0)
		} else {
			e.Failures = 0
			e.RetryAt = 0
			if e.Phase != "unresolved" {
				e.Problem = ""
			}
		}
		return true
	}
	return false
}

// Transient service failures share one persisted backoff across the whole queue.
// Keep the same page pending and retry indefinitely, without marking artists bad.
func transientStatus(status int) bool {
	return status == 408 || status == 429 || (status >= 500 && status <= 599)
}
func (s *State) pauseMusicBrainz(message string, cursor int, now time.Time, delay int64) {
	s.MusicBrainzFailures = min(max(s.MusicBrainzFailures, 0), 6) + 1
	backoff := int64(30) << (s.MusicBrainzFailures - 1)
	s.NextHTTP = now.Unix() + max(backoff, delay)
	s.Cursor = cursor
	s.MusicBrainzProblem = message
}
func (s *State) fail(e *Entry, message string, now time.Time, delay int64) {
	e.Failures = min(max(e.Failures, 0), 8) + 1
	backoff := int64(30) * (1 << min(e.Failures-1, 8))
	if delay > backoff {
		backoff = delay
	}
	e.RetryAt = now.Unix() + backoff
	e.Problem = message
	if e.Failures >= 5 {
		e.Phase = "error"
		e.Problem += "; will retry on the next library refresh"
	}
}
func RetryDelay(headers map[string]string, now time.Time) int64 {
	for k, v := range headers {
		if strings.EqualFold(k, "Retry-After") {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				return min(n, 86400)
			}
			if t, err := time.Parse(time.RFC1123, v); err == nil && t.After(now) {
				return min(int64(t.Sub(now).Seconds())+1, 86400)
			}
		}
	}
	return 0
}
func (e *Entry) match(data []byte) error {
	var result struct {
		Count   *int `json:"count"`
		Artists []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Score int    `json:"score"`
		} `json:"artists"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("invalid artist search response: %w", err)
	}
	if result.Count == nil || *result.Count < len(result.Artists) {
		return fmt.Errorf("invalid artist search pagination")
	}
	matches := []string{}
	for _, a := range result.Artists {
		if a.Score == 100 && normalize(a.Name) == normalize(e.Artist.Name) && ValidMBID(a.ID) {
			matches = append(matches, a.ID)
		}
	}
	// A name alone can never establish identity perfectly. Overrides always win.
	if len(matches) != 1 || *result.Count > len(result.Artists) {
		e.Phase = "unresolved"
		e.Problem = "Artist match is missing or ambiguous; add an artist ID override."
		return nil
	}
	e.ResolvedMBID = matches[0]
	e.Phase = "releases"
	e.Problem = ""
	return nil
}
func (e *Entry) page(data []byte, now time.Time) error {
	var result struct {
		Count  *int      `json:"release-group-count"`
		Offset *int      `json:"release-group-offset"`
		Groups []Release `json:"release-groups"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("invalid release response: %w", err)
	}
	if result.Count == nil || result.Offset == nil || *result.Offset != e.Offset || *result.Count < 0 || *result.Count > 50000 || e.Offset+len(result.Groups) > *result.Count || len(result.Groups) > 100 {
		return fmt.Errorf("release pagination metadata missing or inconsistent")
	}
	if len(result.Groups) == 0 && e.Offset < *result.Count {
		return fmt.Errorf("release pagination ended early")
	}
	if e.Offset+len(result.Groups) > 50000 {
		return fmt.Errorf("release count exceeds safety limit (50,000)")
	}
	for _, r := range result.Groups {
		if !ValidMBID(r.ID) {
			return fmt.Errorf("release response contains an invalid ID")
		}
	}
	e.Pending = append(e.Pending, result.Groups...)
	e.Offset += len(result.Groups)
	if e.Offset >= *result.Count {
		unique := map[string]bool{}
		out := []Release{}
		for _, r := range e.Pending {
			if !unique[r.ID] {
				unique[r.ID] = true
				out = append(out, r)
			}
		}
		e.Releases = out
		e.Pending = nil
		e.Phase = "done"
		e.CheckedAt = now.UTC().Format(time.RFC3339)
		e.Problem = ""
	}
	return nil
}
