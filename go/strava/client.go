package strava

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
	api *api.ClientWithResponses
}

func NewClient(httpClient *http.Client) StravaAPI {
	return &stravaClient{api: &api.ClientWithResponses{
		ClientInterface: &api.Client{Server: "https://www.strava.com/api/v3/", Client: httpClient},
	}}
}

func (c *stravaClient) GetActivity(ctx context.Context, id int64) (core.Activity, error) {
	resp, err := c.api.GetActivityByIdWithResponse(ctx, id, &api.GetActivityByIdParams{IncludeAllEfforts: new(true)})
	if err != nil {
		return core.Activity{}, err
	}
	if resp.JSON200 == nil {
		return core.Activity{}, fmt.Errorf("get activity %d: %s", id, resp.Status())
	}
	return toCore(resp.JSON200), nil
}

func (c *stravaClient) ListActivities(ctx context.Context, perPage int) ([]int64, error) {
	resp, err := c.api.GetLoggedInAthleteActivitiesWithResponse(ctx, &api.GetLoggedInAthleteActivitiesParams{PerPage: &perPage})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("list activities: %s", resp.Status())
	}
	ids := make([]int64, len(*resp.JSON200))
	for i, a := range *resp.JSON200 {
		ids[i] = deref(a.Id)
	}
	return ids, nil
}

func (c *stravaClient) UpdateActivity(ctx context.Context, id int64, upd ActivityUpdate) error {
	if upd.Empty() {
		return nil
	}
	resp, err := c.api.UpdateActivityByIdWithResponse(ctx, id, api.UpdatableActivity{
		Name:         upd.Name,
		Description:  upd.Description,
		Commute:      upd.Commute,
		HideFromHome: upd.HideFromHome,
	})
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("update activity %d: %s", id, resp.Status())
	}
	return nil
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func toCore(a *api.DetailedActivity) core.Activity {
	loc := func(p *api.LatLng) [2]float64 {
		if p == nil || len(*p) != 2 {
			return [2]float64{}
		}
		return [2]float64{float64((*p)[0]), float64((*p)[1])}
	}
	typ := string(deref(a.SportType))
	if typ == "" {
		typ = string(deref(a.Type))
	}
	var efforts []core.SegmentEffort
	for _, se := range deref(a.SegmentEfforts) {
		if se.Segment == nil {
			continue
		}
		efforts = append(efforts, core.SegmentEffort{SegmentID: deref(se.Segment.Id), Name: deref(se.Segment.Name)})
	}
	return core.Activity{
		ID:              int(deref(a.Id)),
		Name:            deref(a.Name),
		Description:     deref(a.Description),
		Type:            typ,
		Commute:         deref(a.Commute),
		Hidden:          deref(a.HideFromHome),
		Time:            deref(a.StartDateLocal),
		Distance:        float64(deref(a.Distance)),
		MovingDuration:  time.Duration(deref(a.MovingTime)) * time.Second,
		ElapsedDuration: time.Duration(deref(a.ElapsedTime)) * time.Second,
		StartLoc:        loc(a.StartLatlng),
		EndLoc:          loc(a.EndLatlng),
		SegmentEfforts:  efforts,
	}
}
