package engine

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"dbut.dev/commuter/go/core"
)

type Result struct {
	Matched bool
	Pending bool
	Failed  *Cond
}

type Engine struct {
	fields    map[string]core.Field
	providers map[string]core.Provider
}

func New(providers ...core.Provider) *Engine {
	e := &Engine{fields: map[string]core.Field{}, providers: map[string]core.Provider{}}
	for _, p := range providers {
		e.providers[p.Name()] = p
		for _, f := range p.Data() {
			e.fields[p.Name()+"."+f.Name] = f
		}
	}
	return e
}

type cache map[string]fetched

type fetched struct {
	val   any
	found bool
}

type Run struct {
	e *Engine
	c cache
}

func (e *Engine) NewRun() *Run { return &Run{e: e, c: cache{}} }

func (r *Run) Evaluate(ctx context.Context, rule Rule, a core.Activity) (Result, error) {
	return r.e.evaluate(ctx, r.c, rule, a)
}

func (r *Run) Apply(ctx context.Context, rule Rule, a *core.Activity) (Result, error) {
	return r.e.apply(ctx, r.c, rule, a)
}

type ProviderValues struct {
	Values map[string]string
	Found  bool
}

func (r *Run) Fetched() map[string]ProviderValues {
	out := map[string]ProviderValues{}
	for key, f := range r.c {
		prov, field := splitKey(key)
		pv, ok := out[prov]
		if !ok {
			pv = ProviderValues{Values: map[string]string{}}
		}
		if f.found {
			pv.Values[field] = Format(f.val)
			pv.Found = true
		}
		out[prov] = pv
	}
	return out
}

func splitKey(key string) (provider, field string) {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

func (e *Engine) value(ctx context.Context, c cache, key string, a core.Activity) (any, bool, error) {
	if f, ok := c[key]; ok {
		return f.val, f.found, nil
	}
	field, ok := e.fields[key]
	if !ok {
		return nil, false, fmt.Errorf("engine: unknown field %q", key)
	}
	val, found, err := field.Fetch(ctx, a)
	if err != nil {
		return nil, false, err
	}
	c[key] = fetched{val: val, found: found}
	return val, found, nil
}

func (e *Engine) Evaluate(ctx context.Context, rule Rule, a core.Activity) (Result, error) {
	return e.evaluate(ctx, cache{}, rule, a)
}

func (e *Engine) evaluate(ctx context.Context, c cache, rule Rule, a core.Activity) (Result, error) {
	pending := false
	for _, cond := range rule.Conds {
		matched, ready, err := e.checkCond(ctx, c, cond, a)
		if err != nil {
			return Result{}, err
		}
		if !ready {
			pending = true
			continue
		}
		if !matched {
			failed := cond
			return Result{Matched: false, Failed: &failed}, nil
		}
	}
	if pending {
		return Result{Pending: true}, nil
	}
	return Result{Matched: true}, nil
}

func (e *Engine) checkCond(ctx context.Context, c cache, cond Cond, a core.Activity) (matched, ready bool, err error) {
	field, ok := e.fields[cond.Field]
	if !ok {
		return false, false, fmt.Errorf("engine: unknown field %q", cond.Field)
	}
	val, found, err := e.value(ctx, c, cond.Field, a)
	if err != nil {
		return false, false, err
	}
	if !found {
		return false, false, nil
	}
	op := findOp(field.Type, cond.Op)
	if op == nil {
		return false, false, fmt.Errorf("engine: field %q has no operator %q", cond.Field, cond.Op)
	}
	operandStr := cond.Value
	if strings.Contains(operandStr, "{") {
		s, pend, err := e.render(ctx, c, operandStr, a)
		if err != nil {
			return false, false, err
		}
		if pend {
			return false, false, nil
		}
		operandStr = s
	}
	var operand any
	if op.Multi {
		parts := strings.Split(operandStr, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		operand = parts
	} else {
		operand, err = parseOperand(field.Type.Kind, operandStr)
		if err != nil {
			return false, false, fmt.Errorf("engine: condition %q: %w", cond.Field, err)
		}
	}
	matched, err = op.Eval(val, operand)
	return matched, true, err
}

func (e *Engine) Apply(ctx context.Context, rule Rule, a *core.Activity) (Result, error) {
	return e.apply(ctx, cache{}, rule, a)
}

func (e *Engine) apply(ctx context.Context, c cache, rule Rule, a *core.Activity) (Result, error) {
	for _, act := range rule.Acts {
		switch act.Key {
		case "commute":
			a.Commute = act.Template == "true"
		case "hide":
			a.Hidden = act.Template == "true"
		case "title", "desc":
			s, pending, err := e.render(ctx, c, act.Template, *a)
			if err != nil {
				return Result{}, err
			}
			if pending {
				return Result{Pending: true}, nil
			}
			if act.Key == "title" {
				a.Name = s
			} else {
				a.Description = s
			}
		}
	}
	return Result{Matched: true}, nil
}

func (e *Engine) render(ctx context.Context, c cache, tmpl string, a core.Activity) (string, bool, error) {
	var b strings.Builder
	pending := false
	for {
		i := strings.IndexByte(tmpl, '{')
		if i < 0 {
			b.WriteString(tmpl)
			break
		}
		j := strings.IndexByte(tmpl[i:], '}')
		if j < 0 {
			b.WriteString(tmpl)
			break
		}
		j += i
		b.WriteString(tmpl[:i])
		key := tmpl[i+1 : j]
		if _, known := e.fields[key]; !known {
			b.WriteString(tmpl[i : j+1])
			tmpl = tmpl[j+1:]
			continue
		}
		val, found, err := e.value(ctx, c, key, a)
		if err != nil {
			return "", false, err
		}
		if found {
			b.WriteString(format(val))
		} else {
			pending = true
		}
		tmpl = tmpl[j+1:]
	}
	return b.String(), pending, nil
}

func findOp(t core.DataType, name string) *core.Operator {
	for _, o := range t.Operators() {
		if o.Name == name {
			op := o
			return &op
		}
	}
	return nil
}

func parseOperand(kind core.Kind, s string) (any, error) {
	s = strings.TrimSpace(s)
	switch kind {
	case core.KindString, core.KindEnum, core.KindNumber:
		// number operators accept a string operand (parsed to *big.Rat internally)
		return s, nil
	case core.KindBool:
		return strconv.ParseBool(s)
	case core.KindDuration:
		return core.ParseDuration(s)
	case core.KindTime, core.KindDate, core.KindDatetime:
		return core.ParseTime(s)
	case core.KindCoords:
		return core.ParseNearTarget(s)
	default:
		return s, nil
	}
}

func Format(v any) string { return format(v) }

func format(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case *big.Rat:
		if x.IsInt() {
			return x.Num().String()
		}
		return strings.TrimRight(strings.TrimRight(x.FloatString(2), "0"), ".")
	case time.Duration:
		return formatDuration(x)
	case time.Time:
		return x.Format("2006-01-02 15:04")
	case [2]float64:
		return fmt.Sprintf("%g, %g", x[0], x[1])
	default:
		return fmt.Sprint(v)
	}
}

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
