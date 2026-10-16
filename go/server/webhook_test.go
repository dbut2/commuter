package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/go/processor"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
)

func testWebhook(t *testing.T) (*gin.Engine, store.Repo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("COMMUTER_TEST_DSN")
	if dsn == "" {
		t.Skip("set COMMUTER_TEST_DSN to run webhook tests")
	}
	sqlDB, err := store.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec("truncate users, rules, activities, provider_data, river_job, strava_credentials cascade"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	repo := store.NewPGRepo(sqlDB)
	proc, err := processor.New(sqlDB, repo, strava.NewAuth("id", "secret", "http://localhost"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{repo: repo, proc: proc}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/strava/webhook", s.webhookEvent)
	return r, repo, sqlDB
}

func post(t *testing.T, r *gin.Engine, body string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/strava/webhook", strings.NewReader(body)))
	return rec.Code
}

func jobKinds(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	rows, err := db.Query("select kind, count(*) from river_job group by kind")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			t.Fatal(err)
		}
		out[k] = n
	}
	return out
}

func TestWebhook_CreateEnqueuesProcessing(t *testing.T) {
	r, repo, db := testWebhook(t)
	if _, err := repo.UpsertUser(context.Background(), 100, "u"); err != nil {
		t.Fatal(err)
	}
	if code := post(t, r, `{"object_type":"activity","aspect_type":"create","object_id":1,"owner_id":100}`); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if got := jobKinds(t, db); got["process_activity"] != 1 || got["refresh_activity"] != 0 {
		t.Fatalf("jobs = %v", got)
	}
}

func TestWebhook_UpdateKnownActivityEnqueuesRefresh(t *testing.T) {
	r, repo, db := testWebhook(t)
	ctx := context.Background()
	uid, err := repo.UpsertUser(ctx, 101, "u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertActivity(ctx, uid, 2, "processed", nil, time.Time{}, nil); err != nil {
		t.Fatal(err)
	}
	body := `{"object_type":"activity","aspect_type":"update","object_id":2,"owner_id":101,"updates":{"title":"x"}}`
	for range 2 {
		if code := post(t, r, body); code != http.StatusOK {
			t.Fatalf("status %d", code)
		}
	}
	if got := jobKinds(t, db); got["refresh_activity"] != 1 || got["process_activity"] != 0 {
		t.Fatalf("jobs = %v", got)
	}
}

func TestWebhook_UpdateUnknownActivityEnqueuesProcessing(t *testing.T) {
	r, repo, db := testWebhook(t)
	if _, err := repo.UpsertUser(context.Background(), 102, "u"); err != nil {
		t.Fatal(err)
	}
	if code := post(t, r, `{"object_type":"activity","aspect_type":"update","object_id":3,"owner_id":102}`); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if got := jobKinds(t, db); got["process_activity"] != 1 {
		t.Fatalf("jobs = %v", got)
	}
}

func TestWebhook_DeauthorizeDeletesUser(t *testing.T) {
	r, repo, _ := testWebhook(t)
	ctx := context.Background()
	if _, err := repo.UpsertUser(ctx, 103, "u"); err != nil {
		t.Fatal(err)
	}
	if code := post(t, r, `{"object_type":"athlete","aspect_type":"update","object_id":103,"owner_id":103,"updates":{"authorized":"false"}}`); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	uid, err := repo.UserIDByStrava(ctx, 103)
	if err != nil || uid != "" {
		t.Fatalf("user still present: %q %v", uid, err)
	}
}

func TestWebhook_IgnoresUnknownAthleteAndOtherEvents(t *testing.T) {
	r, repo, db := testWebhook(t)
	ctx := context.Background()
	if _, err := repo.UpsertUser(ctx, 104, "u"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"object_type":"activity","aspect_type":"create","object_id":5,"owner_id":999}`,
		`{"object_type":"activity","aspect_type":"delete","object_id":5,"owner_id":104}`,
		`{"object_type":"athlete","aspect_type":"update","object_id":104,"owner_id":104,"updates":{"name":"x"}}`,
		`not json`,
	} {
		if code := post(t, r, body); code != http.StatusOK {
			t.Fatalf("%s: status %d", body, code)
		}
	}
	if got := jobKinds(t, db); len(got) != 0 {
		t.Fatalf("jobs = %v", got)
	}
	if uid, _ := repo.UserIDByStrava(ctx, 104); uid == "" {
		t.Fatal("non-deauth athlete update deleted the user")
	}
}
