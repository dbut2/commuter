package processor

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/engine"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
)

type fakeStrava struct {
	act     core.Activity
	ids     []int64
	updated *strava.ActivityUpdate
}

func (f *fakeStrava) GetActivity(context.Context, int64) (core.Activity, error) { return f.act, nil }
func (f *fakeStrava) ListActivities(context.Context, int) ([]int64, error)      { return f.ids, nil }
func (f *fakeStrava) UpdateActivity(_ context.Context, _ int64, u strava.ActivityUpdate) error {
	f.updated = &u
	return nil
}

func testProcessor(t *testing.T) (*Processor, store.Repo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("COMMUTER_TEST_DSN")
	if dsn == "" {
		t.Skip("set COMMUTER_TEST_DSN to run processor tests")
	}
	sqlDB, err := store.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := sqlDB.Exec("truncate users, rules, activities, provider_data, jobs, strava_credentials cascade"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	repo := store.NewPGRepo(sqlDB)
	proc := New(repo, strava.NewAuth("id", "secret", "http://localhost"))
	return proc, repo, sqlDB
}

func seedUser(t *testing.T, repo store.Repo, athleteID int64) string {
	t.Helper()
	ctx := context.Background()
	uid, err := repo.UpsertUser(ctx, athleteID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStravaToken(ctx, uid, []byte(`{"access_token":"x","token_type":"Bearer"}`)); err != nil {
		t.Fatal(err)
	}
	return uid
}

func TestProcess_AppliesRuleAndPersists(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 12345)

	rule := &engine.Rule{
		Name:  "Name runs",
		Conds: []engine.Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []engine.Act{{Key: "title", Template: "Auto: {strava.activity_type}"}},
	}
	if err := repo.CreateRule(ctx, uid, rule); err != nil {
		t.Fatal(err)
	}

	fake := &fakeStrava{act: core.Activity{ID: 999, Type: "Run", Name: "old", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	_, pending, err := proc.process(ctx, uid, 999)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatalf("did not expect pending")
	}
	if fake.updated == nil || fake.updated.Name == nil || *fake.updated.Name != "Auto: Run" {
		t.Fatalf("strava update = %+v", fake.updated)
	}

	feed, err := repo.Feed(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 {
		t.Fatalf("feed len = %d", len(feed))
	}
	it := feed[0]
	if it.Status != "processed" {
		t.Fatalf("status = %q", it.Status)
	}
	if len(it.AppliedRules) != 1 || it.AppliedRules[0] != "Name runs" {
		t.Fatalf("applied = %v", it.AppliedRules)
	}
	if it.Strava["activity_name"] != "Auto: Run" {
		t.Fatalf("cached strava name = %q", it.Strava["activity_name"])
	}
}

func TestProcess_NoMatchLeavesUntouched(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 22222)
	rule := &engine.Rule{
		Name:  "Rides only",
		Conds: []engine.Cond{{Field: "strava.activity_type", Op: "eq", Value: "Ride"}},
		Acts:  []engine.Act{{Key: "commute"}},
	}
	_ = repo.CreateRule(ctx, uid, rule)

	fake := &fakeStrava{act: core.Activity{ID: 1, Type: "Run", Name: "run", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	_, pending, err := proc.process(ctx, uid, 1)
	if err != nil || pending {
		t.Fatalf("err=%v pending=%v", err, pending)
	}
	if fake.updated != nil {
		t.Fatalf("no update expected, got %+v", fake.updated)
	}
	feed, _ := repo.Feed(ctx, uid)
	if len(feed) != 1 || len(feed[0].AppliedRules) != 0 {
		t.Fatalf("expected processed with no applied rules: %+v", feed)
	}
}

func TestProcess_SegmentCountInTemplate(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 33333)
	_ = repo.SetSegment(ctx, uid, store.Segment{Name: "albert_park", SegmentID: 6408668})
	rule := &engine.Rule{
		Name:  "Lap counter",
		Conds: []engine.Cond{{Field: "segments.albert_park_count", Op: "gt", Value: "1"}},
		Acts:  []engine.Act{{Key: "title", Template: "Albert Park x{segments.albert_park_count}"}},
	}
	_ = repo.CreateRule(ctx, uid, rule)

	fake := &fakeStrava{act: core.Activity{
		ID: 2, Type: "Ride", Name: "ride", Time: time.Now(),
		SegmentEfforts: []core.SegmentEffort{
			{SegmentID: 6408668}, {SegmentID: 6408668}, {SegmentID: 6408668}, {SegmentID: 42},
		},
	}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	_, pending, err := proc.process(ctx, uid, 2)
	if err != nil || pending {
		t.Fatalf("err=%v pending=%v", err, pending)
	}
	if fake.updated == nil || fake.updated.Name == nil || *fake.updated.Name != "Albert Park x3" {
		t.Fatalf("strava update = %+v", fake.updated)
	}
}

func TestProcess_ParkrunRuleSkippedWhenUnconnected(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 33333)
	rule := &engine.Rule{
		Name:  "parkrun title",
		Conds: []engine.Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []engine.Act{{Key: "title", Template: "parkrun #{parkrun.position}"}},
	}
	_ = repo.CreateRule(ctx, uid, rule)

	fake := &fakeStrava{act: core.Activity{ID: 7, Type: "Run", Name: "run", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	_, pending, err := proc.process(ctx, uid, 7)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatalf("parkrun not connected: rule should be skipped, not pending")
	}
	if fake.updated != nil {
		t.Fatalf("nothing should be written")
	}
	feed, _ := repo.Feed(ctx, uid)
	if len(feed) != 1 || feed[0].Status != "processed" || len(feed[0].AppliedRules) != 0 {
		t.Fatalf("expected processed with no applied rules: %+v", feed)
	}
}

func TestSync_EnqueuesOnlyNewActivities(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 55555)

	if _, err := repo.UpsertActivity(ctx, uid, 101, "processed", nil, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	fake := &fakeStrava{ids: []int64{101, 102, 103}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	added, err := proc.Sync(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Fatalf("added = %d, want 2", added)
	}

	feed, err := repo.Feed(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 3 {
		t.Fatalf("feed len = %d", len(feed))
	}
	for _, it := range feed {
		switch it.StravaID {
		case 101:
			if it.Status != "processed" || it.Job != nil {
				t.Fatalf("existing activity 101 should be untouched: status=%q job=%+v", it.Status, it.Job)
			}
		case 102, 103:
			if it.Status != "unprocessed" || it.Job == nil {
				t.Fatalf("new activity %d should be queued: status=%q job=%+v", it.StravaID, it.Status, it.Job)
			}
		}
	}
}

func TestRefreshStravaData_UpdatesCacheWithoutRerunningRules(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 66666)

	rule := &engine.Rule{
		Name:  "Name runs",
		Conds: []engine.Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []engine.Act{{Key: "title", Template: "Auto: {strava.activity_type}"}},
	}
	if err := repo.CreateRule(ctx, uid, rule); err != nil {
		t.Fatal(err)
	}

	fake := &fakeStrava{act: core.Activity{ID: 999, Type: "Run", Name: "old", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }
	if _, _, err := proc.process(ctx, uid, 999); err != nil {
		t.Fatal(err)
	}

	fake.act.Name = "Renamed on Strava"
	fake.act.Type = "Ride"
	fake.updated = nil

	known, err := proc.RefreshStravaData(ctx, uid, 999)
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("activity should be known")
	}
	if fake.updated != nil {
		t.Fatalf("refresh must not write to strava, got %+v", fake.updated)
	}

	feed, err := repo.Feed(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 {
		t.Fatalf("feed len = %d", len(feed))
	}
	it := feed[0]
	if it.Strava["activity_name"] != "Renamed on Strava" {
		t.Fatalf("cached name = %q", it.Strava["activity_name"])
	}
	if it.Status != "processed" || len(it.AppliedRules) != 1 {
		t.Fatalf("rules must be untouched: status=%q applied=%v", it.Status, it.AppliedRules)
	}

	known, err = proc.RefreshStravaData(ctx, uid, 12345)
	if err != nil || known {
		t.Fatalf("unknown activity: known=%v err=%v", known, err)
	}
}

func TestProcess_NonMatchingActivityNotPending(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 44444)
	_ = repo.SetParkrunID(ctx, uid, "A1234567")
	rule := &engine.Rule{
		Name:  "parkrun runs",
		Conds: []engine.Cond{{Field: "strava.activity_type", Op: "eq", Value: "Run"}},
		Acts:  []engine.Act{{Key: "title", Template: "parkrun #{parkrun.position}"}},
	}
	_ = repo.CreateRule(ctx, uid, rule)

	fake := &fakeStrava{act: core.Activity{ID: 8, Type: "Ride", Name: "ride", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token) strava.StravaAPI { return fake }

	_, pending, err := proc.process(ctx, uid, 8)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatalf("non-matching activity must not be pending")
	}
	feed, _ := repo.Feed(ctx, uid)
	if len(feed) != 1 || feed[0].Status != "processed" {
		t.Fatalf("expected processed: %+v", feed)
	}
}
