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

type row struct{ Artist, Title, Type, Date, DateLabel, URL, Status, CheckedAt string }
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
		for _, r := range e.Releases {
			if !Visible(r, c, now) || !r.ValidID() {
				continue
			}
			status := "Upcoming"
			_, end, _ := DateInterval(r.Date)
			if end.Format("2006-01-02") < today {
				status = "Recent"
			} else if len(r.Date) < 10 {
				status = "Date uncertain"
			} else if r.Date == today {
				status = "Today"
			}
			p.Rows = append(p.Rows, row{e.Artist.Name, r.Title, r.Type, r.Date, DateLabel(r.Date), r.SourceURL(), status, e.CheckedAt})
		}
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
		fmt.Fprintf(&rows, `<article class="release" data-type="%s" data-period="%s" data-date="%s"><div class="date">%s<br><span class="meta">%s</span></div><div><div class="artist">%s</div><h2><a href="%s" target="_blank" rel="noopener noreferrer">%s</a></h2><div class="meta">Last checked: %s</div></div><span class="tag">%s</span></article>`, esc(r.Type), esc(r.Status), esc(r.Date), esc(r.DateLabel), esc(r.Status), esc(r.Artist), esc(r.URL), esc(r.Title), esc(r.CheckedAt), esc(r.Type))
	}
	if len(p.Issues) > 0 {
		fmt.Fprintf(&issues, `<details><summary>%d artists need attention</summary><ul>`, len(p.Issues))
		for _, i := range p.Issues {
			fmt.Fprintf(&issues, `<li><strong>%s</strong> · Navidrome ID <code>%s</code><br>%s</li>`, esc(i.Name), esc(i.ID), esc(i.Message))
		}
		issues.WriteString("</ul></details>")
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
	// NewReplacer makes a single pass: inserted metadata is never interpreted as a template.
	result := strings.NewReplacer("@@ICS@@", base64.StdEncoding.EncodeToString(CalendarICS(Events(s, c, c.EventNamespace(), now, true), now)), "@@CALENDAR_STATS@@", esc(CalendarStats(s, c, c.EventNamespace(), now)), "@@TODAY@@", now.UTC().Format("2006-01-02"), "@@COUNT@@", fmt.Sprint(len(p.Rows)), "@@COMPLETE@@", fmt.Sprint(p.Complete), "@@ARTISTS@@", fmt.Sprint(p.Artists), "@@PENDING@@", fmt.Sprint(p.Pending), "@@PAST@@", fmt.Sprint(p.PastDays), "@@AHEAD@@", fmt.Sprint(p.AheadDays), "@@GENERATED@@", esc(p.Generated), "@@PROBLEM@@", problem, "@@ROWS@@", rows.String(), "@@ISSUES@@", issues.String(), "@@EMPTYHIDDEN@@", hidden).Replace(dashboard)
	return []byte(result), nil
}
