package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"dbut.dev/commuter/go/database"
	"dbut.dev/commuter/go/engine"
)

type pgRepo struct {
	q *database.Queries
}

func NewPGRepo(db *sql.DB) Repo {
	return &pgRepo{q: database.New(db)}
}

func nullTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: !t.IsZero()} }

func (p *pgRepo) UpsertUser(ctx context.Context, stravaAthleteID int64, name string) (string, error) {
	u, err := p.q.EnsureUser(ctx, database.EnsureUserParams{StravaAthleteID: stravaAthleteID, Name: name})
	if err != nil {
		return "", err
	}
	return u.ID.String(), nil
}

func (p *pgRepo) UserIDByStrava(ctx context.Context, athleteID int64) (string, error) {
	id, err := p.q.UserIDByAthlete(ctx, athleteID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (p *pgRepo) UserName(ctx context.Context, userID string) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	u, err := p.q.GetUser(ctx, uid)
	if err != nil {
		return "", err
	}
	return u.Name, nil
}

func (p *pgRepo) SetStravaToken(ctx context.Context, userID string, token []byte) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return p.q.SetStravaToken(ctx, database.SetStravaTokenParams{UserID: uid, Token: token})
}

func (p *pgRepo) DeleteUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return p.q.DeleteUser(ctx, uid)
}

func (p *pgRepo) Rules(ctx context.Context, userID string) ([]*engine.Rule, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	rows, err := p.q.ListRules(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]*engine.Rule, 0, len(rows))
	for _, row := range rows {
		r, err := toRule(row)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (p *pgRepo) Rule(ctx context.Context, userID, id string) (*engine.Rule, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	row, err := p.q.GetRule(ctx, database.GetRuleParams{ID: rid, UserID: uid})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRule(row)
}

func (p *pgRepo) CreateRule(ctx context.Context, userID string, r *engine.Rule) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	conds, err := json.Marshal(r.Conds)
	if err != nil {
		return err
	}
	acts, err := json.Marshal(r.Acts)
	if err != nil {
		return err
	}
	row, err := p.q.CreateRule(ctx, database.CreateRuleParams{
		UserID: uid, Name: r.Name, Conditions: conds, Actions: acts,
	})
	if err != nil {
		return err
	}
	r.ID = row.ID.String()
	r.Enabled = row.Enabled
	return nil
}

func (p *pgRepo) UpdateRule(ctx context.Context, userID string, r *engine.Rule) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	rid, err := uuid.Parse(r.ID)
	if err != nil {
		return err
	}
	conds, err := json.Marshal(r.Conds)
	if err != nil {
		return err
	}
	acts, err := json.Marshal(r.Acts)
	if err != nil {
		return err
	}
	_, err = p.q.UpdateRule(ctx, database.UpdateRuleParams{
		ID: rid, UserID: uid, Name: r.Name, Conditions: conds, Actions: acts, Enabled: r.Enabled,
	})
	return err
}

func (p *pgRepo) ToggleRule(ctx context.Context, userID, id string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return p.q.ToggleRule(ctx, database.ToggleRuleParams{ID: rid, UserID: uid})
}

func (p *pgRepo) DeleteRule(ctx context.Context, userID, id string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return p.q.DeleteRule(ctx, database.DeleteRuleParams{ID: rid, UserID: uid})
}

func (p *pgRepo) MoveRule(ctx context.Context, userID, id string, up bool) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	rows, err := p.q.ListRules(ctx, uid)
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range rows {
		if r.ID.String() == id {
			idx = i
		}
	}
	swap := idx - 1
	if !up {
		swap = idx + 1
	}
	if idx < 0 || swap < 0 || swap >= len(rows) {
		return nil
	}
	rows[idx], rows[swap] = rows[swap], rows[idx]
	for i, r := range rows {
		if int(r.Priority) == i {
			continue
		}
		err := p.q.SetRulePriority(ctx, database.SetRulePriorityParams{ID: r.ID, UserID: uid, Priority: int32(i)})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *pgRepo) RuleEnabled(ctx context.Context, userID, id string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	rid, err := uuid.Parse(id)
	if err != nil {
		return false, err
	}
	row, err := p.q.GetRule(ctx, database.GetRuleParams{ID: rid, UserID: uid})
	if err != nil {
		return false, err
	}
	return row.Enabled, nil
}

