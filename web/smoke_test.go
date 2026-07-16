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
			[]string{"Connect Strava", "daisyui", "data-theme=\"light\""},
		},
		"dashboard": {
			func(w *strings.Builder) error {
				return DashboardPage(Dashboard{
					Filter:       "pending",
					PendingCount: 2,
					Cards:        []ActivityCard{{ID: "a1", Name: "Morning Ride", Meta: "meta", Status: "processed", Rules: []string{"commute"}}},
				}, "DB").Render(context.Background(), w)
			},
			[]string{"Morning Ride", "badge-success", "tab-active", "htmx.org", "btm-nav"},
		},
		"rules": {
			func(w *strings.Builder) error {
				return RulesPage([]RuleRow{{ID: "r1", Name: "parkrun", When: "w", Then: "t", Enabled: true}}, "DB").Render(context.Background(), w)
			},
			[]string{"parkrun", "hx-post=\"/rules/toggle?id=r1\"", "toggle-success"},
		},
		"editor": {
			func(w *strings.Builder) error {
				return EditorPage(Editor{
					Name:    "parkrun title",
					Enabled: true,
					Conds:   []CondRowVM{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
					Acts:    []ActRowVM{{Key: "title", Template: "x"}},
					Catalog: catalog,
				}, "DB").Render(context.Background(), w)
			},
			[]string{"cond-row", "hx-include=\"closest .cond-row\"", "optgroup label=\"strava\"", "data-token=\"{vars.home}\"", "template-input", "datalist id=\"var-tokens\""},
		},
		"activity": {
			func(w *strings.Builder) error {
				return ActivityDetailPage(ActivityPage{
					ID: "a1", StravaID: 42, Name: "Ride", Status: "failed", JobStatus: "failed", JobError: "boom",
					RunLog:    []RunRow{{Rule: "commute", Status: "applied"}},
					Providers: []ProviderData{{Provider: "strava", Found: true, Data: [][2]string{{"k", "v"}}}},
				}, "DB").Render(context.Background(), w)
			},
			[]string{"strava.com/activities/42", "alert-error", "commute", "badge-error"},
		},
		"vars": {
			func(w *strings.Builder) error {
				return VarsPage([]VarRow{{Name: "home", Type: "coords", Value: "1,2", Used: "1 rule"}}, "DB").Render(context.Background(), w)
			},
			[]string{"form=\"var-add\"", "form=\"var-del\"", "home"},
		},
		"settings": {
			func(w *strings.Builder) error {
				return SettingsPage("A123", []SegmentRow{{Name: "albert_park", SegmentID: "6408668"}}, true, "DB").Render(context.Background(), w)
			},
			[]string{"parkrun_id", "alert-success", "Delete account", "form=\"segment-add\"", "form=\"segment-del\"", "albert_park"},
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
