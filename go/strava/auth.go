package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

var stravaEndpoint = oauth2.Endpoint{
	AuthURL:  "https://www.strava.com/oauth/authorize",
	TokenURL: "https://www.strava.com/oauth/token",
}

type Auth struct {
	cfg *oauth2.Config
}

func NewAuth(clientID, clientSecret, baseURL string) *Auth {
	return &Auth{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     stravaEndpoint,
		RedirectURL:  strings.TrimRight(baseURL, "/") + "/auth/strava/callback",

		Scopes: []string{"read,activity:read_all,activity:write"},
	}}
}

func (a *Auth) AuthURL(state string) string {
	return a.cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("approval_prompt", "auto"))
}

func (a *Auth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return a.cfg.Exchange(ctx, code)
}

func (a *Auth) HTTPClient(ctx context.Context, tok *oauth2.Token) *http.Client {
	return a.cfg.Client(ctx, tok)
}

type stravaAthlete struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
}

func (a *Auth) Athlete(ctx context.Context, tok *oauth2.Token) (int64, string, error) {
	resp, err := a.cfg.Client(ctx, tok).Get("https://www.strava.com/api/v3/athlete")
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("strava athlete: status %d", resp.StatusCode)
	}
	var ath stravaAthlete
	if err := json.NewDecoder(resp.Body).Decode(&ath); err != nil {
		return 0, "", err
	}
	name := strings.TrimSpace(ath.FirstName + " " + ath.LastName)
	return ath.ID, name, nil
}
