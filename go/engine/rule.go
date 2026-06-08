package engine

import "regexp"

type Cond struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

type Act struct {
	Key      string `json:"key"`
	Template string `json:"template"`
}

type Rule struct {
	ID      string
	Name    string
	Enabled bool
	Conds   []Cond
	Acts    []Act
}

var tokenRe = regexp.MustCompile(`\{[^}]+\}`)

func RuleProviders(conds []Cond, acts []Act) []string {
	seen := map[string]bool{}
	var out []string
	add := func(key string) {
		p, _ := splitKey(key)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, c := range conds {
		add(c.Field)
	}
	for _, a := range acts {
		for _, m := range tokenRe.FindAllString(a.Template, -1) {
			add(m[1 : len(m)-1])
		}
	}
	return out
}
