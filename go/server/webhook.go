package server

import (
	"context"
	"fmt"
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
	ObjectType string         `json:"object_type"`
	AspectType string         `json:"aspect_type"`
	ObjectID   int64          `json:"object_id"`
	OwnerID    int64          `json:"owner_id"`
	Updates    map[string]any `json:"updates"`
}

func (s *Server) webhookEvent(c *gin.Context) {
	var ev stravaEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.Status(http.StatusOK)
		return
	}
	if err := s.handleEvent(c.Request.Context(), ev); err != nil {
		log.Printf("commuter: webhook %s %s %d: %v", ev.ObjectType, ev.AspectType, ev.ObjectID, err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (s *Server) handleEvent(ctx context.Context, ev stravaEvent) error {
	userID, err := s.repo.UserIDByStrava(ctx, ev.OwnerID)
	if err != nil || userID == "" {
		return err
	}
	switch {
	case ev.ObjectType == "athlete" && fmt.Sprint(ev.Updates["authorized"]) == "false":
		return s.repo.DeleteUser(ctx, userID)
	case ev.ObjectType == "activity" && ev.AspectType == "create":
		return s.proc.Enqueue(ctx, userID, ev.ObjectID)
	case ev.ObjectType == "activity" && ev.AspectType == "update":
		return s.proc.EnqueueRefresh(ctx, userID, ev.ObjectID)
	}
	return nil
}
