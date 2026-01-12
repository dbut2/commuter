package main

import (
	"context"
	"fmt"
	"time"
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

type Updater func(ctx context.Context, client StravaClient, activity Activity) Activity

func Publicise(ctx context.Context, client StravaClient, activity Activity) Activity {
	activity.Hidden = isCommute(activity)
	fmt.Printf("Activity hidden: %t\n", activity.Hidden)
	return activity
}

func Commute(ctx context.Context, client StravaClient, activity Activity) Activity {
	activity.Commute = isCommute(activity)
	fmt.Printf("Acitivty commute: %t\n", activity.Commute)
	return activity
}

func isCommute(activity Activity) bool {
	if activity.Type != TypeRide {
		fmt.Println("Activity not a ride, public", activity.Type)
		return false
	}

	if (isNear(activity.StartLoc, home) && isNear(activity.EndLoc, work)) ||
		(isNear(activity.StartLoc, work) && isNear(activity.EndLoc, home)) {
		fmt.Println("Activity is near home and work, private")
		return true
	}

	return false
}

func isNear(a, b [2]float64) bool {
	fmt.Println(a, b)

	fmt.Println(a[0] - b[0])
	fmt.Println(a[1] - b[1])

	return (a[0] >= b[0]-margin && a[0] <= b[0]+margin) && (a[1] >= b[1]-margin && a[1] <= b[1]+margin)
}

var (
	home = [2]float64{0, 0}
	work = [2]float64{0, 0}
)

var margin = 0.005

var melbourne = must(time.LoadLocation("Australia/Melbourne"))

func must[T any](v T, err error) T {
	if err != nil {
		panic(err.Error())
	}
	return v
}

var locationCache = make(map[string]*time.Location)

func location(loc string) *time.Location {
	if v, ok := locationCache[loc]; ok {
		return v
	}
	l := must(time.LoadLocation(loc))
	locationCache[loc] = l
	return l
}

func durationString(d time.Duration) string {
	hours := int(d.Hours())
	d -= time.Duration(hours) * time.Hour
	minutes := int(d.Minutes())
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d.Seconds())

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func Challenge(formatter func(day int, distance float64, duration time.Duration) string, start, end, loc string, types ...Type) Updater {
	startTime := must(time.ParseInLocation("02/01/2006", start, location(loc)))
	endTime := must(time.ParseInLocation("02/01/2006", end, location(loc))).AddDate(0, 0, 1).Add(-time.Nanosecond)
	filter := ActivityFilter{
		After:  startTime,
		Before: endTime,
		Types:  types,
	}

	fmt.Println(startTime, endTime)

	return func(ctx context.Context, client StravaClient, activity Activity) Activity {
		if !filter.Valid(activity) {
			return activity
		}

		day := int(activity.Time.Sub(startTime).Hours()/24) + 1
		distance := float64(0)
		duration := time.Duration(0)

		for _, item := range client.ListActivities(ctx, filter) {
			if !filter.Valid(item) {
				continue
			}

			distance += item.Distance
			duration += item.MovingDuration
		}

		activity.Description = formatter(day, distance, duration)
		return activity
	}
}
