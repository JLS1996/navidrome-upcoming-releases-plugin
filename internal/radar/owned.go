package radar

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LibraryAlbum struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Artist       string               `json:"artist"`
	ArtistID     string               `json:"artistId,omitempty"`
	Artists      []LibraryAlbumArtist `json:"artists,omitempty"`
	ReleaseTypes []string             `json:"releaseTypes,omitempty"`
}
type LibraryAlbumArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OwnedScanDue refreshes local ownership periodically and on either saved manual
// refresh transition. Every page is scoped to the verified user/library grants.
func (s *State) OwnedScanDue(now time.Time, force bool) bool {
	if force || !s.OwnedReady || now.Unix()-s.OwnedAt >= 900 {
		if !s.OwnedScanning {
			s.OwnedScanning = true
			s.OwnedOffset = 0
			s.OwnedPending = nil
			s.OwnedRetryAt = 0
		}
	}
	return s.OwnedScanning
}
func (s *State) OwnedPage(raw string, now time.Time) (bool, error) {
	var response struct {
		Response struct {
			Status string `json:"status"`
			List   *struct {
				Albums []LibraryAlbum `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	if json.Unmarshal([]byte(raw), &response) != nil || response.Response.Status != "ok" || response.Response.List == nil {
		return false, fmt.Errorf("unable to read library albums for owned-release filtering")
	}
	albums := response.Response.List.Albums
	if len(albums) > 500 || s.OwnedOffset+len(albums) > 100000 {
		return false, fmt.Errorf("owned-album scan exceeds the supported limit")
	}
	seen := map[string]bool{}
	for _, a := range s.OwnedPending {
		seen[a.ID] = true
	}
	for _, a := range albums {
		if a.ID == "" || a.Name == "" || seen[a.ID] {
			return false, fmt.Errorf("library album pagination changed or returned invalid data; retrying")
		}
		seen[a.ID] = true
	}
	s.OwnedPending = append(s.OwnedPending, albums...)
	s.OwnedOffset += len(albums)
	if len(albums) == 500 {
		return false, nil
	}
	s.OwnedAlbums = s.OwnedPending
	s.OwnedPending = nil
	s.OwnedOffset = 0
	s.OwnedScanning = false
	s.OwnedReady = true
	s.OwnedAt = now.Unix()
	s.OwnedRetryAt = 0
	return true, nil
}
func ownedAlbumMatches(album LibraryAlbum, artist Artist, r Release) bool {
	if normalize(album.Name) != normalize(r.Title) {
		return false
	}
	matched := false
	if album.ArtistID != "" && artist.ID != "" {
		matched = album.ArtistID == artist.ID
	} else {
		matched = normalize(album.Artist) == normalize(artist.Name)
	}
	for _, credit := range album.Artists {
		if (credit.ID != "" && credit.ID == artist.ID) || (credit.ID == "" && normalize(credit.Name) == normalize(artist.Name)) {
			matched = true
		}
	}
	if !matched {
		return false
	}
	if len(album.ReleaseTypes) == 0 {
		return true
	}
	primaryKnown := false
	primaryMatch := false
	for _, option := range PrimaryReleaseTypes {
		for _, tag := range album.ReleaseTypes {
			if strings.EqualFold(tag, option.Value) {
				primaryKnown = true
				if strings.EqualFold(tag, r.PrimaryType()) {
					primaryMatch = true
				}
			}
		}
	}
	if primaryKnown && !primaryMatch {
		return false
	}
	localSecondary := map[string]bool{}
	remoteSecondary := map[string]bool{}
	for _, option := range SecondaryReleaseTypes {
		for _, tag := range album.ReleaseTypes {
			if strings.EqualFold(tag, option.Value) {
				localSecondary[normalize(tag)] = true
			}
		}
		for _, tag := range r.Secondary {
			if strings.EqualFold(tag, option.Value) {
				remoteSecondary[normalize(tag)] = true
			}
		}
	}
	if len(localSecondary) != len(remoteSecondary) {
		return false
	}
	for tag := range localSecondary {
		if !remoteSecondary[tag] {
			return false
		}
	}
	return true
}
func alreadyOwned(albums []LibraryAlbum, artist Artist, r Release) bool {
	for _, album := range albums {
		if ownedAlbumMatches(album, artist, r) {
			return true
		}
	}
	return false
}
