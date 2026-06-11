package processor

import (
	"context"
	"log"
	"time"
)

const (
	jobTTL      = 2 * time.Hour
	pollEvery   = 15 * time.Second
	pollBackoff = 20 * time.Minute
	claimBatch  = 20
)

func (p *Processor) Enqueue(ctx context.Context, userID string, stravaID int64) error {
	aid, err := p.repo.UpsertActivity(ctx, userID, stravaID, "unprocessed", nil, time.Time{}, nil)
	if err != nil {
		return err
	}
	now := time.Now()
	return p.repo.UpsertJob(ctx, aid, now, now.Add(jobTTL))
}

func (p *Processor) StartPoller(ctx context.Context) {
	go func() {
		t := time.NewTicker(pollEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.pollOnce(ctx)
			}
		}
	}()
}

func (p *Processor) KickPoll() { go p.pollOnce(context.Background()) }

func (p *Processor) pollOnce(ctx context.Context) {
	if !p.pollMu.TryLock() {
		return
	}
	defer p.pollMu.Unlock()

	jobs, err := p.repo.ClaimJobs(ctx, claimBatch)
	if err != nil {
		log.Printf("commuter: claim jobs: %v", err)
		return
	}
	for _, j := range jobs {
		aid, pending, err := p.process(ctx, j.UserID, j.StravaID)
		switch {
		case err != nil:
			log.Printf("commuter: job %s: %v", j.ActivityID, err)
			_ = p.repo.CompleteJob(ctx, j.ActivityID, "failed", err.Error())
		case !pending:
			_ = p.repo.CompleteJob(ctx, aid, "completed", "")
		case time.Now().After(j.ExpiresAt):
			_ = p.repo.CompleteJob(ctx, aid, "completed", "gave up waiting")
			_ = p.repo.SetActivityStatus(ctx, aid, "processed")
		default:
			_ = p.repo.UpsertJob(ctx, aid, time.Now().Add(pollBackoff), j.ExpiresAt)
		}
	}
}
