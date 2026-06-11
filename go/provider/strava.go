package provider

import (
	"context"
	"math/big"
	"time"

	"dbut.dev/commuter/go/client/strava"
	"dbut.dev/commuter/go/core"
)

var _ core.Provider = Strava{}

type Strava struct{}

func (s Strava) Name() string { return "strava" }

func (s Strava) Data() []core.Field { return stravaData }

var weekdays = []string{
	time.Monday.String(), time.Tuesday.String(), time.Wednesday.String(),
	time.Thursday.String(), time.Friday.String(), time.Saturday.String(),
	time.Sunday.String(),
}

var sportTypes = []string{
	string(strava.HIKE_SportType),
	string(strava.RIDE_SportType),
	string(strava.RUN_SportType),
	string(strava.SWIM_SportType),
	string(strava.WALK_SportType),
	string(strava.WORKOUT_SportType),
}

var stravaData = []core.Field{
	field("activity_name", "Morning Run", core.TypeString, func(a core.Activity) string { return a.Name }),
	field("activity_description", "Here's a description", core.TypeString, func(a core.Activity) string { return a.Description }),
	field("activity_type", "Run", core.TypeEnum(sportTypes...), func(a core.Activity) string { return a.Type }),
	field("activity_is_commute", "false", core.TypeBool, func(a core.Activity) bool { return a.Commute }),
	field("activity_is_hidden", "false", core.TypeBool, func(a core.Activity) bool { return a.Hidden }),
	field("activity_time", "10/05/2026 10:25am ", core.TypeDatetime, func(a core.Activity) time.Time { return a.Time }),
	field("activity_time_of_day", "10:25am", core.TypeTime, func(a core.Activity) time.Time { return a.Time }),
	field("activity_date", "10/05/2026", core.TypeDate, func(a core.Activity) time.Time { return a.Time }),
	field("activity_day_of_week", "Sunday", core.TypeEnum(weekdays...), func(a core.Activity) string { return a.Time.Weekday().String() }),
	field("activity_distance", "10.0", core.TypeNumber, func(a core.Activity) *big.Rat { return new(big.Rat).SetFloat64(a.Distance / 1000) }),
	field("activity_moving_duration", "25:49", core.TypeDuration, func(a core.Activity) time.Duration { return a.MovingDuration }),
	field("activity_elapsed_duration", "25:51", core.TypeDuration, func(a core.Activity) time.Duration { return a.ElapsedDuration }),
	field("activity_start_location", "[0.0, 0.0]", core.TypeCoords, func(a core.Activity) [2]float64 { return a.StartLoc }),
	field("activity_end_location", "[0.0, 0.0]", core.TypeCoords, func(a core.Activity) [2]float64 { return a.EndLoc }),
}

func field[T any](name string, example string, t core.Type[T], get func(a core.Activity) T) core.Field {
	return core.Field{Name: name, Example: example, Type: t.DataType, Fetch: func(ctx context.Context, a core.Activity) (any, bool, error) {
		return get(a), true, nil
	}}
}
