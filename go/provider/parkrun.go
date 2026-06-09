package provider

import (
	"context"
	"math/big"
	"sync"
	"time"

	"dbut.dev/commuter/go/client/parkrun"
	"dbut.dev/commuter/go/core"
)

var _ core.Provider = Parkrun{}

type Parkrun struct {
	ParkrunID int64
}

func (p Parkrun) Name() string { return "parkrun" }

func (p Parkrun) Data() []core.Field {
	l := &lookup{provider: p, results: map[string]result{}}
	return []core.Field{
		parkrunField("event_name", "Toolern Creek parkrun", core.TypeString, l, func(d parkrun.Data) string { return d.Event }),
		parkrunField("event_number", "521", core.TypeNumber, l, func(d parkrun.Data) *big.Rat { return new(big.Rat).SetInt64(int64(d.RunNumber)) }),
		parkrunField("position", "60", core.TypeNumber, l, func(d parkrun.Data) *big.Rat { return new(big.Rat).SetInt64(int64(d.OverallPos)) }),
		parkrunField("time", "38:40", core.TypeDuration, l, func(d parkrun.Data) time.Duration { return d.Time }),
		parkrunField("age_grade", "33.36%", core.TypeNumber, l, func(d parkrun.Data) *big.Rat { return new(big.Rat).SetFloat64(d.AgeGrade) }),
		parkrunField("is_pb", "false", core.TypeBool, l, func(d parkrun.Data) bool { return d.PB }),
	}
}

type lookup struct {
	provider Parkrun
	mu       sync.Mutex
	results  map[string]result
}

type result struct {
	data  parkrun.Data
	found bool
}

func (l *lookup) get(ctx context.Context, a core.Activity) (parkrun.Data, bool, error) {
	date := a.Time.Format("02/01/2006")

	l.mu.Lock()
	defer l.mu.Unlock()

	if r, ok := l.results[date]; ok {
		return r.data, r.found, nil
	}

	data, found, err := parkrun.Lookup(ctx, parkrun.DefaultBaseURL, l.provider.ParkrunID, date)
	if err != nil {
		return parkrun.Data{}, false, err
	}

	l.results[date] = result{data: data, found: found}
	return data, found, nil
}

func parkrunField[T any](name string, example string, t core.Type[T], l *lookup, get func(parkrun.Data) T) core.Field {
	return core.Field{
		Name:    name,
		Example: example,
		Type:    t.DataType,
		Fetch: func(ctx context.Context, a core.Activity) (any, bool, error) {
			data, found, err := l.get(ctx, a)
			if err != nil || !found {
				return nil, false, err
			}
			return get(data), true, nil
		},
	}
}
