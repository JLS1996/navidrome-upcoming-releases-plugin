package radar

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type HTTPRequest struct {
	Method, URL string
	Headers     map[string]string
	Body        []byte
}
type Send func(HTTPRequest) (Response, error)
type IntegrationConfig struct {
	Namespace                                                                string
	NtfyURL, NtfyTopic, NtfyToken                                            string
	NotifyInitial                                                            bool
	CalendarProvider                                                         string
	CalDAVURL, CalDAVUser, CalDAVPassword                                    string
	GoogleCalendarID, GoogleClientID, GoogleClientSecret, GoogleRefreshToken string
}

func safeServiceURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func (c IntegrationConfig) Validate() error {
	if c.NtfyTopic != "" {
		if !safeServiceURL(c.NtfyURL) {
			return fmt.Errorf("ntfy server must be an HTTPS URL without credentials, query or fragment")
		}
		for _, r := range c.NtfyTopic {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
				return fmt.Errorf("ntfy topic must contain only letters, numbers, underscores and hyphens")
			}
		}
		if len(c.NtfyTopic) > 128 {
			return fmt.Errorf("ntfy topic is too long")
		}
	}
	switch c.CalendarProvider {
	case "", "none":
	case "caldav":
		if !safeServiceURL(c.CalDAVURL) || c.CalDAVUser == "" || c.CalDAVPassword == "" {
			return fmt.Errorf("CalDAV needs an HTTPS calendar collection URL, username and app password")
		}
	case "google":
		if c.GoogleCalendarID == "" || c.GoogleClientID == "" || c.GoogleClientSecret == "" || c.GoogleRefreshToken == "" {
			return fmt.Errorf("Google Calendar needs calendar ID and OAuth client ID, client secret and refresh token")
		}
	default:
		return fmt.Errorf("calendar_provider must be none, caldav or google")
	}
	return nil
}

type RemoteEvent struct {
	Event   CalendarEvent `json:"event"`
	Hash    string        `json:"hash"`
	ETag    string        `json:"etag,omitempty"`
	Phase   string        `json:"phase,omitempty"`
	Present bool          `json:"present"`
}
type DeliveryState struct {
	Schema         int                     `json:"schema"`
	NtfyTarget     string                  `json:"ntfyTarget,omitempty"`
	NtfyReady      bool                    `json:"ntfyReady"`
	NtfySeen       map[string]string       `json:"ntfySeen,omitempty"`
	NextNtfy       int64                   `json:"nextNtfy"`
	CalendarTarget string                  `json:"calendarTarget,omitempty"`
	Remote         map[string]*RemoteEvent `json:"remote,omitempty"`
	NextCalendar   int64                   `json:"nextCalendar"`
	GoogleAuth     string                  `json:"googleAuth,omitempty"`
	AccessToken    string                  `json:"accessToken,omitempty"`
	TokenExpiry    int64                   `json:"tokenExpiry,omitempty"`
	Problems       map[string]string       `json:"problems,omitempty"`
	Turn           int                     `json:"turn"`
	PreferDelivery bool                    `json:"preferDelivery"`
}

func (d *DeliveryState) init() {
	d.Schema = 1
	if d.Turn < 0 || d.Turn > 1 {
		d.Turn = 0
	}
	if d.NtfySeen == nil {
		d.NtfySeen = map[string]string{}
	}
	if d.Remote == nil {
		d.Remote = map[string]*RemoteEvent{}
	}
	if d.Problems == nil {
		d.Problems = map[string]string{}
	}
}

var integrationTransportBase = "" // Set only by a build-tagged fixture, never by plugin configuration.
var googleAPI = "https://www.googleapis.com/calendar/v3"
var googleTokenAPI = "https://oauth2.googleapis.com/token"

func (d *DeliveryState) failure(service string, resp Response, err error, now time.Time) {
	delay := int64(120)
	if r := RetryDelay(resp.Headers, now); r > delay {
		delay = r
	}
	// Never include remote bodies, request headers or credentials in status/logs.
	if err != nil {
		d.Problems[service] = "request failed; check network access and the plugin host allowlist"
	} else {
		d.Problems[service] = fmt.Sprintf("HTTP %d; check service access and credentials", resp.Status)
	}
	if service == "ntfy" {
		d.NextNtfy = now.Unix() + delay
	} else {
		d.NextCalendar = now.Unix() + delay
	}
}

