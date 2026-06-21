package core

import (
	"context"
	"time"
)

type Activity struct {
	Name        string
	Description string
	Type        string
	Commute     bool
	Hidden      bool

	// Not modifiable
	ID               int
	Time             time.Time
	Distance         float64
	MovingDuration   time.Duration
	ElapsedDuration  time.Duration
	StartLoc, EndLoc [2]float64
}

type Provider interface {
	Name() string
	Data() []Field
}

type Field struct {
	Name    string
	Example string
	Type    DataType
	Fetch   fetcher
}

type DataValue struct {
	State DataState
	Val   any
}

type DataState string

const (
	DataStateUnknown  DataState = "unknown"
	DataStateSupplied DataState = "supplied"
	DataStatePending  DataState = "pending"
	DataStateError    DataState = "error"
)

func DataSupplied(v any) DataValue {
	return DataValue{State: DataStateSupplied, Val: v}
}

func DataEmpty() DataValue {
	return DataValue{State: DataStateSupplied, Val: nil}
}

func DataPending() DataValue {
	return DataValue{State: DataStatePending, Val: nil}
}

func DataError(err error) DataValue {
	return DataValue{State: DataStateError, Val: err}
}

type fetcher func(ctx context.Context, a Activity) DataValue
