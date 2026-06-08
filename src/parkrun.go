package main

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"dbut.dev/commuter/src/parkrun"
)

const (
	parkrunTimeout = 6 * time.Hour
	parkrunMinDist = 4.5
	parkrunMaxDist = 5.5
)

type parkrunUpdater struct {
	ownerID      int64
	parkrunnerID int64
	types        []Type
}

type parkrunPayload struct {
	ParkrunnerID int64  `json:"parkrunner_id"`
	EventDate    string `json:"event_date"`
}

func NewParkrun(ownerID, parkrunnerID int64, types ...Type) parkrunUpdater {
	return parkrunUpdater{
		ownerID:      ownerID,
		parkrunnerID: parkrunnerID,
		types:        types,
	}
}

func (u parkrunUpdater) Type() string {
	return "parkrun"
}

func (u parkrunUpdater) Check(ctx context.Context, client StravaClient, activity Activity) (Pending, bool) {
	if len(u.types) > 0 && !slices.Contains(u.types, activity.Type) {
		return Pending{}, false
	}
	if !isParkrun(activity) {
		return Pending{}, false
	}

	data, err := json.Marshal(parkrunPayload{
		ParkrunnerID: u.parkrunnerID,
		EventDate:    activity.Time.Format("02/01/2006"),
	})
	if err != nil {
		return Pending{}, false
	}

	return Pending{
		Type:       u.Type(),
		OwnerID:    u.ownerID,
		ActivityID: int64(activity.ID),
		Deadline:   time.Now().Add(parkrunTimeout),
		Data:       data,
	}, true
}

func (u parkrunUpdater) Resolve(ctx context.Context, client StravaClient, data []byte) (Updater, bool) {
	var payload parkrunPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, false
	}

	result, found, err := parkrun.Lookup(ctx, parkrun.DefaultBaseURL, payload.ParkrunnerID, payload.EventDate)
	if err != nil || !found {
		return nil, false
	}

	return func(_ context.Context, _ StravaClient, activity Activity) Activity {
		activity.Name = result.Event
		return activity
	}, true
}

func isParkrun(activity Activity) bool {
	if activity.Distance < parkrunMinDist || activity.Distance > parkrunMaxDist {
		return false
	}
	if activity.Time.Weekday() != time.Saturday {
		return false
	}
	if activity.Time.Hour() >= 12 {
		return false
	}
	return true
}