// Step makes at most one HTTP request; acknowledgements survive restarts.
func (d *DeliveryState) Step(s State, c Config, ic IntegrationConfig, send Send, now time.Time) bool {
	d.init()
	originalSend := send
	if integrationTransportBase != "" {
		send = func(r HTTPRequest) (Response, error) {
			u, err := url.Parse(r.URL)
			if err != nil {
				return Response{}, err
			}
			r.URL = integrationTransportBase + u.RequestURI()
			return originalSend(r)
		}
	}
	for n := 0; n < 2; n++ {
		which := (d.Turn + n) % 2
		var changed bool
		if which == 0 {
			changed = d.ntfyStep(s, c, ic, send, now)
		} else {
			changed = d.calendarStep(s, c, ic, send, now)
		}
		if changed {
			d.Turn = (which + 1) % 2
			return true
		}
	}
	return false
}
func (d *DeliveryState) ntfyStep(s State, c Config, ic IntegrationConfig, send Send, now time.Time) bool {
	if ic.NtfyTopic == "" {
		delete(d.Problems, "ntfy")
		return false
	}
	target := digest(strings.TrimRight(ic.NtfyURL, "/") + "/" + ic.NtfyTopic + "\x00" + ic.Namespace)
	if target != d.NtfyTarget {
		d.NtfyTarget = target
		d.NtfyReady = false
		d.NtfySeen = map[string]string{}
		d.NextNtfy = 0
	}
	if now.Unix() < d.NextNtfy {
		return false
	}
	events := Events(s, c, ic.Namespace, now, false)
	if !d.NtfyReady {
		if !FullCycle(s, c) {
			return false
		}
		d.NtfyReady = true
		if !ic.NotifyInitial {
			for _, e := range events {
				d.NtfySeen[e.Key] = e.Fingerprint()
			}
			delete(d.Problems, "ntfy")
			return true
		}
	}
	for _, e := range events {
		hash := e.Fingerprint()
		if d.NtfySeen[e.Key] == hash {
			continue
		}
		kind := "New music release"
		if _, ok := d.NtfySeen[e.Key]; ok {
			kind = "Music release updated"
		}
		message := []rune(e.Summary + "\n" + DateLabel(e.Date))
		if len(message) > 700 {
			message = message[:700]
		}
		body, _ := json.Marshal(map[string]any{"topic": ic.NtfyTopic, "title": kind, "message": string(message), "tags": []string{"musical_note"}, "click": e.URL})
		headers := map[string]string{"Content-Type": "application/json"}
		if ic.NtfyToken != "" {
			headers["Authorization"] = "Bearer " + ic.NtfyToken
		}
		resp, err := send(HTTPRequest{Method: "POST", URL: strings.TrimRight(ic.NtfyURL, "/") + "/", Headers: headers, Body: body})
		d.NextNtfy = now.Unix() + 2
		if err != nil || resp.Status < 200 || resp.Status >= 300 {
			d.failure("ntfy", resp, err, now)
		} else {
			d.NtfySeen[e.Key] = hash
			delete(d.Problems, "ntfy")
		}
		return true
	}
	return false
}
func (d *DeliveryState) calendarStep(s State, c Config, ic IntegrationConfig, send Send, now time.Time) bool {
	if ic.CalendarProvider == "" || ic.CalendarProvider == "none" {
		delete(d.Problems, "calendar")
		return false
	}
	destination := ic.GoogleCalendarID
	if ic.CalendarProvider == "caldav" {
		destination = strings.TrimRight(ic.CalDAVURL, "/")
	}
	target := digest(ic.CalendarProvider + "\x00" + destination + "\x00" + ic.Namespace)
	if target != d.CalendarTarget {
		d.CalendarTarget = target
		d.Remote = map[string]*RemoteEvent{}
		d.NextCalendar = 0
	}
	if now.Unix() < d.NextCalendar {
		return false
	}
	if ic.CalendarProvider == "google" {
		auth := digest(ic.GoogleClientID + "\x00" + ic.GoogleClientSecret + "\x00" + ic.GoogleRefreshToken)
		if d.GoogleAuth != auth {
			d.GoogleAuth = auth
			d.AccessToken = ""
			d.TokenExpiry = 0
		}
		if d.AccessToken == "" || d.TokenExpiry <= now.Unix()+60 {
			body := url.Values{"grant_type": {"refresh_token"}, "client_id": {ic.GoogleClientID}, "client_secret": {ic.GoogleClientSecret}, "refresh_token": {ic.GoogleRefreshToken}}
			resp, err := send(HTTPRequest{Method: "POST", URL: googleTokenAPI, Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, Body: []byte(body.Encode())})
			var token struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int64  `json:"expires_in"`
			}
			if err != nil || resp.Status != 200 || json.Unmarshal(resp.Body, &token) != nil || token.AccessToken == "" || token.ExpiresIn < 90 {
				d.failure("calendar", resp, err, now)
			} else {
				d.AccessToken = token.AccessToken
				d.TokenExpiry = now.Unix() + token.ExpiresIn
				delete(d.Problems, "calendar")
			}
			return true
		}
	}
	desired := map[string]CalendarEvent{}
	for _, e := range Events(s, c, ic.Namespace, now, true) {
		desired[e.UID] = e
	}
	keys := []string{}
	for k := range desired {
		keys = append(keys, k)
	}
	for k := range d.Remote {
		if _, ok := desired[k]; !ok && FullCycle(s, c) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, uid := range keys {
		e, wanted := desired[uid]
		record, exists := d.Remote[uid]
		if !exists {
			record = &RemoteEvent{Event: e, Phase: "get"}
			d.Remote[uid] = record
		}
		if wanted && record.Hash == e.Fingerprint() && record.Phase == "" {
			continue
		}
		if !wanted && !record.Present && record.Phase == "" {
			delete(d.Remote, uid)
			continue
		}
		if record.Phase == "" {
			if record.ETag == "" {
				record.Phase = "get"
			} else {
				record.Phase = "write"
			}
		}
		if ic.CalendarProvider == "caldav" {
			d.caldavOp(uid, e, wanted, record, ic, send, now)
		} else {
			d.googleOp(uid, e, wanted, record, ic, send, now)
		}
		d.NextCalendar = max(d.NextCalendar, now.Unix()+2)
		return true
	}
	return false
}
func (d *DeliveryState) caldavOp(uid string, e CalendarEvent, wanted bool, r *RemoteEvent, ic IntegrationConfig, send Send, now time.Time) {
	address := strings.TrimRight(ic.CalDAVURL, "/") + "/" + strings.Split(uid, "@")[0] + ".ics"
	headers := map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(ic.CalDAVUser+":"+ic.CalDAVPassword))}
	request := HTTPRequest{URL: address, Headers: headers}
	phase := r.Phase
	if phase == "get" {
		request.Method = "GET"
	} else if !wanted {
		request.Method = "DELETE"
		headers["If-Match"] = r.ETag
	} else {
		request.Method = "PUT"
		request.Body = CalendarICS([]CalendarEvent{e}, now)
		headers["Content-Type"] = "text/calendar; charset=utf-8"
		if r.Present {
			headers["If-Match"] = r.ETag
		} else {
			headers["If-None-Match"] = "*"
		}
	}
	resp, err := send(request)
	if err != nil {
		d.failure("calendar", resp, err, now)
		return
	}
	if phase == "get" {
		if resp.Status == 404 {
			r.Present = false
			r.ETag = ""
			if !wanted {
				delete(d.Remote, uid)
			} else {
				r.Phase = "write"
			}
			delete(d.Problems, "calendar")
			return
		}
		if resp.Status == 200 {
			if !ownedICS(resp.Body, uid) {
				d.Problems["calendar"] = "event ownership conflict; unrelated CalDAV event was left untouched"
				d.NextCalendar = now.Unix() + 3600
				return
			}
			r.ETag = header(resp.Headers, "ETag")
			if r.ETag == "" {
				d.Problems["calendar"] = "CalDAV did not supply an ETag; refusing an unprotected update"
				d.NextCalendar = now.Unix() + 3600
				return
			}
			r.Present = true
			r.Phase = "write"
			delete(d.Problems, "calendar")
			return
		}
	} else {
		if resp.Status == 412 || resp.Status == 409 || (wanted && resp.Status == 404) {
			r.Phase = "get"
			return
		}
		if !wanted && (resp.Status == 404 || resp.Status >= 200 && resp.Status < 300) {
			delete(d.Remote, uid)
			delete(d.Problems, "calendar")
			return
		}
		if wanted && resp.Status >= 200 && resp.Status < 300 {
			r.Present = true
			r.Phase = ""
			r.Event = e
			r.Hash = e.Fingerprint()
			r.ETag = header(resp.Headers, "ETag")
			delete(d.Problems, "calendar")
			return
		}
	}
	d.failure("calendar", resp, nil, now)
}
func googleID(uid string) string { return "rr" + digest(uid) } // Hex is a valid subset of Google's base32hex alphabet.
func (d *DeliveryState) googleOp(uid string, e CalendarEvent, wanted bool, r *RemoteEvent, ic IntegrationConfig, send Send, now time.Time) {
	collection := googleAPI + "/calendars/" + url.PathEscape(ic.GoogleCalendarID) + "/events"
	address := collection + "/" + googleID(uid)
	headers := map[string]string{"Authorization": "Bearer " + d.AccessToken, "Content-Type": "application/json"}
	req := HTTPRequest{URL: address, Headers: headers}
	phase := r.Phase
	if phase == "get" {
		req.Method = "GET"
	} else if !wanted {
		req.Method = "DELETE"
		headers["If-Match"] = r.ETag
	} else {
		start, _, _ := DateInterval(e.Date)
		payload := map[string]any{"summary": e.Summary, "description": e.Description + "\n" + e.URL, "start": map[string]string{"date": e.Date}, "end": map[string]string{"date": start.AddDate(0, 0, 1).Format("2006-01-02")}, "transparency": "transparent", "extendedProperties": map[string]any{"private": map[string]string{"releaseRadarUID": uid, "releaseRadarHash": e.Fingerprint()}}}
		if r.Present {
			req.Method = "PATCH"
			headers["If-Match"] = r.ETag
		} else {
			req.Method = "POST"
			req.URL = collection
			payload["id"] = googleID(uid)
		}
		req.Body, _ = json.Marshal(payload)
	}
	resp, err := send(req)
	if err != nil {
		d.failure("calendar", resp, err, now)
		return
	}
	if resp.Status == 401 {
		d.AccessToken = ""
		d.TokenExpiry = 0
		d.failure("calendar", resp, nil, now)
		return
	}
	if phase == "get" {
		if resp.Status == 404 || resp.Status == 410 {
			r.Present = false
			r.ETag = ""
			if !wanted {
				delete(d.Remote, uid)
			} else {
				r.Phase = "write"
			}
			delete(d.Problems, "calendar")
			return
		}
		if resp.Status == 200 {
			var remote struct {
				ETag     string `json:"etag"`
				Extended struct {
					Private map[string]string `json:"private"`
				} `json:"extendedProperties"`
			}
			if json.Unmarshal(resp.Body, &remote) != nil || remote.Extended.Private["releaseRadarUID"] != uid {
				d.Problems["calendar"] = "event ownership conflict; unrelated Google event was left untouched"
				d.NextCalendar = now.Unix() + 3600
				return
			}
			if remote.ETag == "" {
				d.Problems["calendar"] = "Google event is missing its ETag"
				d.NextCalendar = now.Unix() + 3600
				return
			}
			r.ETag = remote.ETag
			r.Present = true
			if wanted && remote.Extended.Private["releaseRadarHash"] == e.Fingerprint() {
				r.Phase = ""
				r.Event = e
				r.Hash = e.Fingerprint()
			} else {
				r.Phase = "write"
			}
			delete(d.Problems, "calendar")
			return
		}
	} else {
		if resp.Status == 409 || resp.Status == 412 || (wanted && (resp.Status == 404 || resp.Status == 410)) {
			r.Phase = "get"
			return
		}
		if !wanted && (resp.Status == 404 || resp.Status == 410 || resp.Status >= 200 && resp.Status < 300) {
			delete(d.Remote, uid)
			delete(d.Problems, "calendar")
			return
		}
		if wanted && resp.Status >= 200 && resp.Status < 300 {
			var result struct {
				ETag string `json:"etag"`
				ID   string `json:"id"`
			}
			if json.Unmarshal(resp.Body, &result) != nil || result.ETag == "" || result.ID != googleID(uid) {
				r.Phase = "get"
				d.Problems["calendar"] = "Google write was not confirmed by a valid event response; checking before retry"
				d.NextCalendar = now.Unix() + 120
				return
			}
			r.ETag = result.ETag
			r.Present = true
			r.Event = e
			r.Hash = e.Fingerprint()
			r.Phase = ""
			delete(d.Problems, "calendar")
			return
		}
	}
	d.failure("calendar", resp, nil, now)
}
func header(h map[string]string, key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
func (d DeliveryState) Status() string {
	keys := []string{}
	for key := range d.Problems {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		parts = append(parts, key+": "+d.Problems[key])
	}
	return strings.Join(parts, " · ")
}

func (c IntegrationConfig) Fingerprint() string { b, _ := json.Marshal(c); return digest(string(b)) }

// ValidParts prevents a malformed optional service from blocking another service.
func (c IntegrationConfig) ValidParts() (IntegrationConfig, string) {
	active := c
	problems := []string{}
	onlyNtfy := c
	onlyNtfy.CalendarProvider = "none"
	if err := onlyNtfy.Validate(); err != nil {
		active.NtfyTopic = ""
		problems = append(problems, "ntfy: "+err.Error())
	}
	onlyCalendar := c
	onlyCalendar.NtfyTopic = ""
	if err := onlyCalendar.Validate(); err != nil {
		active.CalendarProvider = "none"
		problems = append(problems, "calendar: "+err.Error())
	}
	return active, strings.Join(problems, " · ")
}
