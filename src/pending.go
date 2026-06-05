package main

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

type Pending struct {
	Type       string    `json:"type"`
	OwnerID    int64     `json:"owner_id"`
	ActivityID int64     `json:"activity_id"`
	Deadline   time.Time `json:"deadline"`
	Data       []byte    `json:"data"`
}

type PendingUpdater interface {
	Type() string
	Check(ctx context.Context, client StravaClient, activity Activity) (Pending, bool)
	Resolve(ctx context.Context, client StravaClient, data []byte) (Updater, bool)
}

func runWorker(ctx context.Context, rClient *redisClient, config oauth2.Config, client StravaClient, cfg Config) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		poll(ctx, rClient, config, client, cfg)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func poll(ctx context.Context, rClient *redisClient, config oauth2.Config, client StravaClient, cfg Config) {
	pending, err := rClient.ListPending(ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, p := range pending {
		resolvePending(ctx, rClient, config, client, cfg, p)
	}
}

func resolvePending(ctx context.Context, rClient *redisClient, config oauth2.Config, client StravaClient, cfg Config, p Pending) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("pending:", r)
		}
	}()

	if time.Now().After(p.Deadline) {
		_ = rClient.RemovePending(ctx, p.Type, p.ActivityID)
		return
	}

	updater, ok := cfg.PendingUpdater(p.OwnerID, p.Type)
	if !ok {
		_ = rClient.RemovePending(ctx, p.Type, p.ActivityID)
		return
	}

	update, ready := updater.Resolve(ctx, client, p.Data)
	if !ready {
		return
	}

	token, err := rClient.GetToken(ctx, config, p.OwnerID)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	ctx = authContext(ctx, token)

	activity := client.GetActivity(ctx, int(p.ActivityID))
	updated := update(ctx, client, activity)
	if updated != activity {
		client.UpdateActivity(ctx, updated)
	}
	_ = rClient.RemovePending(ctx, p.Type, p.ActivityID)
}
