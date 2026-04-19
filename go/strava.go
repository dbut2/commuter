package main

import (
	"context"
	"fmt"
	"math"
	"slices"
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

func Commute(home, work [2]float64, distance, margin float64, types ...Type) Updater {
	return func(ctx context.Context, client StravaClient, activity Activity) Activity {
		if len(types) > 0 && !slices.Contains(types, activity.Type) {
			fmt.Println("Activity not matching type filter", activity.Type)
			return activity
		}

		commute := isCommute(activity, home, work, distance, margin)
		activity.Hidden = commute
		activity.Commute = commute
		fmt.Printf("Activity commute: %t\n", commute)
		return activity
	}
}

func isCommute(activity Activity, home, work [2]float64, maxDistance, margin float64) bool {
	if activity.Distance > maxDistance {
		return false
	}

	isNear := func(a, b [2]float64) bool {
		return distance(a, b) < margin
	}

	start, end := activity.StartLoc, activity.EndLoc

	workTrip := isNear(home, start) && isNear(work, end)
	homeTrip := isNear(work, start) && isNear(home, end)

	return workTrip || homeTrip
}

func distance(a, b [2]float64) float64 {
	x := math.Abs(a[0] - b[0])
	y := math.Abs(a[1] - b[1])
	return math.Sqrt(x*x + y*y)
}

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
