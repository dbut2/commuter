package core

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"time"
)

type Operator struct {
	Name  string
	Label string
	Multi bool

	Eval func(value, operand any) (bool, error)
}

func (d DataType) Operators() []Operator {
	return operators[d.Kind]
}

var operators = map[Kind][]Operator{
	KindString:   stringOps,
	KindEnum:     enumOps,
	KindBool:     boolOps,
	KindNumber:   orderedOps(compareNumber),
	KindDuration: orderedOps(compareDuration),
	KindDate:     orderedOps(compareTime),
	KindTime:     orderedOps(compareTime),
	KindDatetime: orderedOps(compareTime),
	KindCoords:   coordsOps,
}

var stringOps = []Operator{
	{Name: "eq", Label: "is", Eval: equal[string]},
	{Name: "neq", Label: "is not", Eval: not(equal[string])},
}

var enumOps = []Operator{
	{Name: "eq", Label: "is", Eval: equal[string]},
	{Name: "neq", Label: "is not", Eval: not(equal[string])},
	{Name: "in", Label: "is one of", Multi: true, Eval: in},
	{Name: "not_in", Label: "is not one of", Multi: true, Eval: not(in)},
}

var boolOps = []Operator{
	{Name: "eq", Label: "is", Eval: equal[bool]},
}

var coordsOps = []Operator{
	{Name: "near", Label: "is near", Eval: near},
	{Name: "not_near", Label: "is not near", Eval: not(near)},
}

func near(value, operand any) (bool, error) {
	v, err := as[[2]float64](value)
	if err != nil {
		return false, err
	}
	t, err := as[NearTarget](operand)
	if err != nil {
		return false, err
	}
	return HaversineMetres(v, t.Coords) <= t.Radius, nil
}

type compare func(value, operand any) (int, error)

func orderedOps(c compare) []Operator {
	op := func(name, label string, keep func(sign int) bool) Operator {
		return Operator{Name: name, Label: label, Eval: func(value, operand any) (bool, error) {
			sign, err := c(value, operand)
			if err != nil {
				return false, err
			}
			return keep(sign), nil
		}}
	}
	return []Operator{
		op("eq", "equals", func(s int) bool { return s == 0 }),
		op("neq", "does not equal", func(s int) bool { return s != 0 }),
		op("lt", "less than", func(s int) bool { return s < 0 }),
		op("lte", "at most", func(s int) bool { return s <= 0 }),
		op("gt", "greater than", func(s int) bool { return s > 0 }),
		op("gte", "at least", func(s int) bool { return s >= 0 }),
	}
}

func compareNumber(value, operand any) (int, error) {
	a, err := as[*big.Rat](value)
	if err != nil {
		return 0, err
	}
	b, err := toRat(operand)
	if err != nil {
		return 0, err
	}
	return a.Cmp(b), nil
}

func compareDuration(value, operand any) (int, error) {
	a, err := as[time.Duration](value)
	if err != nil {
		return 0, err
	}
	b, err := as[time.Duration](operand)
	if err != nil {
		return 0, err
	}
	return cmp.Compare(a, b), nil
}

func compareTime(value, operand any) (int, error) {
	a, err := as[time.Time](value)
	if err != nil {
		return 0, err
	}
	b, err := as[time.Time](operand)
	if err != nil {
		return 0, err
	}
	switch {
	case a.Before(b):
		return -1, nil
	case a.After(b):
		return 1, nil
	default:
		return 0, nil
	}
}

func equal[T comparable](value, operand any) (bool, error) {
	a, err := as[T](value)
	if err != nil {
		return false, err
	}
	b, err := as[T](operand)
	if err != nil {
		return false, err
	}
	return a == b, nil
}

func in(value, operand any) (bool, error) {
	v, err := as[string](value)
	if err != nil {
		return false, err
	}
	list, err := stringList(operand)
	if err != nil {
		return false, err
	}
	return slices.Contains(list, v), nil
}

func not(op func(value, operand any) (bool, error)) func(value, operand any) (bool, error) {
	return func(value, operand any) (bool, error) {
		ok, err := op(value, operand)
		return !ok, err
	}
}

func toRat(v any) (*big.Rat, error) {
	switch n := v.(type) {
	case *big.Rat:
		return n, nil
	case int:
		return new(big.Rat).SetInt64(int64(n)), nil
	case int64:
		return new(big.Rat).SetInt64(n), nil
	case float64:
		r := new(big.Rat).SetFloat64(n)
		if r == nil {
			return nil, fmt.Errorf("operator: %v is not a finite number", n)
		}
		return r, nil
	case json.Number:
		return parseRat(string(n))
	case string:
		return parseRat(n)
	default:
		return nil, fmt.Errorf("operator: expected number, got %T", v)
	}
}

func parseRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("operator: %q is not a number", s)
	}
	return r, nil
}

func as[T any](v any) (T, error) {
	t, ok := v.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("operator: expected %T, got %T", zero, v)
	}
	return t, nil
}

func stringList(v any) ([]string, error) {
	switch t := v.(type) {
	case []string:
		return t, nil
	case []any:
		out := make([]string, len(t))
		for i, e := range t {
			s, err := as[string](e)
			if err != nil {
				return nil, err
			}
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("operator: expected list, got %T", v)
	}
}
