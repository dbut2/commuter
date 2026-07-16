package provider

import (
	"context"
	"math/big"

	"dbut.dev/commuter/go/core"
)

var _ core.Provider = Segments{}

type SegmentDef struct {
	Name      string
	SegmentID int64
}

type Segments struct {
	Defs []SegmentDef
}

func (s Segments) Name() string { return "segments" }

func (s Segments) Data() []core.Field {
	fields := make([]core.Field, 0, len(s.Defs))
	for _, d := range s.Defs {
		fields = append(fields, core.Field{
			Name:    d.Name + "_count",
			Example: "3",
			Type:    core.TypeNumber.DataType,
			Fetch: func(ctx context.Context, a core.Activity) (any, bool, error) {
				n := int64(0)
				for _, e := range a.SegmentEfforts {
					if e.SegmentID == d.SegmentID {
						n++
					}
				}
				return new(big.Rat).SetInt64(n), true, nil
			},
		})
	}
	return fields
}
