package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

func (h *NodeHandler) StartLocalShell(c *gin.Context) {
	var req struct {
		Shell string `json:"shell"`
		Rows  int    `json:"rows"`
		Cols  int    `json:"cols"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Shell == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shell is required"})
		return
	}
	sessionID := uuid.New().String()
	if err := h.AgentWS.StartLocalShell(c.Param("id"), req.Shell, sessionID, req.Rows, req.Cols); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}

func (h *NodeHandler) CloseLocalShell(c *gin.Context) {
	if err := h.AgentWS.CloseLocalShell(c.Param("id"), c.Param("session_id")); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
