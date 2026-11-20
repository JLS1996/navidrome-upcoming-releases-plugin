package radar

import (
	"encoding/json"
	"fmt"
	"time"
)

// CreditInventory stores only whole credit names and IDs, never song titles or
// paths. A new snapshot replaces the old one only after both scoped scans finish.
type CreditInventory struct {
	Ready   bool            `json:"ready,omitempty"`
	At      int64           `json:"at,omitempty"`
	RetryAt int64           `json:"retryAt,omitempty"`
	Phase   string          `json:"phase,omitempty"`
	Offset  int             `json:"offset,omitempty"`
	Albums  int             `json:"albums,omitempty"`
	Tracks  int             `json:"tracks,omitempty"`
	Credits map[string]bool `json:"credits,omitempty"`
	Pending map[string]bool `json:"pending,omitempty"`
}

type directCredit struct {
	ID       string `json:"id"`
	Artist   string `json:"artist"`
	ArtistID string `json:"artistId"`
}

func creditKey(name, id string) string { return normalize(name) + "\x00" + id }

// IncludedArtist requires a whole-name match even when Navidrome's legacy
// artistId points to a participant in a combined album or track credit.
func IncludedArtist(a Artist, c Config) bool {
	if !c.ExcludeUncredited {
		return true
	}
	return c.DirectReady && (c.DirectCredits[creditKey(a.Name, a.ID)] || c.DirectCredits[creditKey(a.Name, "")])
}

func (s *CreditInventory) Validate() error {
	if (s.Phase != "" && s.Phase != "albums" && s.Phase != "tracks") || s.Offset < 0 || s.Offset > 2000000 || s.Albums < 0 || s.Albums > 100000 || s.Tracks < 0 || s.Tracks > 2000000 || s.At < 0 || s.RetryAt < 0 || len(s.Credits) > 100000 || len(s.Pending) > 100000 {
		return fmt.Errorf("state.json contains invalid direct-credit scan metadata")
	}
	return nil
}
func (s *CreditInventory) ScanDue(now time.Time, force bool) bool {
	if s.Phase == "" && (force || !s.Ready || now.Unix()-s.At >= 900) {
		s.Phase = "albums"
		s.Offset = 0
		s.Albums = 0
		s.Tracks = 0
		s.Pending = map[string]bool{}
		s.RetryAt = 0
	}
	return s.Phase != ""
}
func (s *CreditInventory) Restart(now time.Time) {
	s.Phase = "albums"
	s.Offset = 0
	s.Albums = 0
	s.Tracks = 0
	s.Pending = map[string]bool{}
	s.RetryAt = now.Unix() + 60
}
func (s *CreditInventory) Page(raw string, now time.Time) (bool, error) {
	var response struct {
		Response struct {
			Status string `json:"status"`
			Albums *struct {
				Records []directCredit `json:"album"`
			} `json:"albumList2"`
			Tracks *struct {
				Records []directCredit `json:"song"`
			} `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	if json.Unmarshal([]byte(raw), &response) != nil || response.Response.Status != "ok" {
		return false, fmt.Errorf("unable to read library credits")
	}
	var records []directCredit
	limit := 100000
	switch s.Phase {
	case "albums":
		if response.Response.Albums == nil {
			return false, fmt.Errorf("album list is missing")
		}
		records = response.Response.Albums.Records
	case "tracks":
		if response.Response.Tracks == nil {
			return false, fmt.Errorf("track list is missing")
		}
		records = response.Response.Tracks.Records
		limit = 2000000
	default:
		return false, fmt.Errorf("invalid credit scan phase")
	}
	if len(records) > 500 || s.Offset+len(records) > limit {
		return false, fmt.Errorf("library credit scan exceeds the supported limit")
	}
	if s.Pending == nil {
		s.Pending = map[string]bool{}
	}
	for _, r := range records {
		if r.ID == "" || normalize(r.Artist) == "" {
			return false, fmt.Errorf("library response is missing whole artist credits")
		}
		s.Pending[creditKey(r.Artist, r.ArtistID)] = true
	}
	if len(s.Pending) > 100000 {
		return false, fmt.Errorf("library credit scan exceeds the supported artist limit")
	}
	s.Offset += len(records)
	if s.Phase == "albums" {
		s.Albums = s.Offset
	} else {
		s.Tracks = s.Offset
	}
	if len(records) == 500 {
		return false, nil
	}
	if s.Phase == "albums" {
		s.Phase = "tracks"
		s.Offset = 0
		return false, nil
	}
	s.Credits = s.Pending
	s.Pending = nil
	s.Phase = ""
	s.Offset = 0
	s.Ready = true
	s.At = now.Unix()
	s.RetryAt = 0
	return true, nil
}
