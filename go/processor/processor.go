package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/oauth2"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/engine"
	"dbut.dev/commuter/go/provider"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
)

type Processor struct {
	repo          store.Repo
	auth          *strava.Auth
	stravaFactory func(ctx context.Context, tok *oauth2.Token) strava.StravaAPI
	pollMu        sync.Mutex
}

func New(repo store.Repo, auth *strava.Auth) *Processor {
	p := &Processor{repo: repo, auth: auth}
	p.stravaFactory = func(ctx context.Context, tok *oauth2.Token) strava.StravaAPI {
		return strava.NewClient(auth.HTTPClient(ctx, tok))
	}
	return p
}

func (p *Processor) stravaAPIFor(ctx context.Context, userID string) (strava.StravaAPI, error) {
	raw, err := p.repo.GetStravaToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("no strava token for user")
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, err
	}
	return p.stravaFactory(ctx, &tok), nil
}

func (p *Processor) process(ctx context.Context, userID string, stravaID int64) (activityID string, pending bool, err error) {
	api, err := p.stravaAPIFor(ctx, userID)
	if err != nil {
		return "", false, err
	}
	act, err := api.GetActivity(ctx, stravaID)
	if err != nil {
		return "", false, err
	}
	base := act

	userRules, err := p.repo.Rules(ctx, userID)
	if err != nil {
		return "", false, err
	}
	providers, err := p.userProviders(ctx, userID)
	if err != nil {
		return "", false, err
	}
	eng := engine.New(providers...)
	run := eng.NewRun()

	var applied []string
	applied, pending, err = runRules(ctx, run, userRules, providerSet(providers), &act)
	if err != nil {
		return "", false, err
	}

	if upd := diffUpdate(base, act); !upd.Empty() {
		if err := api.UpdateActivity(ctx, stravaID, upd); err != nil {
			return "", false, err
		}
	}

	status := "processed"
	if pending {
		status = "pending"
	}
	activityID, err = p.repo.UpsertActivity(ctx, userID, stravaID, status, applied, act.Time)
	if err != nil {
		return "", false, err
	}

	if err := p.cacheProviderData(ctx, activityID, act, run); err != nil {
		return "", false, err
	}
	return activityID, pending, nil
}

func runRules(ctx context.Context, run *engine.Run, rules []*engine.Rule, available map[string]bool, act *core.Activity) (applied []string, pending bool, err error) {
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !providersAvailable(engine.RuleProviders(r.Conds, r.Acts), available) {
			continue
		}
		ev, err := run.Evaluate(ctx, *r, *act)
		if err != nil {
			return nil, false, err
		}
		if ev.Pending {
			pending = true
			continue
		}
		if !ev.Matched {
			continue
		}
		ap, err := run.Apply(ctx, *r, act)
		if err != nil {
			return nil, false, err
		}
		if ap.Pending {
			pending = true
			continue
		}
		applied = append(applied, r.Name)
	}
	return applied, pending, nil
}

func providerSet(providers []core.Provider) map[string]bool {
	m := map[string]bool{}
	for _, prov := range providers {
		m[prov.Name()] = true
	}
	return m
}

func (p *Processor) cacheProviderData(ctx context.Context, activityID string, act core.Activity, run *engine.Run) error {
	if err := p.repo.SetProviderData(ctx, activityID, "strava", stravaBlob(ctx, act), true); err != nil {
		return err
	}
	for prov, pv := range run.Fetched() {
		if prov == "strava" {
			continue
		}
		data, err := json.Marshal(pv.Values)
		if err != nil {
			return err
		}
		if err := p.repo.SetProviderData(ctx, activityID, prov, data, pv.Found); err != nil {
			return err
		}
	}
	return nil
}

func diffUpdate(base, act core.Activity) strava.ActivityUpdate {
	var u strava.ActivityUpdate
	if act.Name != base.Name {
		n := act.Name
		u.Name = &n
	}
	if act.Description != base.Description {
		d := act.Description
		u.Description = &d
	}
	if act.Commute != base.Commute {
		c := act.Commute
		u.Commute = &c
	}
	if act.Hidden != base.Hidden {
		h := act.Hidden
		u.HideFromHome = &h
	}
	return u
}

func stravaBlob(ctx context.Context, act core.Activity) []byte {
	m := map[string]string{}
	for _, f := range (provider.Strava{}).Data() {
		v, found, err := f.Fetch(ctx, act)
		if err != nil || !found {
			continue
		}
		m[f.Name] = engine.Format(v)
	}
	b, _ := json.Marshal(m)
	return b
}
