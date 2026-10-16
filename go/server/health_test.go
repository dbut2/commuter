package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/go/store"
)

func TestHealth(t *testing.T) {
	dsn := os.Getenv("COMMUTER_TEST_DSN")
	if dsn == "" {
		t.Skip("set COMMUTER_TEST_DSN to run health tests")
	}
	sqlDB, err := store.Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	s := &Server{repo: store.NewPGRepo(sqlDB)}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", s.health)

	get := func() int {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		return rec.Code
	}
	if code := get(); code != http.StatusOK {
		t.Fatalf("healthy: status %d", code)
	}
	_ = sqlDB.Close()
	if code := get(); code != http.StatusServiceUnavailable {
		t.Fatalf("db closed: status %d", code)
	}
}
