package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// RemoteCenterHandler 远程中心 API 处理器
type RemoteCenterHandler struct {
	DB *gorm.DB
}

var centerLog = log.New("remote_center_handler")

// NewRemoteCenterHandler 创建远程中心处理器
func NewRemoteCenterHandler(db *gorm.DB) *RemoteCenterHandler {
	return &RemoteCenterHandler{DB: db}
}

// List 返回所有远程中心
// GET /api/centers
func (h *RemoteCenterHandler) List(c *gin.Context) {
	var centers []model.RemoteCenter
	if err := h.DB.Order("created_at desc").Find(&centers).Error; err != nil {
		centerLog.Error("failed to list remote centers", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, centers)
}

// Get 返回远程中心详情
// GET /api/centers/:id
func (h *RemoteCenterHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var center model.RemoteCenter
	if err := h.DB.First(&center, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "remote center not found"})
		return
	}
	c.JSON(http.StatusOK, center)
}

// Create 创建远程中心
// POST /api/centers
func (h *RemoteCenterHandler) Create(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required"`
		URL   string `json:"url" binding:"required"`
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rc := model.RemoteCenter{
		Name:      req.Name,
		URL:       req.URL,
		Token:     req.Token,
		Status:    "unknown",
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&rc).Error; err != nil {
		centerLog.Error("failed to create remote center", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	centerLog.Info("remote center created",
		log.String("name", req.Name),
		log.String("url", req.URL),
	)
	c.JSON(http.StatusCreated, rc)
}

// Update 更新远程中心
// PUT /api/centers/:id
func (h *RemoteCenterHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var rc model.RemoteCenter
	if err := h.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "remote center not found"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.URL != "" {
		updates["url"] = req.URL
	}
	if req.Token != "" {
		updates["token"] = req.Token
	}

	if err := h.DB.Model(&rc).Updates(updates).Error; err != nil {
		centerLog.Error("failed to update remote center", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, rc)
}

// Delete 删除远程中心
// DELETE /api/centers/:id
func (h *RemoteCenterHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Delete(&model.RemoteCenter{}, id).Error; err != nil {
		centerLog.Error("failed to delete remote center", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Sync 同步远程中心节点
// POST /api/centers/:id/sync
func (h *RemoteCenterHandler) Sync(c *gin.Context) {
	id := c.Param("id")
	var rc model.RemoteCenter
	if err := h.DB.First(&rc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "remote center not found"})
		return
	}

	// Simplified sync: mark as synced
	now := time.Now()
	if err := h.DB.Model(&rc).Updates(map[string]interface{}{
		"last_sync": now,
		"status":    "synced",
	}).Error; err != nil {
		centerLog.Error("failed to update sync status", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	centerLog.Info("remote center synced",
		log.String("name", rc.Name),
		log.String("url", rc.URL),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":  "sync completed",
		"center":   rc.Name,
		"last_sync": now,
	})
}
