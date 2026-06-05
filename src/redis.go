package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

type redisClient struct {
	client *redis.Client
}

func (r *redisClient) GetToken(ctx context.Context, client oauth2.Config, stravaUser int64) (*oauth2.Token, error) {
	key := fmt.Sprintf("strava:%d", stravaUser)
	bytes, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	token := &oauth2.Token{}
	err = json.Unmarshal(bytes, token)
	if err != nil {
		return nil, err
	}
	if !token.Valid() {
		token, err = client.TokenSource(ctx, token).Token()
		if err != nil {
			return nil, err
		}
		err = r.StoreToken(ctx, stravaUser, token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

func (r *redisClient) StoreToken(ctx context.Context, stravaUser int64, token *oauth2.Token) error {
	bytes, err := json.Marshal(token)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("strava:%d", stravaUser)
	return r.client.Set(ctx, key, bytes, 0).Err()
}

func pendingKey(typ string, id int64) string {
	return fmt.Sprintf("pending:%s:%d", typ, id)
}

func (r *redisClient) Enqueue(ctx context.Context, p Pending) error {
	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, pendingKey(p.Type, p.ActivityID), bytes, time.Until(p.Deadline)).Err()
}

func (r *redisClient) ListPending(ctx context.Context) ([]Pending, error) {
	var pending []Pending
	iter := r.client.Scan(ctx, 0, "pending:*", 0).Iterator()
	for iter.Next(ctx) {
		bytes, err := r.client.Get(ctx, iter.Val()).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var p Pending
		if err := json.Unmarshal(bytes, &p); err != nil {
			continue
		}
		pending = append(pending, p)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return pending, nil
}

func (r *redisClient) RemovePending(ctx context.Context, typ string, id int64) error {
	return r.client.Del(ctx, pendingKey(typ, id)).Err()
}
