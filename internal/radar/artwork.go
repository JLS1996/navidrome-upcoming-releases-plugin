package radar

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxArtworkBytes = 256 << 10
const maxArtworkCache = 4 << 20
const maxArtworkRecords = 100

var artworkBase = "https://coverartarchive.org/release-group/"
var artworkTransportBase = "" // Build-tagged local integration fixture only.

type ArtworkRecord struct {
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
	CheckedAt int64  `json:"checkedAt,omitempty"`
	RetryAt   int64  `json:"retryAt,omitempty"`
	Failures  int    `json:"failures,omitempty"`
	Redirects int    `json:"redirects,omitempty"`
}
type ArtworkState struct {
	Schema   int                       `json:"schema"`
	Scope    string                    `json:"scope"`
	Records  map[string]*ArtworkRecord `json:"records"`
	NextHTTP int64                     `json:"nextHttp"`
	Problem  string                    `json:"problem,omitempty"`
}

func artworkURL(raw string) bool {
	if len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "coverartarchive.org" || h == "archive.org" || strings.HasSuffix(h, ".archive.org")
}
func artworkData(body []byte) (string, error) {
	if len(body) == 0 || len(body) > maxArtworkBytes {
		return "", fmt.Errorf("thumbnail size exceeds limit")
	}
	reader := bytes.NewReader(body)
	mime := ""
	width, height := 0, 0
	if len(body) >= 3 && bytes.Equal(body[:3], []byte{0xff, 0xd8, 0xff}) {
		c, err := jpeg.DecodeConfig(reader)
		if err != nil {
			return "", err
		}
		width, height = c.Width, c.Height
		mime = "image/jpeg"
	} else if len(body) >= 8 && bytes.Equal(body[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		c, err := png.DecodeConfig(reader)
		if err != nil {
			return "", err
		}
		width, height = c.Width, c.Height
		mime = "image/png"
	} else {
		return "", fmt.Errorf("thumbnail is not JPEG or PNG")
	}
	if width <= 0 || height <= 0 || width > 512 || height > 512 {
		return "", fmt.Errorf("thumbnail dimensions exceed limit")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}
func safeArtworkData(data string) bool {
	encoded, ok := strings.CutPrefix(data, "data:image/jpeg;base64,")
	if !ok {
		encoded, ok = strings.CutPrefix(data, "data:image/png;base64,")
	}
	if !ok || len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(maxArtworkBytes) {
		return false
	}
	for _, c := range encoded {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}
func LoadArtwork(dir string) (ArtworkState, error) {
	s := ArtworkState{Schema: 1, Records: map[string]*ArtworkRecord{}}
	b, err := os.ReadFile(filepath.Join(dir, "covers.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if len(b) > maxArtworkCache+512<<10 {
		return s, fmt.Errorf("cover cache exceeds limit")
	}
	s = ArtworkState{}
	if json.Unmarshal(b, &s) != nil || s.Schema != 1 || s.NextHTTP < 0 || len(s.Records) > maxArtworkRecords {
		return s, fmt.Errorf("cover cache is corrupt or unsupported; preserve covers.json before resetting")
	}
	total := 0
	for id, r := range s.Records {
		if !ValidMBID(id) || r == nil || r.Failures < 0 || r.Failures > 7 || r.RetryAt < 0 || r.Redirects < 0 || r.Redirects > 5 || (r.URL != "" && !artworkURL(r.URL)) || (r.Data != "" && !safeArtworkData(r.Data)) {
			return s, fmt.Errorf("cover cache contains invalid artwork metadata")
		}
		total += len(r.Data)
	}
	if total > maxArtworkCache {
		return s, fmt.Errorf("cover cache exceeds limit")
	}
	if s.Records == nil {
		s.Records = map[string]*ArtworkRecord{}
	}
	return s, nil
}
func SaveArtwork(dir string, s ArtworkState) error {
	s.Schema = 1
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return AtomicWrite(dir, "covers.json", b)
}
func (s *ArtworkState) Reconcile(state State, c Config, now time.Time) bool {
	changed := false
	if s.Scope != state.Scope {
		*s = ArtworkState{Schema: 1, Scope: state.Scope, Records: map[string]*ArtworkRecord{}}
		changed = true
	}
	if s.Records == nil {
		s.Records = map[string]*ArtworkRecord{}
	}
	releases := visibleReleases(state, c, now)
	sort.Slice(releases, func(i, j int) bool {
		if releases[i].Date == releases[j].Date {
			return releases[i].ID < releases[j].ID
		}
		return releases[i].Date < releases[j].Date
	})
	wanted := map[string]bool{}
	for _, r := range releases {
		if len(wanted) >= maxArtworkRecords {
			break
		}
		wanted[r.ID] = true
	}
	for id := range s.Records {
		if !wanted[id] {
			delete(s.Records, id)
			changed = true
		}
	}
	for id := range wanted {
		if s.Records[id] == nil {
			s.Records[id] = &ArtworkRecord{}
			changed = true
		}
	}
	return changed
}
func (s ArtworkState) Images() map[string]string {
	out := map[string]string{}
	for id, r := range s.Records {
		if r.Data != "" {
			out[id] = r.Data
		}
	}
	return out
}
func (s *ArtworkState) Step(send Send, now time.Time) bool {
	if now.Unix() < s.NextHTTP {
		return false
	}
	ids := []string{}
	for id := range s.Records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.Records[id]
		if r.RetryAt > now.Unix() || r.Data != "" || r.CheckedAt > now.Unix()-86400 {
			continue
		}
		endpoint := r.URL
		if endpoint == "" {
			endpoint = artworkBase + id + "/front-250"
		}
		if !artworkURL(endpoint) {
			s.fail(r, "Invalid cover destination", now, 86400)
			return true
		}
		req := HTTPRequest{Method: "GET", URL: endpoint, Headers: map[string]string{"User-Agent": MusicBrainzUserAgent(), "Accept": "image/jpeg,image/png"}}
		if artworkTransportBase != "" {
			u, _ := url.Parse(endpoint)
			req.URL = artworkTransportBase + u.RequestURI()
		}
		resp, err := send(req)
		s.NextHTTP = now.Unix() + 2
		if err != nil {
			s.fail(r, "Cover art request failed", now, 0)
			s.NextHTTP = r.RetryAt
			return true
		}
		if resp.Status == 301 || resp.Status == 302 || resp.Status == 303 || resp.Status == 307 || resp.Status == 308 {
			base, _ := url.Parse(endpoint)
			location, e := url.Parse(header(resp.Headers, "Location"))
			next := ""
			if e == nil {
				next = base.ResolveReference(location).String()
			}
			if header(resp.Headers, "Location") == "" || next == endpoint || !artworkURL(next) || r.Redirects >= 5 {
				s.fail(r, "Cover art redirect was refused", now, 86400)
				return true
			}
			r.URL = next
			r.Redirects++
			return true
		}
		if resp.Status == 404 {
			r.CheckedAt = now.Unix()
			r.URL = ""
			r.Redirects = 0
			r.Failures = 0
			r.RetryAt = 0
			s.Problem = ""
			return true
		}
		if resp.Status != 200 {
			s.fail(r, fmt.Sprintf("Cover art returned HTTP %d", resp.Status), now, RetryDelay(resp.Headers, now))
			if transientStatus(resp.Status) {
				s.NextHTTP = r.RetryAt
			}
			return true
		}
		data, e := artworkData(resp.Body)
		if e != nil {
			s.fail(r, "Cover art thumbnail was invalid or too large", now, 86400)
			return true
		}
		total := len(data)
		for other, record := range s.Records {
			if other != id {
				total += len(record.Data)
			}
		}
		if total > maxArtworkCache {
			r.CheckedAt = now.Unix()
			r.URL = ""
			r.Redirects = 0
			s.Problem = "Cover cache is full; some releases will have no image"
			return true
		}
		r.Data = data
		r.URL = ""
		r.CheckedAt = now.Unix()
		r.Redirects = 0
		r.Failures = 0
		r.RetryAt = 0
		s.Problem = ""
		return true
	}
	return false
}
func (s *ArtworkState) fail(r *ArtworkRecord, message string, now time.Time, delay int64) {
	r.Failures = min(max(r.Failures, 0), 6) + 1
	r.URL = ""
	r.Redirects = 0
	r.RetryAt = now.Unix() + max(int64(30)<<(r.Failures-1), delay)
	s.NextHTTP = now.Unix() + 2
	s.Problem = message + "; retries are automatic and release checks continue"
}
