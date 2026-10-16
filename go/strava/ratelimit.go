package strava

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RateLimitError struct {
	Usage   string
	ResetAt time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("strava rate limited (usage %s), resets %s", e.Usage, e.ResetAt.Format(time.RFC3339))
}

type rateLimitTransport struct {
	base http.RoundTripper
	now  func() time.Time
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusTooManyRequests {
		return resp, err
	}
	_ = resp.Body.Close()
	now := t.now().UTC()
	e := &RateLimitError{Usage: usageSummary(resp.Header), ResetAt: nextQuarterHour(now)}
	if dailyExhausted(resp.Header) {
		e.ResetAt = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	}
	log.Printf("commuter: %v", e)
	return nil, e
}

func nextQuarterHour(t time.Time) time.Time {
	return t.Truncate(15 * time.Minute).Add(15 * time.Minute)
}

func usageSummary(h http.Header) string {
	return fmt.Sprintf("overall %s of %s, read %s of %s",
		h.Get("X-RateLimit-Usage"), h.Get("X-RateLimit-Limit"),
		h.Get("X-ReadRateLimit-Usage"), h.Get("X-ReadRateLimit-Limit"))
}

func dailyExhausted(h http.Header) bool {
	for _, p := range [][2]string{
		{"X-RateLimit-Usage", "X-RateLimit-Limit"},
		{"X-ReadRateLimit-Usage", "X-ReadRateLimit-Limit"},
	} {
		used, okU := daily(h.Get(p[0]))
		limit, okL := daily(h.Get(p[1]))
		if okU && okL && used >= limit {
			return true
		}
	}
	return false
}

func daily(v string) (int, bool) {
	parts := strings.Split(v, ",")
	if len(parts) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	return n, err == nil
}
