//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/extism/go-pdk"
	host "release-radar/internal/navhost"
	"release-radar/internal/radar"
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
	c.Username = str("username", "")
	c.CalendarNamespace = str("calendar_namespace", "release-radar")
	var err error
	c.AheadDays, err = strconv.Atoi(str("ahead_days", "90"))
	if err != nil {
		return c, fmt.Errorf("ahead_days must be an integer")
	}
	c.PastDays, err = strconv.Atoi(str("past_days", "30"))
	if err != nil {
		return c, fmt.Errorf("past_days must be an integer")
	}
	c.IncludeSingles = str("include_singles", "false") == "true"
	c.IncludeSecondary = str("include_secondary", "false") == "true"
	c.AutoMatch = str("auto_match", "true") == "true"
	if err = json.Unmarshal([]byte(str("artist_overrides", "{}")), &c.Overrides); err != nil {
		return c, fmt.Errorf("artist_overrides must be a JSON object of Navidrome ID to MusicBrainz ID")
	}
	var excluded []string
	if err = json.Unmarshal([]byte(str("excluded_artists", "[]")), &excluded); err != nil {
		return c, fmt.Errorf("excluded_artists must be a JSON array of Navidrome IDs")
	}
	for _, id := range excluded {
		c.Exclusions[id] = true
	}
	return c, c.Validate()
}
func integrationConfig(c radar.Config) radar.IntegrationConfig {
	return radar.IntegrationConfig{
		Namespace: c.EventNamespace(), NtfyURL: str("ntfy_server", "https://ntfy.sh"), NtfyTopic: str("ntfy_topic", ""), NtfyToken: str("ntfy_access_token", ""), NotifyInitial: str("ntfy_notify_initial", "false") == "true",
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
	exportAllowed := false
	finish := func(problem error) error {
		if problem != nil {
			s.Problem = problem.Error()
		}
		if err := radar.Save(dir, s, c, now); err != nil {
			return err
		}
		if !exportAllowed {
			return nil
		}
		exportID, err := strconv.Atoi(str("export_library_id", "0"))
		if err != nil || exportID < 0 {
			return fmt.Errorf("export_library_id must be a nonnegative integer")
		}
		if exportID > 0 {
			lib, err := host.Library(int32(exportID))
			if err != nil {
				return fmt.Errorf("export library: %w", err)
			}
			if lib.MountPoint == "" {
				return fmt.Errorf("export library filesystem is not mounted")
			}
			b, err := radar.Render(s, c, now)
			if err != nil {
				return err
			}
			if err = radar.ExportReport(lib.MountPoint, b); err != nil {
				return fmt.Errorf("HTML export failed; grant library write access and check folder permissions: %w", err)
			}
			if err = radar.ExportCalendar(lib.MountPoint, radar.CalendarICS(radar.Events(s, c, c.EventNamespace(), now, true), now)); err != nil {
				return fmt.Errorf("calendar export failed: %w", err)
			}
		}
		return nil
	}
	if configErr != nil {
		s = radar.State{Schema: 1}
		return finish(configErr)
	}
	users, err := host.Users()
	if err != nil {
		return finish(err)
	}
	granted := make([]string, 0, len(users))
	for _, u := range users {
		granted = append(granted, u.UserName)
	}
	c.Username, err = radar.ResolveUsername(c.Username, granted)
	if err != nil {
		s = radar.State{Schema: 1}
		return finish(err)
	}
	// Event IDs and delivery targets must use the resolved account, including
	// when selection was automatic, so switching modes preserves event identity.
	ic = integrationConfig(c)
	libs, err := host.Libraries()
	if err != nil {
		return finish(err)
	}
	ids := []int{}
	for _, l := range libs {
		ids = append(ids, int(l.ID))
	}
	rawFolders, err := host.Subsonic("getMusicFolders?u=" + url.QueryEscape(c.Username))
	if err != nil {
		return finish(fmt.Errorf("unable to verify current user library access"))
	}
	ids, err = radar.AccessibleLibraries(rawFolders, ids)
	if err != nil {
		return finish(err)
	}
	scope := fmt.Sprintf("%s|%v", c.Fingerprint(), ids)
	if s.Scope != scope {
		s = radar.State{Schema: 1, Scope: scope}
	}
	if len(ids) == 0 {
		s = radar.State{Schema: 1}
		return finish(fmt.Errorf("assign at least one library shared by the selected user and plugin"))
	}
	exportID, parseErr := strconv.Atoi(str("export_library_id", "0"))
	if parseErr != nil || exportID < 0 || exportID > 2147483647 {
		return finish(fmt.Errorf("export_library_id must be a nonnegative 32-bit library ID"))
	}
	if exportID > 0 {
		found := false
		for _, id := range ids {
			if id == exportID {
				found = true
			}
		}
		if !found {
			return finish(fmt.Errorf("export library must be accessible to both the configured user and plugin"))
		}
	}
	exportAllowed = true
	// Retry discovery failures at most once per minute, not every two seconds.
	refreshToken := str("refresh_token", "")
	if s.RefreshToken != refreshToken {
		s.DiscoveredAt = 0
		s.RefreshToken = refreshToken
	}
	if s.Scope != scope || now.Unix()-s.DiscoveredAt >= 86400 {
		if s.NextHTTP > now.Unix() {
			return nil
		}
		artists, err := discover(c.Username, ids)
		if err != nil {
			s.NextHTTP = now.Unix() + 60
			return finish(err)
		}
		s.Reconcile(artists, c, scope, now)
	}
	previousProblem := s.Problem
	s.Problem = ""
	view := fmt.Sprintf("%d/%d/%t/%t/%s", c.AheadDays, c.PastDays, c.IncludeSingles, c.IncludeSecondary, str("export_library_id", "0")) + ic.Fingerprint()
	viewChanged := s.ViewConfig != view
	s.ViewConfig = view
	d, loadErr := radar.LoadDeliveries(dir)
	if loadErr != nil {
		return finish(loadErr)
	}
	activeIntegrations, configurationProblem := ic.ValidParts()
	deliveryChanged := false
	if d.PreferDelivery {
		deliveryChanged = d.Step(s, c, activeIntegrations, send, now)
	}
	changed := false
	if !deliveryChanged {
		changed = s.Step(func(endpoint string) (radar.Response, error) {
			ua := "NavidromeReleaseRadar/" + radar.Version + " (" + str("contact", "local Navidrome plugin") + ")"
			return send(radar.HTTPRequest{Method: "GET", URL: endpoint, Headers: map[string]string{"User-Agent": ua, "Accept": "application/json"}})
		}, c, now)
		if !changed {
			deliveryChanged = d.Step(s, c, activeIntegrations, send, now)
		}
	}
	if deliveryChanged {
		d.PreferDelivery = false
	} else if changed {
		d.PreferDelivery = true
	}
	s.IntegrationProblem = strings.Trim(strings.Join([]string{configurationProblem, d.Status()}, " · "), " ·")
	if changed || deliveryChanged {
		if err = radar.SaveDeliveries(dir, d); err != nil {
			return err
		}
	}

	// Avoid rewriting the dashboard every idle tick. Render at least hourly for date windows.
	if !changed && !deliveryChanged && !viewChanged && previousProblem == "" && s.DiscoveredAt != now.Unix() && now.Unix()-s.RenderedAt < 3600 {
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
