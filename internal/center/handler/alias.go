package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// AliasHandler 虚拟设备名 API 处理器
type AliasHandler struct {
	DB *gorm.DB
}

var aliasLog = log.New("alias_handler")

// NewAliasHandler 创建别名处理器
func NewAliasHandler(db *gorm.DB) *AliasHandler {
	return &AliasHandler{DB: db}
}

// List 返回所有别名
// GET /api/aliases
func (h *AliasHandler) List(c *gin.Context) {
	var aliases []model.DeviceAlias
	if err := h.DB.Order("created_at desc").Find(&aliases).Error; err != nil {
		aliasLog.Error("failed to list aliases", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, aliases)
}

// Create 创建别名
// POST /api/aliases
func (h *AliasHandler) Create(c *gin.Context) {
	var req struct {
		Alias    string `json:"alias" binding:"required"`
		DeviceID string `json:"device_id" binding:"required"`
		NodeID   string `json:"node_id"`
		Protocol string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	alias := model.DeviceAlias{
		Alias:     req.Alias,
		DeviceID:  req.DeviceID,
		NodeID:    req.NodeID,
		Protocol:  req.Protocol,
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&alias).Error; err != nil {
		aliasLog.Error("failed to create alias", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	aliasLog.Info("alias created",
		log.String("alias", req.Alias),
		log.String("device_id", req.DeviceID),
	)
	c.JSON(http.StatusCreated, alias)
}

// Delete 删除别名
// DELETE /api/aliases/:id
func (h *AliasHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Delete(&model.DeviceAlias{}, id).Error; err != nil {
		aliasLog.Error("failed to delete alias", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Resolve 解析 hubterm://xxx 到真实设备
// GET /api/aliases/resolve?alias=hubterm://xxx
func (h *AliasHandler) Resolve(c *gin.Context) {
	aliasName := c.Query("alias")
	var alias model.DeviceAlias
	if err := h.DB.Where("alias = ?", aliasName).First(&alias).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alias not found"})
		return
	}

	// Also return the device info if available
	var device model.Device
	deviceInfo := gin.H{}
	if err := h.DB.Where("device_id = ?", alias.DeviceID).First(&device).Error; err == nil {
		deviceInfo = gin.H{
			"name":     device.Name,
			"type":     device.Type,
			"ip":       device.IP,
			"status":   device.Status,
			"protocol": device.Protocol,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"alias":    alias.Alias,
		"device_id": alias.DeviceID,
		"node_id":  alias.NodeID,
		"protocol": alias.Protocol,
		"device":   deviceInfo,
	})
}
