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
	r.GET("/webhook/strava", s.webhookVerify)
	r.POST("/webhook/strava", s.webhookEvent)
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

func (s *Server) root(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if _, ok := s.user(c); ok {
		c.String(http.StatusOK, `<!doctype html><title>commuter</title><p>Connected.</p><form method="post" action="/logout"><button>Log out</button></form>`)
		return
	}
	c.String(http.StatusOK, `<!doctype html><title>commuter</title><form method="post" action="/connect"><button>Connect Strava</button></form>`)
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
