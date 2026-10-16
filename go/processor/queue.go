package processor

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivertype"

	"dbut.dev/commuter/go/strava"
)

const (
	jobTTL      = 2 * time.Hour
	pollBackoff = 20 * time.Minute
)

type ProcessActivityArgs struct {
	UserID    string    `json:"user_id"`
	StravaID  int64     `json:"strava_id" river:"unique"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (ProcessActivityArgs) Kind() string { return "process_activity" }

func (ProcessActivityArgs) InsertOpts() river.InsertOpts { return uniqueInsertOpts }

type RefreshActivityArgs struct {
	UserID   string `json:"user_id"`
	StravaID int64  `json:"strava_id" river:"unique"`
}

func (RefreshActivityArgs) Kind() string { return "refresh_activity" }

func (RefreshActivityArgs) InsertOpts() river.InsertOpts { return uniqueInsertOpts }

var uniqueInsertOpts = river.InsertOpts{
	MaxAttempts: 3,
	UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRetryable,
			rivertype.JobStateRunning,
			rivertype.JobStateScheduled,
		},
	},
}

type activityWorker struct {
	river.WorkerDefaults[ProcessActivityArgs]
	process     func(ctx context.Context, userID string, stravaID int64) (string, bool, error)
	setFinished func(ctx context.Context, activityID, status string) error
}

func (w *activityWorker) Work(ctx context.Context, job *river.Job[ProcessActivityArgs]) error {
	activityID, pending, err := w.process(ctx, job.Args.UserID, job.Args.StravaID)
	if err != nil {
		return snoozeIfRateLimited(err)
	}
	if !pending {
		return nil
	}
	if time.Now().After(job.Args.ExpiresAt) {
		return w.setFinished(ctx, activityID, "processed")
	}
	return river.JobSnooze(pollBackoff)
}

type refreshWorker struct {
	river.WorkerDefaults[RefreshActivityArgs]
	p *Processor
}

func (w *refreshWorker) Work(ctx context.Context, job *river.Job[RefreshActivityArgs]) error {
	_, err := w.p.RefreshStravaData(ctx, job.Args.UserID, job.Args.StravaID)
	return snoozeIfRateLimited(err)
}

func snoozeIfRateLimited(err error) error {
	var rl *strava.RateLimitError
	if errors.As(err, &rl) {
		return river.JobSnooze(max(time.Until(rl.ResetAt), time.Minute))
	}
	return err
}

func newRiver(db *sql.DB, p *Processor) (*river.Client[*sql.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &activityWorker{process: p.process, setFinished: p.repo.SetActivityStatus})
	river.AddWorker(workers, &refreshWorker{p: p})
	return river.NewClient(riverdatabasesql.New(db), &river.Config{
		Queues:            map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}},
		Workers:           workers,
		FetchPollInterval: 5 * time.Second,
		SoftStopTimeout:   5 * time.Second,
	})
}

func (p *Processor) Start(ctx context.Context) error { return p.river.Start(ctx) }

func (p *Processor) Stop(ctx context.Context) error { return p.river.Stop(ctx) }

func (p *Processor) Enqueue(ctx context.Context, userID string, stravaID int64) error {
	if _, err := p.repo.UpsertActivity(ctx, userID, stravaID, "unprocessed", nil, time.Time{}, nil); err != nil {
		return err
	}
	res, err := p.river.Insert(ctx, ProcessActivityArgs{UserID: userID, StravaID: stravaID, ExpiresAt: time.Now().Add(jobTTL)}, nil)
	if err != nil {
		return err
	}
	if res.UniqueSkippedAsDuplicate && res.Job.State == rivertype.JobStateScheduled {
		_, err = p.river.JobRetry(ctx, res.Job.ID)
	}
	return err
}

func (p *Processor) EnqueueRefresh(ctx context.Context, userID string, stravaID int64) error {
	activityID, err := p.repo.ActivityIDByStrava(ctx, userID, stravaID)
	if err != nil {
		return err
	}
	if activityID == "" {
		return p.Enqueue(ctx, userID, stravaID)
	}
	_, err = p.river.Insert(ctx, RefreshActivityArgs{UserID: userID, StravaID: stravaID}, nil)
	return err
}
