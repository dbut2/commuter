package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"dbut.dev/commuter/go/processor"
	"dbut.dev/commuter/go/server"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
)

func main() {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	clientID, clientSecret := os.Getenv("STRAVA_CLIENT_ID"), os.Getenv("STRAVA_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("commuter: STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET are required")
	}

	sqlDB, err := store.Connect(os.Getenv("DATABASE_DSN"))
	if err != nil {
		log.Fatalf("commuter: db: %v", err)
	}
	log.Printf("commuter: using postgres (schema %s)", store.Schema)
	repo := store.NewPGRepo(sqlDB)

	webhookToken := os.Getenv("STRAVA_WEBHOOK_TOKEN")
	auth := strava.NewAuth(clientID, clientSecret, baseURL)
	if r := os.Getenv("STRAVA_REDIRECT_URL"); r != "" {
		auth.SetRedirectURL(r)
	}
	proc, err := processor.New(sqlDB, repo, auth)
	if err != nil {
		log.Fatalf("commuter: queue: %v", err)
	}
	srv, err := server.New(repo, auth, proc, baseURL, []byte(os.Getenv("SESSION_SECRET")), webhookToken)
	if err != nil {
		log.Fatalf("commuter: %v", err)
	}
	if err := proc.Start(context.Background()); err != nil {
		log.Fatalf("commuter: queue: %v", err)
	}

	log.Printf("commuter listening on :%s", port)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatal(err)
	}
}
