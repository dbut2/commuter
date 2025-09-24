package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dbut.dev/commute/src/strava-api"
	"github.com/antihax/optional"
)

type StravaWebhook struct {
	AspectType     string `json:"aspect_type"`
	EventTime      int64  `json:"event_time"`
	ObjectID       int64  `json:"object_id"`
	ObjectType     string `json:"object_type"`
	OwnerID        int64  `json:"owner_id"`
	SubscriptionID int    `json:"subscription_id"`
	Updates        struct {
		Title   string `json:"title"`
		Private bool   `json:"private"`
		Type    string `json:"type"`
	}
}

type Updater func(ctx context.Context, client *strava.APIClient, detailedActivity *strava.DetailedActivity, updatableActivity *strava.UpdatableActivity) bool

func Publicise(ctx context.Context, client *strava.APIClient, detailedActivity *strava.DetailedActivity, updatableActivity *strava.UpdatableActivity) bool {
	wasHidden := detailedActivity.HideFromHome
	hidden := shouldBeHidden(detailedActivity)
	fmt.Printf("Activity hidden: %t\n", hidden)
	updatableActivity.HideFromHome = hidden
	return wasHidden != hidden
}

func shouldBeHidden(activity *strava.DetailedActivity) bool {
	if *activity.SportType != strava.RIDE_SportType {
		fmt.Println("Activity not a ride, public", *activity.SportType)
		return false
	}

	if activity.StartLatlng == nil || activity.EndLatlng == nil {
		fmt.Println("Activity doesn't have start and end coords, cannot ascertain if commute, public")
		return false
	}

	if (isNear(activity.StartLatlng, home) && isNear(activity.EndLatlng, work)) ||
		(isNear(activity.StartLatlng, work) && isNear(activity.EndLatlng, home)) {
		fmt.Println("Activity is near home and work, private")
		return true
	}

	return false
}

func isNear(latLng *strava.LatLng, point [2]float64) bool {
	lat1, lat2 := latLng[0], point[0]
	lng1, lng2 := latLng[1], point[1]

	return (lat1 >= lat2-margin && lat1 <= lat2+margin) && (lng1 >= lng2-margin && lng1 <= lng2+margin)
}

var (
	home = [2]float64{0, 0}
	work = [2]float64{0, 0}
)

var margin = 0.005

func updatable(activity *strava.DetailedActivity) *strava.UpdatableActivity {
	return &strava.UpdatableActivity{
		Commute:      activity.Commute,
		Trainer:      activity.Trainer,
		HideFromHome: activity.HideFromHome,
		Description:  activity.Description,
		Name:         activity.Name,
		Type_:        activity.Type_,
		SportType:    activity.SportType,
		GearId:       activity.GearId,
	}
}

var melbourne = must(time.LoadLocation("Australia/Melbourne"))

func must[T any](v T, err error) T {
	if err != nil {
		panic(err.Error())
	}
	return v
}

func Nedds(ctx context.Context, client *strava.APIClient, detailedActivity *strava.DetailedActivity, updatableActivity *strava.UpdatableActivity) bool {
	if !isNeddsSport(*detailedActivity.SportType) {
		fmt.Println("Not a run or walk", *detailedActivity.SportType)
		fmt.Println("Not Nedds")
		return false
	}

	if !isNeddsDates(detailedActivity.StartDate) {
		fmt.Println("Outside dates", detailedActivity.StartDate.In(melbourne))
		fmt.Println("Not Nedds")
		return false
	}

	fmt.Println("Is Nedds")

	list, _, err := client.ActivitiesApi.GetLoggedInAthleteActivities(ctx, &strava.ActivitiesApiGetLoggedInAthleteActivitiesOpts{
		After:   optional.NewInt32(int32(neddsStartDate.Unix())),
		Before:  optional.NewInt32(int32(neddsEndDate.Unix())),
		PerPage: optional.NewInt32(100),
	})
	if err != nil {
		panic(err.Error())
	}

	totalTime := time.Duration(0)
	totalKms := float64(0)

	for _, item := range list {
		if !isNeddsDates(item.StartDate) {
			continue
		}

		if !isNeddsSport(*item.SportType) {
			continue
		}

		if item.StartDate.After(detailedActivity.StartDate) {
			continue
		}

		totalTime += time.Second * time.Duration(item.MovingTime)
		totalKms += float64(item.Distance) / 1000
	}

	var day = int(detailedActivity.StartDate.Sub(neddsStartDate).Hours()/24) + 1

	desc := fmt.Sprintf(strings.TrimSpace(neddsTemplate),
		day,
		totalKms,
		(totalKms/160.934)*100,
		duration(totalTime),
	)

	if detailedActivity.Description == desc {
		fmt.Println("Description already updated")
		return false
	}

	updatableActivity.Description = desc
	return true
}

func duration(d time.Duration) string {
	hours := int(d.Hours())
	d -= time.Duration(hours) * time.Hour
	minutes := int(d.Minutes())
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d.Seconds())

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

var neddsTemplate = `
Nedd's Uncomfortable Challenge: Day %d/10
Distance: %.1fkm/160.9km (%.1f%%)
Total Time: %s
`

var (
	neddsStartDate = time.Date(2024, time.October, 20, 0, 0, 0, 0, melbourne)
	neddsEndDate   = time.Date(2024, time.October, 29, 23, 59, 59, 1e9-1, melbourne)
)

func isNeddsSport(sport strava.SportType) bool {
	return sport == strava.RUN_SportType || sport == strava.WALK_SportType
}

func isNeddsDates(date time.Time) bool {
	return date.After(neddsStartDate) && date.Before(neddsEndDate)
}