func (p *pgRepo) Vars(ctx context.Context, userID string) ([]Var, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	rows, err := p.q.ListVars(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]Var, len(rows))
	for i, r := range rows {
		out[i] = Var{Name: r.Name, Type: r.Type, Value: r.Value}
	}
	return out, nil
}

func (p *pgRepo) SetVar(ctx context.Context, userID string, v Var) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return p.q.UpsertVar(ctx, database.UpsertVarParams{UserID: uid, Name: v.Name, Type: v.Type, Value: v.Value})
}

func (p *pgRepo) DeleteVar(ctx context.Context, userID, name string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return p.q.DeleteVar(ctx, database.DeleteVarParams{UserID: uid, Name: name})
}

func (p *pgRepo) ParkrunID(ctx context.Context, userID string) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	return p.q.GetParkrunID(ctx, uid)
}

func (p *pgRepo) SetParkrunID(ctx context.Context, userID, id string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return p.q.SetParkrunID(ctx, database.SetParkrunIDParams{
		ID:        uid,
		ParkrunID: sql.NullString{String: id, Valid: id != ""},
	})
}

func (p *pgRepo) GetStravaToken(ctx context.Context, userID string) ([]byte, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	tok, err := p.q.GetStravaToken(ctx, uid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return tok, err
}

func (p *pgRepo) UpsertActivity(ctx context.Context, userID string, stravaID int64, status string, appliedRules []string, activityTime time.Time, runLog []RunEntry) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	if appliedRules == nil {
		appliedRules = []string{}
	}
	applied, err := json.Marshal(appliedRules)
	if err != nil {
		return "", err
	}
	if runLog == nil {
		runLog = []RunEntry{}
	}
	logJSON, err := json.Marshal(runLog)
	if err != nil {
		return "", err
	}
	row, err := p.q.UpsertActivity(ctx, database.UpsertActivityParams{
		UserID: uid, StravaID: stravaID, Status: status, AppliedRules: applied,
		ActivityTime: nullTime(activityTime), RunLog: logJSON,
	})
	if err != nil {
		return "", err
	}
	return row.ID.String(), nil
}

func (p *pgRepo) ActivityIDByStrava(ctx context.Context, userID string, stravaID int64) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	id, err := p.q.ActivityIDByStrava(ctx, database.ActivityIDByStravaParams{UserID: uid, StravaID: stravaID})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (p *pgRepo) SetActivityTime(ctx context.Context, activityID string, t time.Time) error {
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return err
	}
	return p.q.SetActivityTime(ctx, database.SetActivityTimeParams{ID: aid, ActivityTime: nullTime(t)})
}

