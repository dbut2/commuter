package provider

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dbut.dev/commuter/go/core"
)

func TestSegmentsCount(t *testing.T) {
	p := Segments{Defs: []SegmentDef{
		{Name: "albert_park", SegmentID: 6408668},
		{Name: "yarra_blvd", SegmentID: 123},
	}}
	a := core.Activity{SegmentEfforts: []core.SegmentEffort{
		{SegmentID: 6408668, Name: "Albert Park Full Lap"},
		{SegmentID: 6408668, Name: "Albert Park Full Lap"},
		{SegmentID: 6408668, Name: "Albert Park Full Lap"},
		{SegmentID: 999, Name: "Other"},
	}}

	fields := p.Data()
	require.Len(t, fields, 2)

	want := map[string]int64{"albert_park_count": 3, "yarra_blvd_count": 0}
	for _, f := range fields {
		val, found, err := f.Fetch(context.Background(), a)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, 0, val.(*big.Rat).Cmp(new(big.Rat).SetInt64(want[f.Name])), f.Name)
	}
}

func TestStravaStoppedDuration(t *testing.T) {
	a := core.Activity{
		MovingDuration:  25*time.Minute + 49*time.Second,
		ElapsedDuration: 28*time.Minute + 3*time.Second,
	}
	for _, f := range (Strava{}).Data() {
		if f.Name != "activity_stopped_duration" {
			continue
		}
		val, found, err := f.Fetch(context.Background(), a)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, 2*time.Minute+14*time.Second, val)
		return
	}
	t.Fatal("activity_stopped_duration field not found")
}
