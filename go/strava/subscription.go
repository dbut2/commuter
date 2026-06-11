package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const subscriptionsURL = "https://www.strava.com/api/v3/push_subscriptions"

type subscription struct {
	ID          int64  `json:"id"`
	CallbackURL string `json:"callback_url"`
}

func (a *Auth) EnsureSubscription(ctx context.Context, callbackURL, verifyToken string) error {
	subs, err := a.listSubscriptions(ctx)
	if err != nil {
		return err
	}
	for _, s := range subs {
		if s.CallbackURL == callbackURL {
			return nil
		}
		if err := a.deleteSubscription(ctx, s.ID); err != nil {
			return err
		}
	}
	return a.createSubscription(ctx, callbackURL, verifyToken)
}

func (a *Auth) deleteSubscription(ctx context.Context, id int64) error {
	q := url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
	}
	u := fmt.Sprintf("%s/%d?%s", subscriptionsURL, id, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete subscription %d: status %d", id, resp.StatusCode)
	}
	return nil
}

func (a *Auth) listSubscriptions(ctx context.Context) ([]subscription, error) {
	q := url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subscriptionsURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list subscriptions: status %d", resp.StatusCode)
	}
	var subs []subscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return nil, err
	}
	return subs, nil
}

func (a *Auth) createSubscription(ctx context.Context, callbackURL, verifyToken string) error {
	form := url.Values{
		"client_id":     {a.cfg.ClientID},
		"client_secret": {a.cfg.ClientSecret},
		"callback_url":  {callbackURL},
		"verify_token":  {verifyToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscriptionsURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create subscription: status %d", resp.StatusCode)
	}
	return nil
}
