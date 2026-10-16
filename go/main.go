package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	httpSrv := &http.Server{Addr: ":" + port, Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("commuter listening on :%s", port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Print("commuter: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("commuter: http shutdown: %v", err)
	}
	if err := proc.Stop(shutdownCtx); err != nil {
		log.Printf("commuter: queue shutdown: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("commuter: db close: %v", err)
	}
}
