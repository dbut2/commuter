package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/antihax/optional"

	"dbut.dev/commuter/src/strava"
)

type StravaClient interface {
	GetActivity(ctx context.Context, id int) Activity
	ListActivities(ctx context.Context, filter ActivityFilter) []Activity
	UpdateActivity(ctx context.Context, activity Activity)
}

type ActivityFilter struct {
	After, Before time.Time
	Types         []Type
}

func (f ActivityFilter) Valid(a Activity) bool {
	if !f.After.IsZero() && a.Time.Before(f.After) {
		fmt.Println("too early")
		return false
	}

	if !f.Before.IsZero() && a.Time.After(f.Before) {
		fmt.Println("too later")
		return false
	}

	if len(f.Types) > 0 && !slices.Contains(f.Types, a.Type) {
		fmt.Println("bad sport")
		return false
	}

	return true
}

type Activity struct {
	Name        string
	Description string
	Type        Type
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

type Type strava.SportType

const (
	TypeUnknown = Type("")
	TypeRun     = Type(strava.RUN_SportType)
	TypeWalk    = Type(strava.WALK_SportType)
	TypeRide    = Type(strava.RIDE_SportType)
)

func s1(s *strava.SportType) Type {
	if s == nil {
		return TypeUnknown
	}
	return Type(*s)
}

func s2(s Type) *strava.SportType {
	if s == TypeUnknown {
		return nil
	}
	return (*strava.SportType)(&s)
}

func ActivityFromStrava(activity strava.DetailedActivity) Activity {
	var startLoc, endLoc [2]float64
	if activity.StartLatlng != nil {
		startLoc = *activity.StartLatlng
	}
	if activity.EndLatlng != nil {
		endLoc = *activity.EndLatlng
	}

	return Activity{
		ID:              int(activity.Id),
		Name:            activity.Name,
		Description:     activity.Description,
		Type:            s1(activity.SportType),
		Commute:         activity.Commute,
		Hidden:          activity.HideFromHome,
		Time:            activity.StartDate,
		Distance:        float64(activity.Distance) / 1000,
		MovingDuration:  time.Second * time.Duration(activity.MovingTime),
		ElapsedDuration: time.Second * time.Duration(activity.ElapsedTime),
		StartLoc:        startLoc,
		EndLoc:          endLoc,
	}
}

func ActivityFromSummary(activity strava.SummaryActivity) Activity {
	var startLoc, endLoc [2]float64
	if activity.StartLatlng != nil {
		startLoc = *activity.StartLatlng
	}
	if activity.EndLatlng != nil {
		endLoc = *activity.EndLatlng
	}

	return Activity{
		ID:              int(activity.Id),
		Name:            activity.Name,
		Type:            s1(activity.SportType),
		Commute:         activity.Commute,
		Hidden:          activity.HideFromHome,
		Time:            activity.StartDate,
		Distance:        float64(activity.Distance) / 1000,
		MovingDuration:  time.Second * time.Duration(activity.MovingTime),
		ElapsedDuration: time.Second * time.Duration(activity.ElapsedTime),
		StartLoc:        startLoc,
		EndLoc:          endLoc,
	}
}

func (a Activity) ToUpdatable() strava.UpdatableActivity {
	return strava.UpdatableActivity{
		Commute:      a.Commute,
		HideFromHome: a.Hidden,
		Description:  a.Description,
		Name:         a.Name,
		SportType:    s2(a.Type),
	}
}

type stravaClient strava.APIClient

var _ StravaClient = new(stravaClient)

func (s *stravaClient) GetActivity(ctx context.Context, id int) Activity {
	resp, _, err := s.ActivitiesApi.GetActivityById(ctx, int64(id), nil)
	if err != nil {
		panic(err.Error())
	}

	return ActivityFromStrava(resp)
}

func (s *stravaClient) ListActivities(ctx context.Context, filter ActivityFilter) []Activity {
	after := optional.Int32{}
	if !filter.After.IsZero() {
		after = optional.NewInt32(int32(filter.After.Unix()))
	}

	before := optional.Int32{}
	if !filter.Before.IsZero() {
		before = optional.NewInt32(int32(filter.Before.Unix()))
	}

	resp, _, err := s.ActivitiesApi.GetLoggedInAthleteActivities(ctx, &strava.ActivitiesApiGetLoggedInAthleteActivitiesOpts{
		Before: before,
		After:  after,
	})
	if err != nil {
		panic(err.Error())
	}

	var activities []Activity
	for _, r := range resp {
		activity := ActivityFromSummary(r)
		if filter.Valid(activity) {
			activities = append(activities, ActivityFromSummary(r))
		}
	}
	return activities
}

func (s *stravaClient) UpdateActivity(ctx context.Context, activity Activity) {
	_, _, err := s.ActivitiesApi.UpdateActivityById(ctx, int64(activity.ID), &strava.ActivitiesApiUpdateActivityByIdOpts{
		Body: optional.NewInterface(activity.ToUpdatable()),
	})
	if err != nil {
		panic(err.Error())
	}
}
