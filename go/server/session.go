package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

const (
	cookieName  = "commuter_session"
	stateCookie = "commuter_oauth_state"
)

type sessions struct {
	secret []byte
}

func newSessions(secret []byte) *sessions {
	if len(secret) == 0 {
		secret = randomBytes(32)
	}
	return &sessions{secret: secret}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func randomToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes(24)) }

func (s *sessions) sign(userID string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(userID))
	return userID + "|" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *sessions) verify(v string) (string, bool) {
	i := strings.LastIndex(v, "|")
	if i < 0 {
		return "", false
	}
	userID, sig := v[:i], v[i+1:]
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(userID))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if hmac.Equal([]byte(sig), []byte(want)) {
		return userID, true
	}
	return "", false
}

func (s *sessions) set(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.sign(userID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

func setStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
