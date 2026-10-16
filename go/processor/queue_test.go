package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"golang.org/x/oauth2"

	"dbut.dev/commuter/go/core"
	"dbut.dev/commuter/go/strava"
)

func testJob(expiresAt time.Time) *river.Job[ProcessActivityArgs] {
	return &river.Job[ProcessActivityArgs]{
		JobRow: &rivertype.JobRow{},
		Args:   ProcessActivityArgs{UserID: "u", StravaID: 1, ExpiresAt: expiresAt},
	}
}

func TestWorker_SnoozesWhilePending(t *testing.T) {
	w := &activityWorker{
		process: func(context.Context, string, int64) (string, bool, error) { return "a", true, nil },
		setFinished: func(context.Context, string, string) error {
			t.Fatal("must not finish before expiry")
			return nil
		},
	}
	err := w.Work(context.Background(), testJob(time.Now().Add(time.Hour)))
	var snooze *rivertype.JobSnoozeError
	if !errors.As(err, &snooze) || snooze.Duration != pollBackoff {
		t.Fatalf("err = %v, want snooze for %v", err, pollBackoff)
	}
}

func TestWorker_GivesUpAfterExpiry(t *testing.T) {
	var finished string
	w := &activityWorker{
		process: func(context.Context, string, int64) (string, bool, error) { return "a", true, nil },
		setFinished: func(_ context.Context, id, status string) error {
			finished = id + ":" + status
			return nil
		},
	}
	if err := w.Work(context.Background(), testJob(time.Now().Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	if finished != "a:processed" {
		t.Fatalf("finished = %q", finished)
	}
}

func TestWorker_ReturnsProcessError(t *testing.T) {
	boom := errors.New("boom")
	w := &activityWorker{
		process: func(context.Context, string, int64) (string, bool, error) { return "", false, boom },
	}
	if err := w.Work(context.Background(), testJob(time.Now().Add(time.Hour))); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestEnqueue_DeduplicatesJobs(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()
	uid := seedUser(t, repo, 77777)

	for range 3 {
		if err := proc.Enqueue(ctx, uid, 501); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := sqlDB.QueryRow("select count(*) from river_job where kind = 'process_activity'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("jobs = %d, want 1", n)
	}
	feed, err := repo.Feed(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].Job == nil || feed[0].Job.Status != "queued" {
		t.Fatalf("feed = %+v", feed)
	}
}

func TestQueue_ProcessesEnqueuedActivity(t *testing.T) {
	proc, repo, sqlDB := testProcessor(t)
	defer func() { _ = sqlDB.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	uid := seedUser(t, repo, 88888)

	fake := &fakeStrava{act: core.Activity{ID: 601, Type: "Run", Name: "run", Time: time.Now()}}
	proc.stravaFactory = func(context.Context, *oauth2.Token, func(*oauth2.Token) error) strava.StravaAPI { return fake }

	if err := proc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proc.river.Stop(context.Background()) }()
	if err := proc.Enqueue(ctx, uid, 601); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		feed, err := repo.Feed(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		if len(feed) == 1 && feed[0].Status == "processed" && feed[0].Job != nil && feed[0].Job.Status == "completed" {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("activity was not processed by the queue")
}