func (p *pgRepo) ExistingStravaIDs(ctx context.Context, userID string, stravaIDs []int64) (map[int64]bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	ids, err := p.q.ExistingStravaIDs(ctx, database.ExistingStravaIDsParams{UserID: uid, StravaIds: stravaIDs})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (p *pgRepo) ActivityDetail(ctx context.Context, userID, activityID string) (*ActivityInfo, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return nil, err
	}
	a, err := p.q.GetActivity(ctx, database.GetActivityParams{ID: aid, UserID: uid})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	info := &ActivityInfo{
		ActivityID: a.ID.String(),
		StravaID:   a.StravaID,
		Status:     a.Status,
		UpdatedAt:  a.UpdatedAt,
		Strava:     map[string]string{},
	}
	_ = json.Unmarshal(a.AppliedRules, &info.AppliedRules)
	_ = json.Unmarshal(a.RunLog, &info.RunLog)

	blobs, err := p.q.ListProviderData(ctx, aid)
	if err != nil {
		return nil, err
	}
	for _, b := range blobs {
		pb := ProviderBlob{Provider: b.Provider, Found: b.Found, FetchedAt: b.FetchedAt, Data: map[string]string{}}
		_ = json.Unmarshal(b.Data, &pb.Data)
		if b.Provider == "strava" {
			info.Strava = pb.Data
		}
		info.Providers = append(info.Providers, pb)
	}

	j, err := p.q.GetJob(ctx, aid)
	if err == nil {
		info.Job = &JobInfo{Status: j.Status, NextRun: j.NextRun, ExpiresAt: j.ExpiresAt, LastError: j.LastError.String}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return info, nil
}

func (p *pgRepo) SetProviderData(ctx context.Context, activityID, provider string, data []byte, found bool) error {
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return err
	}
	return p.q.UpsertProviderData(ctx, database.UpsertProviderDataParams{
		ActivityID: aid, Provider: provider, Data: data, Found: found,
	})
}

func (p *pgRepo) UpsertJob(ctx context.Context, activityID string, nextRun, expiresAt time.Time) error {
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return err
	}
	return p.q.UpsertJob(ctx, database.UpsertJobParams{
		ActivityID: aid, NextRun: nextRun, ExpiresAt: expiresAt,
	})
}

func (p *pgRepo) CompleteJob(ctx context.Context, activityID, status, lastErr string) error {
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return err
	}
	return p.q.CompleteJob(ctx, database.CompleteJobParams{
		ActivityID: aid, Status: status,
		LastError: sql.NullString{String: lastErr, Valid: lastErr != ""},
	})
}

func (p *pgRepo) ClaimJobs(ctx context.Context, limit int) ([]ClaimedJob, error) {
	rows, err := p.q.ClaimJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]ClaimedJob, len(rows))
	for i, r := range rows {
		out[i] = ClaimedJob{
			ActivityID: r.ActivityID.String(), UserID: r.UserID.String(),
			StravaID: r.StravaID, ExpiresAt: r.ExpiresAt,
		}
	}
	return out, nil
}

func (p *pgRepo) SetActivityStatus(ctx context.Context, activityID, status string) error {
	aid, err := uuid.Parse(activityID)
	if err != nil {
		return err
	}
	return p.q.SetActivityStatus(ctx, database.SetActivityStatusParams{ID: aid, Status: status})
}

func (p *pgRepo) Feed(ctx context.Context, userID string) ([]FeedItem, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	rows, err := p.q.ListFeed(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]FeedItem, 0, len(rows))
	for _, r := range rows {
		item := FeedItem{
			ActivityID: r.ID.String(),
			StravaID:   r.StravaID,
			Status:     r.Status,
			Strava:     map[string]string{},
		}
		_ = json.Unmarshal(r.AppliedRules, &item.AppliedRules)
		if r.StravaData.Valid {
			_ = json.Unmarshal(r.StravaData.RawMessage, &item.Strava)
		}
		if r.JobStatus.Valid {
			item.Job = &JobInfo{
				Status:    r.JobStatus.String,
				NextRun:   r.JobNextRun.Time,
				ExpiresAt: r.JobExpiresAt.Time,
				LastError: r.JobLastError.String,
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func toRule(row database.Rule) (*engine.Rule, error) {
	var conds []engine.Cond
	var acts []engine.Act
	if len(row.Conditions) > 0 {
		if err := json.Unmarshal(row.Conditions, &conds); err != nil {
			return nil, err
		}
	}
	if len(row.Actions) > 0 {
		if err := json.Unmarshal(row.Actions, &acts); err != nil {
			return nil, err
		}
	}
	return &engine.Rule{
		ID:      row.ID.String(),
		Name:    row.Name,
		Enabled: row.Enabled,
		Conds:   conds,
		Acts:    acts,
	}, nil
}
