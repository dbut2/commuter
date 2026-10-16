package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSessions_RoundTrip(t *testing.T) {
	s := newSessions([]byte("secret"), true)
	uid, ok := s.verify(s.sign("user-1"))
	if !ok || uid != "user-1" {
		t.Fatalf("verify = %q, %v", uid, ok)
	}
}

func TestSessions_RejectsTampered(t *testing.T) {
	s := newSessions([]byte("secret"), true)
	v := s.sign("user-1")
	for name, bad := range map[string]string{
		"other user":   strings.Replace(v, "user-1", "user-2", 1),
		"other secret": newSessions([]byte("other"), true).sign("user-1"),
		"legacy":       "user-1|" + s.mac("user-1"),
		"empty":        "",
	} {
		if _, ok := s.verify(bad); ok {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSessions_Expires(t *testing.T) {
	s := newSessions([]byte("secret"), true)
	issued := time.Now()
	s.now = func() time.Time { return issued }
	v := s.sign("user-1")

	s.now = func() time.Time { return issued.Add(sessionTTL - time.Minute) }
	if _, ok := s.verify(v); !ok {
		t.Fatal("rejected before expiry")
	}
	s.now = func() time.Time { return issued.Add(sessionTTL + time.Minute) }
	if _, ok := s.verify(v); ok {
		t.Fatal("accepted after expiry")
	}
}

func TestSessions_CookieFlags(t *testing.T) {
	rec := httptest.NewRecorder()
	newSessions([]byte("secret"), true).set(rec, "user-1")
	c := rec.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.MaxAge != int(sessionTTL.Seconds()) || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie = %+v", c)
	}
}

func TestSameOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sameOrigin("https://app.example", "/strava/webhook"))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/rules", ok)
	r.POST("/rules/save", ok)
	r.POST("/strava/webhook", ok)

	cases := []struct {
		name, method, path, origin, referer string
		want                                int
	}{
		{"get without origin", http.MethodGet, "/rules", "", "", http.StatusOK},
		{"post same origin", http.MethodPost, "/rules/save", "https://app.example", "", http.StatusOK},
		{"post referer fallback", http.MethodPost, "/rules/save", "", "https://app.example/rules", http.StatusOK},
		{"post cross origin", http.MethodPost, "/rules/save", "https://evil.example", "", http.StatusForbidden},
		{"post http downgrade", http.MethodPost, "/rules/save", "http://app.example", "", http.StatusForbidden},
		{"post no origin or referer", http.MethodPost, "/rules/save", "", "", http.StatusForbidden},
		{"webhook exempt", http.MethodPost, "/strava/webhook", "", "", http.StatusOK},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.referer != "" {
			req.Header.Set("Referer", tc.referer)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, rec.Code, tc.want)
		}
	}
}
