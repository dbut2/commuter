package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"

	"dbut.dev/commuter/src/strava/client"
	"dbut.dev/commuter/src/strava/client/activities"
	"dbut.dev/commuter/src/strava/models"
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

type Type models.SportType

const (
	TypeUnknown = Type("")
	TypeRun     = Type(models.SportTypeRun)
	TypeWalk    = Type(models.SportTypeWalk)
	TypeRide    = Type(models.SportTypeRide)
)

func s1(s models.SportType) Type {
	return Type(s)
}

func s2(s Type) models.SportType {
	return models.SportType(s)
}

func latLng(l models.LatLng) [2]float64 {
	var loc [2]float64
	for i := 0; i < len(loc) && i < len(l); i++ {
		loc[i] = float64(l[i])
	}
	return loc
}

func ActivityFromStrava(activity models.DetailedActivity) Activity {
	return Activity{
		ID:              int(activity.ID),
		Name:            activity.Name,
		Description:     activity.Description,
		Type:            s1(activity.SportType),
		Commute:         activity.Commute,
		Hidden:          activity.HideFromHome,
		Time:            time.Time(activity.StartDate),
		Distance:        float64(activity.Distance) / 1000,
		MovingDuration:  time.Second * time.Duration(activity.MovingTime),
		ElapsedDuration: time.Second * time.Duration(activity.ElapsedTime),
		StartLoc:        latLng(activity.StartLatlng),
		EndLoc:          latLng(activity.EndLatlng),
	}
}

func ActivityFromSummary(activity models.SummaryActivity) Activity {
	return Activity{
		ID:              int(activity.ID),
		Name:            activity.Name,
		Type:            s1(activity.SportType),
		Commute:         activity.Commute,
		Hidden:          activity.HideFromHome,
		Time:            time.Time(activity.StartDate),
		Distance:        float64(activity.Distance) / 1000,
		MovingDuration:  time.Second * time.Duration(activity.MovingTime),
		ElapsedDuration: time.Second * time.Duration(activity.ElapsedTime),
		StartLoc:        latLng(activity.StartLatlng),
		EndLoc:          latLng(activity.EndLatlng),
	}
}

func (a Activity) ToUpdatable() models.UpdatableActivity {
	return models.UpdatableActivity{
		Commute:      a.Commute,
		HideFromHome: a.Hidden,
		Description:  a.Description,
		Name:         a.Name,
		SportType:    s2(a.Type),
	}
}

type stravaClient struct {
	api *client.StravaAPIV3
}

var _ StravaClient = new(stravaClient)

func (s *stravaClient) GetActivity(ctx context.Context, id int) Activity {
	params := activities.NewGetActivityByIDParams().WithContext(ctx).WithID(int64(id))
	resp, err := s.api.Activities.GetActivityByID(params, authInfo(ctx))
	if err != nil {
		panic(err.Error())
	}

	return ActivityFromStrava(*resp.Payload)
}

func (s *stravaClient) ListActivities(ctx context.Context, filter ActivityFilter) []Activity {
	params := activities.NewGetLoggedInAthleteActivitiesParams().WithContext(ctx)
	if !filter.After.IsZero() {
		after := filter.After.Unix()
		params.SetAfter(&after)
	}
	if !filter.Before.IsZero() {
		before := filter.Before.Unix()
		params.SetBefore(&before)
	}

	resp, err := s.api.Activities.GetLoggedInAthleteActivities(params, authInfo(ctx))
	if err != nil {
		panic(err.Error())
	}

	var activitiesList []Activity
	for _, r := range resp.Payload {
		activity := ActivityFromSummary(*r)
		if filter.Valid(activity) {
			activitiesList = append(activitiesList, activity)
		}
	}
	return activitiesList
}

func (s *stravaClient) UpdateActivity(ctx context.Context, activity Activity) {
	body := activity.ToUpdatable()
	params := activities.NewUpdateActivityByIDParams().WithContext(ctx).WithID(int64(activity.ID)).WithBody(&body)
	_, err := s.api.Activities.UpdateActivityByID(params, authInfo(ctx))
	if err != nil {
		panic(err.Error())
	}
}

// authInfo builds a per-request bearer-token authenticator from the access
// token stored on the context by authContext.
func authInfo(ctx context.Context) runtime.ClientAuthInfoWriter {
	token, _ := ctx.Value(accessTokenKey).(string)
	return httptransport.BearerToken(token)
}
