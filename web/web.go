package web

import (
	"strconv"

	"dbut.dev/x/kit"
)

type ActivityCard struct {
	ID     string
	Name   string
	Meta   string
	Status string
	Note   string
	Rules  []string
}

type Dashboard struct {
	Filter       string
	PendingCount int
	FailedCount  int
	Cards        []ActivityCard
}

type RuleRow struct {
	ID      string
	Name    string
	When    string
	Then    string
	Enabled bool
}

type Op struct {
	Name  string
	Label string
	Multi bool
}

type Field struct {
	Key     string
	Kind    string
	Values  []string
	Example string
	Ops     []Op
}

type Provider struct {
	Name   string
	Note   string
	Fields []Field
}

type CondRowVM struct {
	Field string
	Op    string
	Value string
}

type ActRowVM struct {
	Key      string
	Template string
}

type Editor struct {
	ID      string
	Name    string
	Enabled bool
	Conds   []CondRowVM
	Acts    []ActRowVM
	Catalog []Provider
	Error   string
}

type ActOption struct {
	Key   string
	Label string
	Bool  bool
}

var ActOptions = []ActOption{
	{Key: "title", Label: "Set title"},
	{Key: "desc", Label: "Set description"},
	{Key: "commute", Label: "Mark commute", Bool: true},
	{Key: "hide", Label: "Hide from home feed", Bool: true},
}

type RunRow struct {
	Rule   string
	Status string
	Note   string
}

type ProviderData struct {
	Provider  string
	Found     bool
	FetchedAt string
	Data      [][2]string
}

type ActivityPage struct {
	ID        string
	StravaID  int64
	Name      string
	Meta      string
	Status    string
	JobStatus string
	JobError  string
	NextRun   string
	ExpiresAt string
	RunLog    []RunRow
	Providers []ProviderData
}

type VarRow struct {
	Name  string
	Type  string
	Value string
	Used  string
}

var VarTypes = []string{"string", "number", "coords", "duration", "date", "time", "datetime", "bool"}

func nav(active, initials string) kit.Nav {
	return kit.Nav{
		Brand:     "commuter",
		BrandHref: "/activities",
		Active:    active,
		Initials:  initials,
		Links: []kit.NavLink{
			{Label: "Activities", Href: "/activities", Icon: "activity"},
			{Label: "Rules", Href: "/rules", Icon: "list"},
			{Label: "Variables", Href: "/vars", Icon: "layers"},
		},
		Menu: []kit.MenuEntry{
			{Label: "Settings", Href: "/settings"},
			{Label: "Log out", PostAction: "/logout"},
		},
	}
}

func statusTone(s string) kit.Tone {
	switch s {
	case "processed":
		return kit.Success
	case "pending":
		return kit.Warning
	case "failed":
		return kit.Error
	default:
		return kit.Ghost
	}
}

func noteStyle(status string) kit.TextOpts {
	switch status {
	case "failed":
		return kit.TextOpts{XS: true, Tone: kit.Error}
	case "pending":
		return kit.TextOpts{XS: true, Tone: kit.Warning}
	}
	return kit.TextOpts{XS: true, Faint: true}
}

func runMark(status string) (mark string, tone kit.Tone) {
	switch status {
	case "applied":
		return "✓", kit.Success
	case "pending":
		return "…", kit.Warning
	default:
		return "−", ""
	}
}

func runNote(r RunRow) string {
	switch {
	case r.Note != "":
		return r.Note
	case r.Status == "disabled":
		return "disabled"
	case r.Status == "no_match":
		return "no match"
	case r.Status == "applied":
		return "applied"
	}
	return r.Status
}

func FindField(catalog []Provider, key string) (Field, bool) {
	for _, p := range catalog {
		for _, f := range p.Fields {
			if p.Name+"."+f.Key == key {
				return f, true
			}
		}
	}
	return Field{}, false
}

func findOp(f Field, name string) Op {
	for _, o := range f.Ops {
		if o.Name == name {
			return o
		}
	}
	if len(f.Ops) > 0 {
		return f.Ops[0]
	}
	return Op{}
}

func findAct(key string) ActOption {
	for _, a := range ActOptions {
		if a.Key == key {
			return a
		}
	}
	return ActOptions[0]
}

func fieldGroups(catalog []Provider) []kit.OptGroup {
	var groups []kit.OptGroup
	for _, p := range catalog {
		if len(p.Fields) == 0 {
			continue
		}
		g := kit.OptGroup{Label: p.Name}
		for _, f := range p.Fields {
			g.Options = append(g.Options, kit.Option{Value: p.Name + "." + f.Key})
		}
		groups = append(groups, g)
	}
	return groups
}

func opOptions(f Field) []kit.Option {
	var opts []kit.Option
	for _, o := range f.Ops {
		opts = append(opts, kit.Option{Value: o.Name, Label: o.Label})
	}
	return opts
}

func valueOptions(values []string) []kit.Option {
	var opts []kit.Option
	for _, v := range values {
		opts = append(opts, kit.Option{Value: v})
	}
	return opts
}

func boolOptions() []kit.Option {
	return []kit.Option{{Value: "true"}, {Value: "false"}}
}

func boolSel(v string) string {
	if v == "false" {
		return "false"
	}
	return "true"
}

func actSelOptions() []kit.Option {
	var opts []kit.Option
	for _, a := range ActOptions {
		opts = append(opts, kit.Option{Value: a.Key, Label: a.Label})
	}
	return opts
}

func typeOptions() []kit.Option {
	return valueOptions(VarTypes)
}

func varTokens(catalog []Provider) []string {
	var tokens []string
	for _, p := range catalog {
		if p.Name != "vars" {
			continue
		}
		for _, f := range p.Fields {
			tokens = append(tokens, "{vars."+f.Key+"}")
		}
	}
	return tokens
}

func stravaURL(id int64) string {
	return "https://www.strava.com/activities/" + strconv.FormatInt(id, 10)
}
