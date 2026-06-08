package core

import (
	"math/big"
	"time"
)

type DataType struct {
	Kind   Kind     `json:"kind"`
	Values []string `json:"values,omitempty"`
}

type Kind string

const (
	KindString   Kind = "string"
	KindNumber   Kind = "number"
	KindDate     Kind = "date"
	KindTime     Kind = "time"
	KindDatetime Kind = "datetime"
	KindDuration Kind = "duration"
	KindEnum     Kind = "enum"
	KindBool     Kind = "bool"
	KindCoords   Kind = "coords"
)

type Type[T any] struct{ DataType }

var (
	TypeString   = Type[string]{DataType{Kind: KindString}}
	TypeNumber   = Type[*big.Rat]{DataType{Kind: KindNumber}}
	TypeDate     = Type[time.Time]{DataType{Kind: KindDate}}
	TypeTime     = Type[time.Time]{DataType{Kind: KindTime}}
	TypeDatetime = Type[time.Time]{DataType{Kind: KindDatetime}}
	TypeDuration = Type[time.Duration]{DataType{Kind: KindDuration}}
	TypeBool     = Type[bool]{DataType{Kind: KindBool}}
	TypeCoords   = Type[[2]float64]{DataType{Kind: KindCoords}}
)

func TypeEnum(values ...string) Type[string] {
	return Type[string]{DataType{Kind: KindEnum, Values: values}}
}
