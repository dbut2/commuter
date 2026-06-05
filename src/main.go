package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"dbut.dev/commuter/src/strava/client"
	"dbut.dev/commuter/src/strava/client/athletes"
)

// ctxKey is the type used for context values set by this package.
type ctxKey string

// accessTokenKey carries the Strava OAuth access token through the request
// context so per-call authenticators can read it.
const accessTokenKey ctxKey = "strava-access-token"

func main() {
	baseUrl := "https://commuter.dbut.dev"

	oauth2Config := oauth2.Config{
		ClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		ClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		Endpoint:     endpoints.Strava,
		RedirectURL:  baseUrl + "/strava/callback",
		Scopes:       []string{"activity:read,activity:read_all,activity:write"},
	}

	sClient := &stravaClient{api: client.NewHTTPClient(strfmt.Default)}
	stravaAPI := StravaClient(sClient)

	rClient := &redisClient{client: redis.NewClient(&redis.Options{Addr: "redis:6379"})}

	cfg := loadConfig()

	e := gin.Default()

	e.GET("/", func(c *gin.Context) {

	})

	e.GET("/strava/login", func(c *gin.Context) {
		c.Redirect(http.StatusFound, oauth2Config.AuthCodeURL("state"))
	})

	e.GET("/strava/callback", func(c *gin.Context) {
		code := c.Query("code")
		token, err := oauth2Config.Exchange(c, code)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		ctx := authContext(c, token)

		athleteResp, err := sClient.api.Athletes.GetLoggedInAthlete(athletes.NewGetLoggedInAthleteParams().WithContext(ctx), authInfo(ctx))
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		c.SetCookie("token", token.AccessToken, int(time.Until(token.Expiry).Seconds()), "/", "commuter.dbut.dev", true, true)

		err = rClient.StoreToken(ctx, athleteResp.Payload.ID, token)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Redirect(http.StatusFound, baseUrl)
	})

	e.GET("/strava/webhook", webhook(rClient, oauth2Config, stravaAPI, cfg))
	e.POST("/strava/webhook", webhook(rClient, oauth2Config, stravaAPI, cfg))

	fmt.Println("Starting server")

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	if err := http.ListenAndServe(":"+port, e); err != nil {
		log.Fatalf("failed to start server: %e", err)
	}
}

func webhook(rClient *redisClient, config oauth2.Config, client StravaClient, cfg Config) func(c *gin.Context) {
	return func(c *gin.Context) {
		if validate(c) {
			return
		}

		wh := StravaWebhook{}
		err := c.BindJSON(&wh)
		if err != nil {
			_ = c.Error(err)
			fmt.Println(err.Error())
			c.Status(http.StatusInternalServerError)
			return
		}

		fmt.Printf("%+v\n", wh)

		if wh.ObjectType != "activity" {
			fmt.Println("not an activity", wh.ObjectID)
			return
		}

		if wh.AspectType != "create" && wh.AspectType != "update" {
			fmt.Println("not a create or update")
			return
		}

		updaters, ok := cfg.UpdatersForUser(wh.OwnerID)
		if !ok {
			fmt.Printf("WARNING: no config for user %d\n", wh.OwnerID)
			return
		}

		fmt.Printf("Updating activity: %d\n", wh.ObjectID)

		ctx := context.Context(c)

		token, err := rClient.GetToken(ctx, config, wh.OwnerID)
		if err != nil {
			_ = c.Error(err)
			fmt.Println(err.Error())
			c.Status(http.StatusInternalServerError)
			return
		}

		ctx = authContext(ctx, token)

		activity := client.GetActivity(ctx, int(wh.ObjectID))
		original := activity
		for _, u := range updaters {
			activity = u(ctx, client, activity)
		}
		updated := activity != original

		fmt.Printf("Updated: %t\n", updated)

		if updated {
			client.UpdateActivity(ctx, activity)
		}
	}
}

func validate(c *gin.Context) bool {
	if c.Query("hub.mode") != "subscribe" {
		return false
	}

	fmt.Println("validation")

	challenge := c.Query("hub.challenge")

	c.JSON(http.StatusOK, struct {
		Challenge string `json:"hub.challenge"`
	}{
		Challenge: challenge,
	})

	return true
}

func authContext(ctx context.Context, token *oauth2.Token) context.Context {
	return context.WithValue(ctx, accessTokenKey, token.AccessToken)
}
