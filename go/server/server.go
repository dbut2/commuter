package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/go/processor"
	"dbut.dev/commuter/go/store"
	"dbut.dev/commuter/go/strava"
)

type Server struct {
	repo         store.Repo
	auth         *strava.Auth
	sess         *sessions
	engine       *gin.Engine
	webhookToken string
	proc         *processor.Processor
}

func New(repo store.Repo, auth *strava.Auth, proc *processor.Processor, sessionSecret []byte, webhookToken string) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		repo:         repo,
		auth:         auth,
		proc:         proc,
		sess:         newSessions(sessionSecret),
		webhookToken: webhookToken,
		engine:       gin.New(),
	}
	s.engine.Use(gin.Recovery())
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.engine.ServeHTTP(w, r) }

func (s *Server) routes() {
	r := s.engine
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
	setStateCookie(c.Writer, state)
	c.Redirect(http.StatusSeeOther, s.auth.AuthURL(state))
}

func (s *Server) stravaCallback(c *gin.Context) {
	ctx := c.Request.Context()
	st, err := c.Cookie(stateCookie)
	if err != nil || st == "" || st != c.Query("state") {
		c.String(http.StatusBadRequest, "invalid oauth state")
		return
	}
	clearCookie(c.Writer, stateCookie)
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
	clearCookie(c.Writer, cookieName)
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
	clearCookie(c.Writer, cookieName)
	c.Redirect(http.StatusSeeOther, "/")
}
