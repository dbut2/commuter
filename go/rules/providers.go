package rules

import (
	"regexp"
	"strings"
)

var tokenRe = regexp.MustCompile(`\{[^}]+\}`)

func providerOf(key string) string {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i]
	}
	return ""
}

func RuleProviders(conds []Cond, acts []Act) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, c := range conds {
		add(providerOf(c.Field))
	}
	for _, a := range acts {
		for _, m := range tokenRe.FindAllString(a.Template, -1) {
			add(providerOf(strings.Trim(m, "{}")))
		}
	}
	return out
}
