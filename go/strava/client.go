package strava

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dbut.dev/commuter/go/core"
)

const stravaAPIBase = "https://www.strava.com/api/v3"

type StravaAPI interface {
	GetActivity(ctx context.Context, id int64) (core.Activity, error)
	ListActivities(ctx context.Context, perPage int) ([]int64, error)
	UpdateActivity(ctx context.Context, id int64, upd ActivityUpdate) error
}

type ActivityUpdate struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Commute      *bool   `json:"commute,omitempty"`
	HideFromHome *bool   `json:"hide_from_home,omitempty"`
}

func (u ActivityUpdate) Empty() bool {
	return u.Name == nil && u.Description == nil && u.Commute == nil && u.HideFromHome == nil
}

type stravaClient struct {
	http *http.Client
}

func NewClient(httpClient *http.Client) StravaAPI { return &stravaClient{http: httpClient} }

type stravaActivity struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	SportType      string    `json:"sport_type"`
	Type           string    `json:"type"`
	Commute        bool      `json:"commute"`
	HideFromHome   bool      `json:"hide_from_home"`
	StartDateLocal time.Time `json:"start_date_local"`
	Distance       float64   `json:"distance"`
	MovingTime     int       `json:"moving_time"`
	ElapsedTime    int       `json:"elapsed_time"`
	StartLatLng    []float64 `json:"start_latlng"`
	EndLatLng      []float64 `json:"end_latlng"`
}

func (a stravaActivity) toCore() core.Activity {
	loc := func(p []float64) [2]float64 {
		if len(p) >= 2 {
			return [2]float64{p[0], p[1]}
		}
		return [2]float64{}
	}
	typ := a.SportType
	if typ == "" {
		typ = a.Type
	}
	return core.Activity{
		ID:              int(a.ID),
		Name:            a.Name,
		Description:     a.Description,
		Type:            typ,
		Commute:         a.Commute,
		Hidden:          a.HideFromHome,
		Time:            a.StartDateLocal,
		Distance:        a.Distance,
		MovingDuration:  time.Duration(a.MovingTime) * time.Second,
		ElapsedDuration: time.Duration(a.ElapsedTime) * time.Second,
		StartLoc:        loc(a.StartLatLng),
		EndLoc:          loc(a.EndLatLng),
	}
}

func (c *stravaClient) GetActivity(ctx context.Context, id int64) (core.Activity, error) {
	var sa stravaActivity
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/activities/%d", id), nil, &sa); err != nil {
		return core.Activity{}, err
	}
	return sa.toCore(), nil
}

func (c *stravaClient) ListActivities(ctx context.Context, perPage int) ([]int64, error) {
	var list []stravaActivity
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/athlete/activities?per_page=%d", perPage), nil, &list); err != nil {
		return nil, err
	}
	ids := make([]int64, len(list))
	for i, a := range list {
		ids[i] = a.ID
	}
	return ids, nil
}

func (c *stravaClient) UpdateActivity(ctx context.Context, id int64, upd ActivityUpdate) error {
	if upd.Empty() {
		return nil
	}
	body, err := json.Marshal(upd)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/activities/%d", id), body, nil)
}

func (c *stravaClient) do(ctx context.Context, method, path string, body []byte, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, stravaAPIBase+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("strava %s %s: %d: %s", method, path, resp.StatusCode, b)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
