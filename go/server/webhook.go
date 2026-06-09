package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) webhookVerify(c *gin.Context) {
	if c.Query("hub.mode") == "subscribe" && s.webhookToken != "" && c.Query("hub.verify_token") == s.webhookToken {
		c.JSON(http.StatusOK, gin.H{"hub.challenge": c.Query("hub.challenge")})
		return
	}
	c.String(http.StatusForbidden, "forbidden")
}

type stravaEvent struct {
	ObjectType string `json:"object_type"`
	AspectType string `json:"aspect_type"`
	ObjectID   int64  `json:"object_id"`
	OwnerID    int64  `json:"owner_id"`
}

func (s *Server) webhookEvent(c *gin.Context) {
	var ev stravaEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.Status(http.StatusOK)
		return
	}
	c.Status(http.StatusOK)
	if ev.ObjectType != "activity" || (ev.AspectType != "create" && ev.AspectType != "update") {
		return
	}
	go func() {
		ctx := context.Background()
		userID, err := s.repo.UserIDByStrava(ctx, ev.OwnerID)
		if err != nil || userID == "" {
			return
		}
		if err := s.proc.Enqueue(ctx, userID, ev.ObjectID); err != nil {
			log.Printf("commuter: webhook enqueue: %v", err)
			return
		}
		s.proc.KickPoll()
	}()
}
