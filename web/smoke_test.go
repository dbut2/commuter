package web

import (
	"context"
	"strings"
	"testing"
)

func TestPagesRender(t *testing.T) {
	catalog := []Provider{
		{Name: "strava", Fields: []Field{{Key: "activity_type", Kind: "enum", Values: []string{"Ride", "Run"}, Ops: []Op{{Name: "eq", Label: "is"}}}}},
		{Name: "vars", Note: "your variables (Variables page)", Fields: []Field{{Key: "home", Kind: "coords", Example: "-37.6, 144.5"}}},
	}
	pages := map[string]struct {
		render func(w *strings.Builder) error
		want   []string
	}{
		"login": {
			func(w *strings.Builder) error { return LoginPage().Render(context.Background(), w) },
			[]string{"Connect with Strava", "Powered by Strava", "/static/app.css"},
		},
		"dashboard": {
			func(w *strings.Builder) error {
				return DashboardPage(Dashboard{
					Filter:       "pending",
					PendingCount: 2,
					Cards:        []ActivityCard{{ID: "a1", Name: "Morning Ride", Meta: "meta", Status: "processed", Rules: []string{"commute"}}},
				}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"Morning Ride", "led processed", "aria-current=\"page\"", "htmx.org", "2 pending"},
		},
		"rules": {
			func(w *strings.Builder) error {
				return RulesPage([]RuleRow{{ID: "r1", Name: "parkrun", When: "w", Then: "t", Enabled: true}}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"parkrun", "hx-post=\"/rules/toggle?id=r1\"", "checked"},
		},
		"editor": {
			func(w *strings.Builder) error {
				return EditorPage(Editor{
					Name:    "parkrun title",
					Enabled: true,
					Conds:   []CondRowVM{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
					Acts:    []ActRowVM{{Key: "title", Template: "x"}},
					Catalog: catalog,
				}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"cond-row", "hx-include=\"closest .cond-row\"", "optgroup label=\"strava\"", "data-token=\"{vars.home}\"", "template-input", "datalist id=\"var-tokens\""},
		},
		"activity": {
			func(w *strings.Builder) error {
				return ActivityDetailPage(ActivityPage{
					ID: "a1", StravaID: 42, Name: "Ride", Status: "failed", JobStatus: "failed", JobError: "boom",
					RunLog:    []RunRow{{Rule: "commute", Status: "applied"}},
					Providers: []ProviderData{{Provider: "strava", Found: true, Data: [][2]string{{"k", "v"}}}},
				}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"strava.com/activities/42", "led failed", "commute", "boom"},
		},
		"activity retrying": {
			func(w *strings.Builder) error {
				return ActivityDetailPage(ActivityPage{
					ID: "a1", StravaID: 42, Name: "Ride", Status: "unprocessed", JobStatus: "retrying", JobError: "strava 502", NextRun: "10:15",
				}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"led unprocessed", "strava 502", "10:15"},
		},
		"privacy": {
			func(w *strings.Builder) error { return PrivacyPage().Render(context.Background(), w) },
			[]string{"what's stored", "where they start and finish", "Lookup services", "revoking access"},
		},
		"vars": {
			func(w *strings.Builder) error {
				return VarsPage([]VarRow{{Name: "home", Type: "coords", Value: "1,2", Used: "1 rule"}}, Nav{Active: "activities", Pending: 2}).Render(context.Background(), w)
			},
			[]string{"form=\"var-add\"", "form=\"var-del\"", "home"},
		},
		"settings": {
			func(w *strings.Builder) error {
				return SettingsPage("A123", []SegmentRow{{Name: "albert_park", SegmentID: "6408668"}}, true, Nav{Parkrun: true}).Render(context.Background(), w)
			},
			[]string{"parkrun_id", "saved", "delete account", "form=\"segment-add\"", "form=\"segment-del\"", "albert_park", "parkrun linked"},
		},
	}
	for name, p := range pages {
		var w strings.Builder
		if err := p.render(&w); err != nil {
			t.Fatalf("%s: render: %v", name, err)
		}
		html := w.String()
		for _, want := range p.want {
			if !strings.Contains(html, want) {
				t.Errorf("%s: output missing %q", name, want)
			}
		}
	}
}
