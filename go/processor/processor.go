package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

const syncBatch = 30

func (p *Processor) Sync(ctx context.Context, userID string) (int, error) {
	api, err := p.stravaAPIFor(ctx, userID)
	if err != nil {
		return 0, err
	}
	ids, err := api.ListActivities(ctx, syncBatch)
	if err != nil {
		return 0, err
	}
	existing, err := p.repo.ExistingStravaIDs(ctx, userID, ids)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, id := range ids {
		if existing[id] {
			continue
		}
		if err := p.Enqueue(ctx, userID, id); err != nil {
			return 0, err
		}
		added++
	}
	if added > 0 {
		p.KickPoll()
	}
	return added, nil
}

func (p *Processor) RefreshStravaData(ctx context.Context, userID string, stravaID int64) (bool, error) {
	activityID, err := p.repo.ActivityIDByStrava(ctx, userID, stravaID)
	if err != nil || activityID == "" {
		return false, err
	}
	api, err := p.stravaAPIFor(ctx, userID)
	if err != nil {
		return true, err
	}
	act, err := api.GetActivity(ctx, stravaID)
	if err != nil {
		return true, err
	}
	if err := p.repo.SetProviderData(ctx, activityID, "strava", stravaBlob(ctx, act), true); err != nil {
		return true, err
	}
	return true, p.repo.SetActivityTime(ctx, activityID, act.Time)
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
	var runLog []store.RunEntry
	applied, runLog, pending, err = runRules(ctx, run, userRules, providerSet(providers), &act)
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
	activityID, err = p.repo.UpsertActivity(ctx, userID, stravaID, status, applied, act.Time, runLog)
	if err != nil {
		return "", false, err
	}

	if err := p.cacheProviderData(ctx, activityID, act, run, providers); err != nil {
		return "", false, err
	}
	return activityID, pending, nil
}

func runRules(ctx context.Context, run *engine.Run, rules []*engine.Rule, available map[string]bool, act *core.Activity) (applied []string, runLog []store.RunEntry, pending bool, err error) {
	logEntry := func(r *engine.Rule, status, note string) {
		runLog = append(runLog, store.RunEntry{Rule: r.Name, Status: status, Note: note})
	}
	for _, r := range rules {
		if !r.Enabled {
			logEntry(r, "disabled", "")
			continue
		}
		if missing := missingProviders(engine.RuleProviders(r.Conds, r.Acts), available); len(missing) > 0 {
			logEntry(r, "skipped", "needs "+strings.Join(missing, ", "))
			continue
		}
		ev, err := run.Evaluate(ctx, *r, *act)
		if err != nil {
			return nil, nil, false, err
		}
		if ev.Pending {
			pending = true
			logEntry(r, "pending", "waiting on provider data")
			continue
		}
		if !ev.Matched {
			note := ""
			if c := ev.Failed; c != nil {
				note = fmt.Sprintf("%s %s %s: no match", c.Field, c.Op, c.Value)
			}
			logEntry(r, "no_match", note)
			continue
		}
		ap, err := run.Apply(ctx, *r, act)
		if err != nil {
			return nil, nil, false, err
		}
		if ap.Pending {
			pending = true
			logEntry(r, "pending", "conditions matched, waiting on data for templates")
			continue
		}
		applied = append(applied, r.Name)
		logEntry(r, "applied", "")
	}
	return applied, runLog, pending, nil
}

func providerSet(providers []core.Provider) map[string]bool {
	m := map[string]bool{}
	for _, prov := range providers {
		m[prov.Name()] = true
	}
	return m
}

func (p *Processor) cacheProviderData(ctx context.Context, activityID string, act core.Activity, run *engine.Run, providers []core.Provider) error {
	if err := p.repo.SetProviderData(ctx, activityID, "strava", stravaBlob(ctx, act), true); err != nil {
		return err
	}
	fetched := run.Fetched()
	for _, prov := range providers {
		name := prov.Name()
		if name == "strava" || name == "vars" {
			continue
		}
		pv, touched := fetched[name]
		if !touched {
			continue
		}
		values := pv.Values
		if pv.Found {
			values = providerBlob(ctx, prov, act)
		}
		data, err := json.Marshal(values)
		if err != nil {
			return err
		}
		if err := p.repo.SetProviderData(ctx, activityID, name, data, pv.Found); err != nil {
			return err
		}
	}
	return nil
}

func providerBlob(ctx context.Context, prov core.Provider, act core.Activity) map[string]string {
	m := map[string]string{}
	for _, f := range prov.Data() {
		dv := f.Fetch(ctx, act)
		if dv.State != core.DataStateSupplied {
			continue
		}
		m[f.Name] = engine.Format(dv.Val)
	}
	return m
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
	b, _ := json.Marshal(providerBlob(ctx, provider.Strava{}, act))
	return b
}
