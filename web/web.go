package web

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type ActivityCard struct {
	ID     string
	Name   string
	Meta   string
	When   string
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
	Facts     [][2]string
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

type SegmentRow struct {
	Name      string
	SegmentID string
}

type Nav struct {
	Active   string
	Pending  int
	Failed   int
	Rules    int
	RulesOn  int
	Vars     int
	Parkrun  bool
	Segments int
}

func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func (n Nav) activityLines() []string {
	var lines []string
	if n.Pending > 0 {
		lines = append(lines, strconv.Itoa(n.Pending)+" pending")
	}
	if n.Failed > 0 {
		lines = append(lines, strconv.Itoa(n.Failed)+" failed")
	}
	if len(lines) == 0 {
		lines = append(lines, "all processed")
	}
	return lines
}

func (n Nav) ruleLines() []string {
	return []string{count(n.Rules, "rule", "rules"), strconv.Itoa(n.RulesOn) + " on"}
}

func (n Nav) varLines() []string {
	return []string{strconv.Itoa(n.Vars) + " saved"}
}

func (n Nav) settingLines() []string {
	parkrun := "no parkrun ID"
	if n.Parkrun {
		parkrun = "parkrun linked"
	}
	return []string{parkrun, count(n.Segments, "segment", "segments")}
}

func runNote(r RunRow) string {
	switch {
	case r.Note != "":
		return r.Note
	case r.Status == "no_match":
		return "no match"
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

func boolSel(v string) string {
	if v == "false" {
		return "false"
	}
	return "true"
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

func current(on bool) templ.Attributes {
	if on {
		return templ.Attributes{"aria-current": "page"}
	}
	return nil
}

func joinRules(rules []string) string { return strings.Join(rules, ", ") }
