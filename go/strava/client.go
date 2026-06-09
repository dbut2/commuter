package strava

import (
	"context"
	"net/http"
	"time"

	"github.com/antihax/optional"

	"dbut.dev/commuter/go/core"

	api "dbut.dev/commuter/go/client/strava"
)

type StravaAPI interface {
	GetActivity(ctx context.Context, id int64) (core.Activity, error)
	ListActivities(ctx context.Context, perPage int) ([]int64, error)
	UpdateActivity(ctx context.Context, id int64, upd ActivityUpdate) error
}

type ActivityUpdate struct {
	Name         *string
	Description  *string
	Commute      *bool
	HideFromHome *bool
}

func (u ActivityUpdate) Empty() bool {
	return u.Name == nil && u.Description == nil && u.Commute == nil && u.HideFromHome == nil
}

type stravaClient struct {
	api *api.APIClient
}

func NewClient(httpClient *http.Client) StravaAPI {
	cfg := api.NewConfiguration()
	cfg.HTTPClient = httpClient
	return &stravaClient{api: api.NewAPIClient(cfg)}
}

func (c *stravaClient) GetActivity(ctx context.Context, id int64) (core.Activity, error) {
	a, _, err := c.api.ActivitiesApi.GetActivityById(ctx, id, nil)
	if err != nil {
		return core.Activity{}, err
	}
	return toCore(a), nil
}

func (c *stravaClient) ListActivities(ctx context.Context, perPage int) ([]int64, error) {
	acts, _, err := c.api.ActivitiesApi.GetLoggedInAthleteActivities(ctx, &api.ActivitiesApiGetLoggedInAthleteActivitiesOpts{
		PerPage: optional.NewInt32(int32(perPage)),
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(acts))
	for i, a := range acts {
		ids[i] = a.Id
	}
	return ids, nil
}

func (c *stravaClient) UpdateActivity(ctx context.Context, id int64, upd ActivityUpdate) error {
	if upd.Empty() {
		return nil
	}
	var body api.UpdatableActivity
	if upd.Name != nil {
		body.Name = *upd.Name
	}
	if upd.Description != nil {
		body.Description = *upd.Description
	}
	if upd.Commute != nil {
		body.Commute = *upd.Commute
	}
	if upd.HideFromHome != nil {
		body.HideFromHome = *upd.HideFromHome
	}
	_, _, err := c.api.ActivitiesApi.UpdateActivityById(ctx, id, &api.ActivitiesApiUpdateActivityByIdOpts{
		Body: optional.NewInterface(body),
	})
	return err
}

func toCore(a api.DetailedActivity) core.Activity {
	loc := func(p *[2]float64) [2]float64 {
		if p != nil {
			return *p
		}
		return [2]float64{}
	}
	typ := ""
	if a.SportType != nil {
		typ = string(*a.SportType)
	} else if a.Type_ != nil {
		typ = string(*a.Type_)
	}
	return core.Activity{
		ID:              int(a.Id),
		Name:            a.Name,
		Description:     a.Description,
		Type:            typ,
		Commute:         a.Commute,
		Hidden:          a.HideFromHome,
		Time:            a.StartDateLocal,
		Distance:        float64(a.Distance),
		MovingDuration:  time.Duration(a.MovingTime) * time.Second,
		ElapsedDuration: time.Duration(a.ElapsedTime) * time.Second,
		StartLoc:        loc(a.StartLatlng),
		EndLoc:          loc(a.EndLatlng),
	}
}
