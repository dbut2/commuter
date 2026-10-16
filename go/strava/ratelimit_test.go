package strava

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, headers map[string]string) roundTripperFunc {
	return func(*http.Request) (*http.Response, error) {
		h := http.Header{}
		for k, v := range headers {
			h.Set(k, v)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}
}

var fixedNow = time.Date(2026, 10, 16, 6, 37, 12, 0, time.UTC)

func TestRateLimitTransport_ShortTermResetsAtNextQuarterHour(t *testing.T) {
	tr := &rateLimitTransport{now: func() time.Time { return fixedNow }, base: respond(http.StatusTooManyRequests, map[string]string{
		"X-RateLimit-Limit": "200,2000", "X-RateLimit-Usage": "201,900",
	})}
	_, err := tr.RoundTrip(testRequest(t))
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v", err)
	}
	if want := time.Date(2026, 10, 16, 6, 45, 0, 0, time.UTC); !rl.ResetAt.Equal(want) {
		t.Fatalf("reset = %v, want %v", rl.ResetAt, want)
	}
}

func TestRateLimitTransport_DailyResetsAtMidnightUTC(t *testing.T) {
	tr := &rateLimitTransport{now: func() time.Time { return fixedNow }, base: respond(http.StatusTooManyRequests, map[string]string{
		"X-RateLimit-Limit": "200,2000", "X-RateLimit-Usage": "50,1500",
		"X-ReadRateLimit-Limit": "100,1000", "X-ReadRateLimit-Usage": "40,1000",
	})}
	_, err := tr.RoundTrip(testRequest(t))
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v", err)
	}
	if want := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC); !rl.ResetAt.Equal(want) {
		t.Fatalf("reset = %v, want %v", rl.ResetAt, want)
	}
}

func TestRateLimitTransport_PassesThroughOtherResponses(t *testing.T) {
	tr := &rateLimitTransport{now: time.Now, base: respond(http.StatusOK, nil)}
	resp, err := tr.RoundTrip(testRequest(t))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestRateLimitError_SurvivesGeneratedClient(t *testing.T) {
	c := NewClient(&http.Client{Transport: &rateLimitTransport{now: time.Now, base: respond(http.StatusTooManyRequests, map[string]string{
		"X-RateLimit-Limit": "200,2000", "X-RateLimit-Usage": "200,300",
	})}})
	_, err := c.GetActivity(context.Background(), 1)
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("errors.As failed through generated client: %T %v", err, err)
	}
}

func testRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://www.strava.com/api/v3/activities/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
