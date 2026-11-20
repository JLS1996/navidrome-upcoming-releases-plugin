package radar

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

//go:embed dashboard.html
var dashboard string

type row struct{ Artist, Title, Type, TypeLabel, Secondary, Date, DateLabel, URL, Status, CheckedAt string }
type issue struct{ Name, ID, Message string }
type report struct {
	Generated                  string
	Artists, Complete, Pending int
	Rows                       []row
	Issues                     []issue
	Problem                    string
	AheadDays, PastDays        int
}

func Render(s State, c Config, now time.Time) ([]byte, error) {
	p := report{Generated: now.UTC().Format("2 Jan 2006, 15:04 UTC"), Artists: len(s.Entries), Problem: s.Problem, AheadDays: c.AheadDays, PastDays: c.PastDays}
	today := now.UTC().Format("2006-01-02")
	for _, e := range s.Entries {
		if e.Phase == "done" {
			p.Complete++
		} else if e.Phase == "match" || e.Phase == "releases" {
			p.Pending++
		}
		if e.Problem != "" {
			p.Issues = append(p.Issues, issue{e.Artist.Name, e.Artist.ID, e.Problem})
		}
	}
	for _, r := range visibleReleases(s, c, now) {
		status := "Upcoming"
		_, end, _ := DateInterval(r.Date)
		if len(r.Date) < 10 {
			status = "Date uncertain"
		} else if end.Format("2006-01-02") < today {
			status = "Recent"
		} else if r.Date == today {
			status = "Today"
		}
		p.Rows = append(p.Rows, row{Artist: r.ArtistName, Title: r.Title, Type: r.PrimaryType(), TypeLabel: r.TypeLabel(), Secondary: strings.Join(r.Secondary, "|"), Date: r.Date, DateLabel: DateLabel(r.Date), URL: r.SourceURL(), Status: status, CheckedAt: r.CheckedAt})
	}
	sort.SliceStable(p.Rows, func(i, j int) bool {
		a, b := p.Rows[i], p.Rows[j]
		if a.Date == b.Date {
			return strings.ToLower(a.Artist+a.Title) < strings.ToLower(b.Artist+b.Title)
		}
		return a.Date < b.Date
	})
	// Avoid text/template reflection, unsupported by TinyGo 0.40.1 at runtime.
	esc := html.EscapeString
	var rows, issues strings.Builder
	for _, r := range p.Rows {
		fmt.Fprintf(&rows, `<article class="release" data-type="%s" data-secondary="%s" data-period="%s" data-date="%s"><div class="date">%s<br><span class="meta">%s</span></div><div><div class="artist">%s</div><h2><a href="%s" target="_blank" rel="noopener noreferrer">%s</a></h2><div class="meta">Last checked: %s</div></div><span class="tag">%s</span></article>`, esc(r.Type), esc(r.Secondary), esc(r.Status), esc(r.Date), esc(r.DateLabel), esc(r.Status), esc(r.Artist), esc(r.URL), esc(r.Title), esc(r.CheckedAt), esc(r.TypeLabel))
	}
	if len(p.Issues) > 0 {
		summary := fmt.Sprintf("%d artists need attention", len(p.Issues))
		if len(p.Issues) == 1 {
			summary = "1 artist needs attention"
		}
		fmt.Fprintf(&issues, `<details><summary>%s</summary><ul>`, summary)
		for _, i := range p.Issues {
			fmt.Fprintf(&issues, `<li><strong>%s</strong> · Navidrome ID <code>%s</code><br>%s</li>`, esc(i.Name), esc(i.ID), esc(i.Message))
		}
		issues.WriteString("</ul></details>")
	}
	if s.MusicBrainzProblem != "" {
		retry := time.Unix(s.NextHTTP, 0).UTC().Format("2 Jan 2006, 15:04:05 UTC")
		status := s.MusicBrainzProblem + ". Checks paused; automatic retry after " + retry + ". Existing results are kept."
		if p.Problem != "" {
			p.Problem += " · "
		}
		p.Problem += status
	}
	problem := ""
	if s.IntegrationProblem != "" {
		if p.Problem != "" {
			p.Problem += " · "
		}
		p.Problem += "Integration status: " + s.IntegrationProblem
	}
	if p.Problem != "" {
		problem = `<div class="notice" role="status">` + esc(p.Problem) + `</div>`
	}
	hidden := ""
	if len(p.Rows) > 0 {
		hidden = "hidden"
	}
	events := Events(s, c, c.EventNamespace(), now, true)
	ics := CalendarICS(events, now)
	emptyNote := ""
	if len(events) == 0 {
		emptyNote = `<p class="notice">No dated calendar events yet. Importing this calendar will add nothing. Checks may still be running, or no exact release dates fall within your filters.</p>`
	}
	var typeOptions strings.Builder
	for _, option := range append(append([]ReleaseTypeOption{}, PrimaryReleaseTypes...), SecondaryReleaseTypes...) {
		fmt.Fprintf(&typeOptions, `<option value="%s">%s</option>`, esc(option.Value), esc(option.Value))
	}
	// NewReplacer makes a single pass: inserted metadata is never interpreted as a template.
	result := strings.NewReplacer("@@TYPE_OPTIONS@@", typeOptions.String(), "@@CALENDAR_EMPTY_NOTE@@", emptyNote, "@@CALENDAR_TEXT@@", esc(string(ics)), "@@ICS@@", base64.StdEncoding.EncodeToString(ics), "@@CALENDAR_STATS@@", esc(CalendarStats(s, c, c.EventNamespace(), now)), "@@TODAY@@", now.UTC().Format("2006-01-02"), "@@COUNT@@", fmt.Sprint(len(p.Rows)), "@@COMPLETE@@", fmt.Sprint(p.Complete), "@@ARTISTS@@", fmt.Sprint(p.Artists), "@@PENDING@@", fmt.Sprint(p.Pending), "@@PAST@@", fmt.Sprint(p.PastDays), "@@AHEAD@@", fmt.Sprint(p.AheadDays), "@@GENERATED@@", esc(p.Generated), "@@PROBLEM@@", problem, "@@ROWS@@", rows.String(), "@@ISSUES@@", issues.String(), "@@EMPTYHIDDEN@@", hidden).Replace(dashboard)
	return []byte(result), nil
}
