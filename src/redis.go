package main

import (
	"context"
	"encoding/json"
	"fmt"

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
		r.StoreToken(ctx, stravaUser, token)
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
