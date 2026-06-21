package provider

import (
	"context"
	"fmt"

	"dbut.dev/commuter/go/core"
)

var _ core.Provider = Vars{}

type VarDef struct {
	Name  string
	Kind  core.Kind
	Value string
}

type Vars struct {
	Defs []VarDef
}

func (v Vars) Name() string { return "vars" }

func (v Vars) Data() []core.Field {
	fields := make([]core.Field, 0, len(v.Defs))
	for _, d := range v.Defs {
		fields = append(fields, core.Field{
			Name:    d.Name,
			Example: d.Value,
			Type:    core.DataType{Kind: d.Kind},
			Fetch: func(ctx context.Context, a core.Activity) core.DataValue {
				val, err := core.ParseValue(d.Kind, d.Value)
				if err != nil {
					return core.DataError(fmt.Errorf("var %q: %w", d.Name, err))
				}
				return core.DataSupplied(val)
			},
		})
	}
	return fields
}
