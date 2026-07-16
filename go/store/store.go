package store

import (
	"context"
	"time"

	"dbut.dev/commuter/go/engine"
)

type Repo interface {
	UpsertUser(ctx context.Context, stravaAthleteID int64, name string) (userID string, err error)
	UserIDByStrava(ctx context.Context, stravaAthleteID int64) (string, error)
	UserName(ctx context.Context, userID string) (string, error)
	SetStravaToken(ctx context.Context, userID string, token []byte) error
	DeleteUser(ctx context.Context, userID string) error

	Rules(ctx context.Context, userID string) ([]*engine.Rule, error)
	Rule(ctx context.Context, userID, id string) (*engine.Rule, error)
	CreateRule(ctx context.Context, userID string, r *engine.Rule) error
	UpdateRule(ctx context.Context, userID string, r *engine.Rule) error
	ToggleRule(ctx context.Context, userID, id string) error
	DeleteRule(ctx context.Context, userID, id string) error
	MoveRule(ctx context.Context, userID, id string, up bool) error
	RuleEnabled(ctx context.Context, userID, id string) (bool, error)

	Vars(ctx context.Context, userID string) ([]Var, error)
	SetVar(ctx context.Context, userID string, v Var) error
	DeleteVar(ctx context.Context, userID, name string) error

	Segments(ctx context.Context, userID string) ([]Segment, error)
	SetSegment(ctx context.Context, userID string, s Segment) error
	DeleteSegment(ctx context.Context, userID, name string) error

	ParkrunID(ctx context.Context, userID string) (string, error)
	SetParkrunID(ctx context.Context, userID, id string) error

	GetStravaToken(ctx context.Context, userID string) ([]byte, error)
	UpsertActivity(ctx context.Context, userID string, stravaID int64, status string, appliedRules []string, activityTime time.Time, runLog []RunEntry) (activityID string, err error)
	ExistingStravaIDs(ctx context.Context, userID string, stravaIDs []int64) (map[int64]bool, error)
	ActivityIDByStrava(ctx context.Context, userID string, stravaID int64) (string, error)
	SetActivityTime(ctx context.Context, activityID string, t time.Time) error
	SetProviderData(ctx context.Context, activityID, provider string, data []byte, found bool) error
	Feed(ctx context.Context, userID string) ([]FeedItem, error)
	ActivityDetail(ctx context.Context, userID, activityID string) (*ActivityInfo, error)
	SetActivityStatus(ctx context.Context, activityID, status string) error
	UpsertJob(ctx context.Context, activityID string, nextRun, expiresAt time.Time) error
	CompleteJob(ctx context.Context, activityID, status, lastErr string) error
	ClaimJobs(ctx context.Context, limit int) ([]ClaimedJob, error)
}

type Var struct {
	Name  string
	Type  string
	Value string
}

type Segment struct {
	Name      string
	SegmentID int64
}

type RunEntry struct {
	Rule   string `json:"rule"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type ProviderBlob struct {
	Provider  string
	Found     bool
	FetchedAt time.Time
	Data      map[string]string
}

type ActivityInfo struct {
	ActivityID   string
	StravaID     int64
	Status       string
	AppliedRules []string
	RunLog       []RunEntry
	UpdatedAt    time.Time
	Strava       map[string]string
	Providers    []ProviderBlob
	Job          *JobInfo
}

type FeedItem struct {
	ActivityID   string
	StravaID     int64
	Status       string
	AppliedRules []string
	Strava       map[string]string
	Job          *JobInfo
}

type JobInfo struct {
	Status    string
	NextRun   time.Time
	ExpiresAt time.Time
	LastError string
}

type ClaimedJob struct {
	ActivityID string
	UserID     string
	StravaID   int64
	ExpiresAt  time.Time
}
