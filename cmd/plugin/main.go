//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	host "github.com/JLS1996/navidrome-upcoming-releases-plugin/internal/navhost"
	"github.com/JLS1996/navidrome-upcoming-releases-plugin/internal/radar"
	"github.com/extism/go-pdk"
)

type plugin struct{}

func exported(err error) int32 {
	if err != nil {
		pdk.SetError(err)
		return -1
	}
	return 0
}

//go:wasmexport nd_on_init
func initialize() int32 { return exported((&plugin{}).OnInit()) }

//go:wasmexport nd_scheduler_callback
func scheduled() int32 { return exported((&plugin{}).tick()) }
func main()            {}
func (p *plugin) OnInit() error {
	// Retain the scheduler key used by existing installations.
	if err := host.Schedule("@every 2s", "tick", "release-radar-tick"); err != nil {
		return err
	}
	return p.tick()
}
func str(key, def string) string {
	v, ok := host.ConfigValue(key)
	if !ok || v == "" {
		return def
	}
	return v
}
func config() (radar.Config, error) {
	c := radar.DefaultConfig()
	c.Theme = str("dashboard_theme", "dark")
	c.CoverArt = str("cover_art", "false") == "true"
	c.ExcludeUncredited = str("exclude_uncredited", "false") == "true"
	c.ExcludeOwned = str("exclude_owned", "false") == "true"
	c.CalendarNamespace = str("calendar_namespace", "navidrome-upcoming-releases-plugin")
	var err error
	c.AheadDays, err = strconv.Atoi(str("ahead_days", "90"))
	if err != nil {
		return c, fmt.Errorf("ahead_days must be an integer")
	}
	c.PastDays, err = strconv.Atoi(str("past_days", "30"))
	if err != nil {
		return c, fmt.Errorf("past_days must be an integer")
	}
	c.PrimaryTypes = nil
	for _, option := range radar.PrimaryReleaseTypes {
		fallback := "false"
		if option.Value == "Album" || option.Value == "EP" {
			fallback = "true"
		}
		if str(option.Key, fallback) == "true" {
			c.PrimaryTypes = append(c.PrimaryTypes, option.Value)
		}
	}
	c.IncludeStandard = str("include_standard", "true") == "true"
	for _, option := range radar.SecondaryReleaseTypes {
		if str(option.Key, "false") == "true" {
			c.SecondaryTypes = append(c.SecondaryTypes, option.Value)
		}
	}
	c.AutoMatch = str("auto_match", "true") == "true"
	if err = json.Unmarshal([]byte(str("artist_overrides", "{}")), &c.Overrides); err != nil || c.Overrides == nil {
		return c, fmt.Errorf("artist_overrides must be a JSON object of Navidrome ID to MusicBrainz ID")
	}
	var excluded []string
	if err = json.Unmarshal([]byte(str("excluded_artists", "[]")), &excluded); err != nil || excluded == nil {
		return c, fmt.Errorf("excluded_artists must be a JSON array of Navidrome IDs")
	}
	for _, id := range excluded {
		c.Exclusions[id] = true
	}
	return c, c.Validate()
}
func integrationConfig(c radar.Config) radar.IntegrationConfig {
	return radar.IntegrationConfig{
		Namespace: c.EventNamespace(), NtfyEnabled: str("ntfy_enabled", "false") == "true", NotifyNew: str("ntfy_notify_new", "true") == "true", NotifyUpdates: str("ntfy_notify_updates", "true") == "true", NotifyReleaseDay: str("ntfy_notify_release_day", "false") == "true", NtfyURL: str("ntfy_server", "https://ntfy.sh"), NtfyTopic: str("ntfy_topic", ""), NtfyToken: str("ntfy_access_token", ""), NotifyInitial: str("ntfy_notify_initial", "false") == "true",
		CalendarProvider: str("calendar_provider", "none"), CalDAVURL: str("caldav_calendar_url", ""), CalDAVUser: str("caldav_username", ""), CalDAVPassword: str("caldav_app_password", ""),
		GoogleCalendarID: str("google_calendar_id", ""), GoogleClientID: str("google_client_id", ""), GoogleClientSecret: str("google_client_secret", ""), GoogleRefreshToken: str("google_refresh_token", "")}
}
func send(req radar.HTTPRequest) (radar.Response, error) {
	response, err := host.Do(host.Request{Method: req.Method, URL: req.URL, Headers: req.Headers, Body: req.Body, TimeoutMs: 6000, NoFollowRedirects: true})
	if err != nil {
		return radar.Response{}, err
	}
	if response == nil {
		return radar.Response{}, fmt.Errorf("empty HTTP host response")
	}
	return radar.Response{Status: int(response.StatusCode), Headers: response.Headers, Body: response.Body}, nil
}
func (p *plugin) tick() error {
	now := time.Now().UTC()
	dir, err := host.StorageRoot()
	if err != nil {
		return err
	}
	s, err := radar.Load(dir)
	if err != nil {
		return err
	}
	c, configErr := config()
	ic := integrationConfig(c)
	finish := func(problem error) error {
		if problem != nil {
			s.Problem = problem.Error()
		}
		return radar.Save(dir, s, c, now)
	}
	deny := func(problem error) error { s = radar.State{Schema: 1}; return finish(problem) }
	if configErr != nil {
		s = radar.State{Schema: 1}
		return finish(configErr)
	}
	users, err := host.Users()
	if err != nil {
		return deny(fmt.Errorf("unable to verify granted user access"))
	}
	granted := make([]string, 0, len(users))
	for _, u := range users {
		granted = append(granted, u.UserName)
	}
	c.Username, err = radar.ResolveUsername(granted)
	if err != nil {
		s = radar.State{Schema: 1}
		return finish(err)
	}
	// Keep stable event identities tied to the granted Navidrome account.
	ic = integrationConfig(c)
	libs, err := host.Libraries()
	if err != nil {
		return deny(fmt.Errorf("unable to verify granted library access"))
	}
	ids := []int{}
	for _, l := range libs {
		ids = append(ids, int(l.ID))
	}
	rawFolders, err := host.Subsonic("getMusicFolders?u=" + url.QueryEscape(c.Username))
	if err != nil {
		return deny(fmt.Errorf("unable to verify current user library access"))
	}
	ids, err = radar.AccessibleLibraries(rawFolders, ids)
	if err != nil {
		return deny(err)
	}
	scope := fmt.Sprintf("%s|%v", c.Fingerprint(), ids)
	if s.Scope != scope {
		s = radar.State{Schema: 1, Scope: scope}
	}
	if len(ids) == 0 {
		s = radar.State{Schema: 1}
		return finish(fmt.Errorf("assign at least one library shared by the selected user and plugin"))
	}
	c.DirectReady, c.DirectCredits = s.Direct.Ready, s.Direct.Credits
	refreshChanged := s.ApplyRefresh(str("manual_refresh", "false") == "true")
	if refreshChanged {
		s.Direct.At = 0
	}
	if c.ExcludeOwned {
		c.OwnershipReady = s.OwnedReady
		c.OwnedAlbums = s.OwnedAlbums
		if s.OwnedScanDue(now, refreshChanged) && now.Unix() >= s.OwnedRetryAt {
			q := url.Values{"u": {c.Username}, "type": {"alphabeticalByName"}, "size": {"500"}, "offset": {strconv.Itoa(s.OwnedOffset)}}
			for _, id := range ids {
				q.Add("musicFolderId", strconv.Itoa(id))
			}
			raw, scanErr := host.Subsonic("getAlbumList2?" + q.Encode())
			complete := false
			if scanErr == nil {
				complete, scanErr = s.OwnedPage(raw, now)
			}
			if scanErr != nil {
				s.OwnedRetryAt = now.Unix() + 60
				s.OwnedOffset = 0
				s.OwnedPending = nil
				return finish(fmt.Errorf("owned-album scan failed; retrying in one minute"))
			}
			if !complete {
				return finish(fmt.Errorf("Checking library albums before excluding owned releases: %d scanned", s.OwnedOffset))
			}
			c.OwnershipReady = true
			c.OwnedAlbums = s.OwnedAlbums
		}
		if s.OwnedScanning {
			return finish(fmt.Errorf("Owned-album scan is waiting to retry; integrations are paused"))
		}
	}
	if c.ExcludeUncredited {
		c.DirectReady, c.DirectCredits = s.Direct.Ready, s.Direct.Credits
		if s.Direct.ScanDue(now, refreshChanged) && now.Unix() >= s.Direct.RetryAt {
			q := url.Values{"u": {c.Username}}
			for _, id := range ids {
				q.Add("musicFolderId", strconv.Itoa(id))
			}
			endpoint := "getAlbumList2"
			if s.Direct.Phase == "albums" {
				q.Set("type", "alphabeticalByName")
				q.Set("size", "500")
				q.Set("offset", strconv.Itoa(s.Direct.Offset))
			} else {
				endpoint = "search3"
				q.Set("query", "")
				q.Set("artistCount", "0")
				q.Set("albumCount", "0")
				q.Set("songCount", "500")
				q.Set("songOffset", strconv.Itoa(s.Direct.Offset))
			}
			raw, scanErr := host.Subsonic(endpoint + "?" + q.Encode())
			complete := false
			if scanErr == nil {
				complete, scanErr = s.Direct.Page(raw, now)
			}
			if scanErr != nil {
				s.Direct.Restart(now)
				return finish(fmt.Errorf("direct-credit scan failed: %v; retrying in one minute", scanErr))
			}
			if !complete {
				return finish(fmt.Errorf("Checking directly credited music: %d albums and %d tracks scanned; integrations are paused", s.Direct.Albums, s.Direct.Tracks))
			}
			c.DirectReady, c.DirectCredits = true, s.Direct.Credits
		}
		if s.Direct.Phase != "" {
			return finish(fmt.Errorf("Direct-credit scan is waiting to retry; integrations are paused"))
		}
	}
	if (s.RefreshPending || now.Unix()-s.DiscoveredAt >= 86400) && now.Unix() >= s.NextDiscovery {
		artists, err := discover(c.Username, ids)
		if err != nil {
			s.NextDiscovery = now.Unix() + 60
			return finish(err)
		}
		s.Reconcile(artists, c, scope, now)
	}
	previousProblem := s.Problem
	if s.NextDiscovery == 0 {
		s.Problem = ""
	}
	view := radar.Version + "/" + fmt.Sprintf("%d/%d/%v/%v/%t", c.AheadDays, c.PastDays, c.PrimaryTypes, c.SecondaryTypes, c.IncludeStandard) + fmt.Sprint(c.Theme, c.CoverArt, c.ExcludeOwned, s.OwnedAt, c.ExcludeUncredited, s.Direct.At) + ic.Fingerprint()
	viewChanged := s.ViewConfig != view
	s.ViewConfig = view
	d, loadErr := radar.LoadDeliveries(dir)
	if loadErr != nil {
		return finish(loadErr)
	}
	var artwork radar.ArtworkState
	var artworkErr error
	artworkChanged := false
	if c.CoverArt {
		artwork, artworkErr = radar.LoadArtwork(dir)
		if artworkErr == nil {
			artworkChanged = artwork.Reconcile(s, c, now)
		}
	}
	artworkRequest := false
	if c.CoverArt && artworkErr == nil && s.PreferArtwork {
		artworkRequest = artwork.Step(send, now)
	}
	activeIntegrations, configurationProblem := ic.ValidParts()
	deliveryChanged := false
	if !artworkRequest && d.PreferDelivery {
		deliveryChanged = d.Step(s, c, activeIntegrations, send, now)
	}
	changed := false
	if !artworkRequest && !deliveryChanged {
		changed = s.Step(func(endpoint string) (radar.Response, error) {
			ua := radar.MusicBrainzUserAgent()
			return send(radar.HTTPRequest{Method: "GET", URL: endpoint, Headers: map[string]string{"User-Agent": ua, "Accept": "application/json"}})
		}, c, now)
		if !changed {
			deliveryChanged = d.Step(s, c, activeIntegrations, send, now)
		}
	}
	if !artworkRequest && !deliveryChanged && !changed && c.CoverArt && artworkErr == nil {
		artworkRequest = artwork.Step(send, now)
	}
	if artworkRequest {
		s.PreferArtwork = false
	} else if changed || deliveryChanged {
		s.PreferArtwork = true
	}
	previousArtworkProblem := s.ArtworkProblem
	s.ArtworkProblem = ""
	if c.CoverArt {
		if artworkErr != nil {
			s.ArtworkProblem = "Cover cache could not be read; preserve covers.json before resetting it"
		} else {
			c.Covers = artwork.Images()
			s.ArtworkProblem = artwork.Problem
			if artworkChanged || artworkRequest {
				if err = radar.SaveArtwork(dir, artwork); err != nil {
					s.ArtworkProblem = "Cover cache could not be saved"
				}
			}
		}
	}
	if deliveryChanged {
		d.PreferDelivery = false
	} else if changed {
		d.PreferDelivery = true
	}
	previousIntegrationProblem := s.IntegrationProblem
	s.IntegrationProblem = strings.Trim(strings.Join([]string{configurationProblem, d.Status()}, " · "), " ·")
	if changed || deliveryChanged {
		if err = radar.SaveDeliveries(dir, d); err != nil {
			return err
		}
	}

	// Avoid rewriting the dashboard every idle tick. Render at least hourly for date windows.
	if !refreshChanged && !changed && !deliveryChanged && !artworkRequest && !artworkChanged && previousArtworkProblem == s.ArtworkProblem && !viewChanged && previousProblem == "" && previousIntegrationProblem == s.IntegrationProblem && s.DiscoveredAt != now.Unix() && now.Unix()-s.RenderedAt < 3600 {
		return nil
	}
	return finish(nil)
}
func discover(username string, ids []int) ([]radar.Artist, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no verified libraries were selected")
	}
	q := url.Values{"u": {username}}
	for _, id := range ids {
		q.Add("musicFolderId", strconv.Itoa(id))
	}
	raw, err := host.Subsonic("getArtists?" + q.Encode())
	if err != nil {
		return nil, err
	}
	artists, err := radar.ParseArtists(raw)
	if err != nil {
		return nil, err
	}
	result := []radar.Artist{}
	for _, a := range artists {
		if !strings.EqualFold(strings.TrimSpace(a.Name), "Various Artists") {
			result = append(result, a)
		}
	}
	return result, nil
}
