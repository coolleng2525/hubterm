package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// DeviceMgmtHandler 设备管理 API 处理器
type DeviceMgmtHandler struct {
	DB *gorm.DB
}

var devMgmtLog = log.New("device_mgmt")

// NewDeviceMgmtHandler 创建设备管理处理器
func NewDeviceMgmtHandler(db *gorm.DB) *DeviceMgmtHandler {
	return &DeviceMgmtHandler{DB: db}
}

// List 设备列表（支持 type/tag/status 过滤）
// GET /api/devices
func (h *DeviceMgmtHandler) List(c *gin.Context) {
	query := h.DB.Model(&model.Device{})

	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if tag := c.Query("tag"); tag != "" {
		query = query.Where("tags LIKE ?", "%"+tag+"%")
	}

	var devices []model.Device
	if err := query.Order("updated_at desc").Find(&devices).Error; err != nil {
		devMgmtLog.Error("failed to list devices", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, devices)
}

// Create 创建设备
// POST /api/devices
func (h *DeviceMgmtHandler) Create(c *gin.Context) {
	var req struct {
		DeviceID     string   `json:"device_id" binding:"required"`
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		IP           string   `json:"ip"`
		NodeID       string   `json:"node_id"`
		Protocol     string   `json:"protocol"`
		PortName     string   `json:"port_name"`
		Status       string   `json:"status"`
		Capabilities []string `json:"capabilities"`
		Location     string   `json:"location"`
		Tags         []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	capsJSON := "[]"
	if len(req.Capabilities) > 0 {
		b, _ := json.Marshal(req.Capabilities)
		capsJSON = string(b)
	}
	tagsJSON := "[]"
	if len(req.Tags) > 0 {
		b, _ := json.Marshal(req.Tags)
		tagsJSON = string(b)
	}

	status := req.Status
	if status == "" {
		status = "offline"
	}

	device := model.Device{
		DeviceID:     req.DeviceID,
		Name:         req.Name,
		Type:         req.Type,
		IP:           req.IP,
		NodeID:       req.NodeID,
		Protocol:     req.Protocol,
		PortName:     req.PortName,
		Status:       status,
		Capabilities: capsJSON,
		Location:     req.Location,
		Tags:         tagsJSON,
		LastSeen:     time.Now(),
	}
	if err := h.DB.Create(&device).Error; err != nil {
		devMgmtLog.Error("failed to create device", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	devMgmtLog.Info("device created",
		log.String("device_id", req.DeviceID),
		log.String("name", req.Name),
	)
	c.JSON(http.StatusCreated, device)
}

// Update 更新设备
// PUT /api/devices/:id
func (h *DeviceMgmtHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var device model.Device
	if err := h.DB.Where("device_id = ? OR id = ?", id, id).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		IP       string `json:"ip"`
		NodeID   string `json:"node_id"`
		Protocol string `json:"protocol"`
		PortName string `json:"port_name"`
		Status   string `json:"status"`
		Location string `json:"location"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.IP != "" {
		updates["ip"] = req.IP
	}
	if req.NodeID != "" {
		updates["node_id"] = req.NodeID
	}
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
	}
	if req.PortName != "" {
		updates["port_name"] = req.PortName
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	updates["updated_at"] = time.Now()

	if err := h.DB.Model(&device).Updates(updates).Error; err != nil {
		devMgmtLog.Error("failed to update device", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Reload to return updated record
	h.DB.First(&device, device.ID)
	c.JSON(http.StatusOK, device)
}

// Delete 删除设备
// DELETE /api/devices/:id
func (h *DeviceMgmtHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	result := h.DB.Where("device_id = ? OR id = ?", id, id).Delete(&model.Device{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateTags 更新标签
// PATCH /api/devices/:id/tags
func (h *DeviceMgmtHandler) UpdateTags(c *gin.Context) {
	id := c.Param("id")
	var device model.Device
	if err := h.DB.Where("device_id = ? OR id = ?", id, id).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req struct {
		Tags []string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	if err := h.DB.Model(&device).Update("tags", string(tagsJSON)).Error; err != nil {
		devMgmtLog.Error("failed to update tags", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tags": req.Tags})
}

// UpdateCapabilities 更新能力
// PATCH /api/devices/:id/capabilities
func (h *DeviceMgmtHandler) UpdateCapabilities(c *gin.Context) {
	id := c.Param("id")
	var device model.Device
	if err := h.DB.Where("device_id = ? OR id = ?", id, id).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	var req struct {
		Capabilities []string `json:"capabilities" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	capsJSON, _ := json.Marshal(req.Capabilities)
	if err := h.DB.Model(&device).Update("capabilities", string(capsJSON)).Error; err != nil {
		devMgmtLog.Error("failed to update capabilities", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "capabilities": req.Capabilities})
}

// generateDeviceID creates a unique device ID if not provided
func generateDeviceID() string {
	return uuid.New().String()[:8]
}
