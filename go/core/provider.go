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
	SegmentEfforts   []SegmentEffort
}

type SegmentEffort struct {
	SegmentID int64
	Name      string
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

type fetcher func(ctx context.Context, a Activity) (any, bool, error)
