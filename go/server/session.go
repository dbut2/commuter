package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	cookieName  = "commuter_session"
	stateCookie = "commuter_oauth_state"
	sessionTTL  = 30 * 24 * time.Hour
)

type sessions struct {
	secret []byte
	secure bool
	now    func() time.Time
}

func newSessions(secret []byte, secure bool) *sessions {
	return &sessions{secret: secret, secure: secure, now: time.Now}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func randomToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(24)) }

func (s *sessions) mac(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *sessions) sign(userID string) string {
	payload := userID + "|" + strconv.FormatInt(s.now().Unix(), 10)
	return payload + "|" + s.mac(payload)
}

func (s *sessions) verify(v string) (string, bool) {
	i := strings.LastIndex(v, "|")
	if i < 0 {
		return "", false
	}
	payload, sig := v[:i], v[i+1:]
	if !hmac.Equal([]byte(sig), []byte(s.mac(payload))) {
		return "", false
	}
	j := strings.LastIndex(payload, "|")
	if j < 0 {
		return "", false
	}
	issued, err := strconv.ParseInt(payload[j+1:], 10, 64)
	if err != nil || s.now().Sub(time.Unix(issued, 0)) > sessionTTL {
		return "", false
	}
	return payload[:j], true
}

func (s *sessions) set(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.sign(userID),
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *sessions) clear(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure})
}

func (s *sessions) setState(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func sameOrigin(origin string, exempt ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		for _, p := range exempt {
			if c.Request.URL.Path == p {
				c.Next()
				return
			}
		}
		got := c.GetHeader("Origin")
		if got == "" {
			if ref, err := url.Parse(c.GetHeader("Referer")); err == nil && ref.Host != "" {
				got = ref.Scheme + "://" + ref.Host
			}
		}
		if got != origin {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
