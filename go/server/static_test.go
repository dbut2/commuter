package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"dbut.dev/commuter/web"
)

func TestStaticAssetsServedAtStaticPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.StaticFS("/static", http.FS(web.Static))
	for _, p := range []string{
		"/static/btn_strava_connect_with_orange.svg",
		"/static/api_logo_pwrdBy_strava_horiz_black.svg",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", p, rec.Code)
		}
	}
}
