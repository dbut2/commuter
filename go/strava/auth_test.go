package strava

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestHTTPClient_SavesRefreshedToken(t *testing.T) {
	var refreshes atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := refreshes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"access_token":"new-%d","refresh_token":"rotated-%d","token_type":"Bearer","expires_in":21600}`, n, n)
	}))
	defer tokenSrv.Close()

	var gotAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer apiSrv.Close()

	a := &Auth{cfg: &oauth2.Config{ClientID: "id", ClientSecret: "secret", Endpoint: oauth2.Endpoint{TokenURL: tokenSrv.URL}}}
	expired := &oauth2.Token{AccessToken: "old", RefreshToken: "refresh", TokenType: "Bearer", Expiry: time.Now().Add(-time.Hour)}

	var saved []*oauth2.Token
	client := a.HTTPClient(context.Background(), expired, func(tok *oauth2.Token) error {
		saved = append(saved, tok)
		return nil
	})

	for range 2 {
		resp, err := client.Get(apiSrv.URL)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}

	if refreshes.Load() != 1 {
		t.Fatalf("refreshes = %d, want 1", refreshes.Load())
	}
	if len(saved) != 1 {
		t.Fatalf("saved %d tokens, want 1", len(saved))
	}
	if saved[0].AccessToken != "new-1" || saved[0].RefreshToken != "rotated-1" {
		t.Fatalf("saved token = %+v", saved[0])
	}
	if gotAuth != "Bearer new-1" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestHTTPClient_ValidTokenNotSaved(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer apiSrv.Close()

	a := &Auth{cfg: &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "http://unused.invalid"}}}
	valid := &oauth2.Token{AccessToken: "ok", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}

	client := a.HTTPClient(context.Background(), valid, func(*oauth2.Token) error {
		t.Fatal("save called for a valid token")
		return nil
	})
	resp, err := client.Get(apiSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}
