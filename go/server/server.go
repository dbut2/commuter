package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/go/processor"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
	"dbut.dev/commuter/web"
)

type Server struct {
	repo         store.Repo
	auth         *strava.Auth
	sess         *sessions
	engine       *gin.Engine
	webhookToken string
	proc         *processor.Processor
}

func New(repo store.Repo, auth *strava.Auth, proc *processor.Processor, baseURL string, sessionSecret []byte, webhookToken string) (*Server, error) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid base url %q", baseURL)
	}
	if len(sessionSecret) == 0 {
		return nil, fmt.Errorf("session secret is required")
	}
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		repo:         repo,
		auth:         auth,
		proc:         proc,
		sess:         newSessions(sessionSecret, base.Scheme == "https"),
		webhookToken: webhookToken,
		engine:       gin.New(),
	}
	s.engine.Use(gin.Recovery(), sameOrigin(base.Scheme+"://"+base.Host, "/strava/webhook"))
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.engine.ServeHTTP(w, r) }

func (s *Server) routes() {
	r := s.engine
	r.StaticFS("/static", http.FS(web.Static))
	r.GET("/health", s.health)
	r.GET("/", s.root)
	r.POST("/connect", s.connect)
	r.GET("/auth/strava/callback", s.stravaCallback)
	r.POST("/logout", s.logout)
	r.POST("/account/delete", s.deleteAccount)
	r.GET("/strava/webhook", s.webhookVerify)
	r.POST("/strava/webhook", s.webhookEvent)

	r.GET("/activities", s.activitiesPage)
	r.POST("/activities/sync", s.activitiesSync)
	r.GET("/activities/:id", s.activityPage)
	r.POST("/activities/:id/rerun", s.activityRerun)

	r.GET("/rules", s.rulesPage)
	r.GET("/rules/edit", s.ruleEditor)
	r.POST("/rules/save", s.ruleSave)
	r.POST("/rules/delete", s.ruleDelete)
	r.POST("/rules/toggle", s.ruleToggle)
	r.POST("/rules/move", s.ruleMove)
	r.GET("/rules/rows/cond", s.condRowFragment)
	r.GET("/rules/rows/act", s.actRowFragment)

	r.GET("/vars", s.varsPage)
	r.POST("/vars", s.varSave)
	r.POST("/vars/delete", s.varDelete)

	r.GET("/settings", s.settingsPage)
	r.POST("/settings/parkrun", s.settingsParkrun)
	r.POST("/settings/segments", s.segmentSave)
	r.POST("/settings/segments/delete", s.segmentDelete)
}

func (s *Server) health(c *gin.Context) {
	if err := s.repo.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) fail(c *gin.Context, err error) {
	log.Printf("commuter: %v", err)
	c.String(http.StatusInternalServerError, "internal error")
}

func (s *Server) user(c *gin.Context) (string, bool) {
	v, err := c.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	return s.sess.verify(v)
}

func (s *Server) connect(c *gin.Context) {
	state := randomToken()
	s.sess.setState(c.Writer, state)
	c.Redirect(http.StatusSeeOther, s.auth.AuthURL(state))
}

func (s *Server) stravaCallback(c *gin.Context) {
	ctx := c.Request.Context()
	st, err := c.Cookie(stateCookie)
	if err != nil || st == "" || st != c.Query("state") {
		c.String(http.StatusBadRequest, "invalid oauth state")
		return
	}
	s.sess.clear(c.Writer, stateCookie)
	if c.Query("error") != "" {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	tok, err := s.auth.Exchange(ctx, c.Query("code"))
	if err != nil {
		s.fail(c, err)
		return
	}
	athID, name, err := s.auth.Athlete(ctx, tok)
	if err != nil {
		s.fail(c, err)
		return
	}
	uid, err := s.repo.UpsertUser(ctx, athID, name)
	if err != nil {
		s.fail(c, err)
		return
	}
	tokJSON, err := json.Marshal(tok)
	if err != nil {
		s.fail(c, err)
		return
	}
	if err := s.repo.SetStravaToken(ctx, uid, tokJSON); err != nil {
		s.fail(c, err)
		return
	}
	s.sess.set(c.Writer, uid)
	c.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) logout(c *gin.Context) {
	s.sess.clear(c.Writer, cookieName)
	c.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) deleteAccount(c *gin.Context) {
	userID, ok := s.user(c)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	if err := s.repo.DeleteUser(c.Request.Context(), userID); err != nil {
		s.fail(c, err)
		return
	}
	s.sess.clear(c.Writer, cookieName)
	c.Redirect(http.StatusSeeOther, "/")
}
