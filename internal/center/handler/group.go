package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// GroupHandler 设备分组 API 处理器
type GroupHandler struct {
	DB      *gorm.DB
	AgentWS *AgentWSHandler
}

var groupLog = log.New("group_handler")

// NewGroupHandler 创建分组处理器
func NewGroupHandler(db *gorm.DB, agentWS *AgentWSHandler) *GroupHandler {
	return &GroupHandler{
		DB:      db,
		AgentWS: agentWS,
	}
}

// ListGroups 分组列表
// GET /api/groups
func (h *GroupHandler) ListGroups(c *gin.Context) {
	var groups []model.DeviceGroup
	if err := h.DB.Order("created_at desc").Find(&groups).Error; err != nil {
		groupLog.Error("failed to list groups", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// GetGroup 分组详情
// GET /api/groups/:id
func (h *GroupHandler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	var group model.DeviceGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var members []model.DeviceGroupMember
	h.DB.Where("group_id = ?", group.ID).Find(&members)

	// Also fetch device details
	type memberDevice struct {
		model.DeviceGroupMember
		Device model.Device `json:"device,omitempty"`
	}
	memberDevices := make([]memberDevice, 0, len(members))
	for _, m := range members {
		md := memberDevice{DeviceGroupMember: m}
		h.DB.Where("device_id = ?", m.DeviceID).First(&md.Device)
		memberDevices = append(memberDevices, md)
	}

	c.JSON(http.StatusOK, gin.H{
		"group":   group,
		"members": memberDevices,
	})
}

// CreateGroup 创建分组
// POST /api/groups
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Desc string `json:"desc"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := model.DeviceGroup{
		Name:      req.Name,
		Desc:      req.Desc,
		CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&group).Error; err != nil {
		groupLog.Error("failed to create group", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	groupLog.Info("group created", log.String("name", req.Name))
	c.JSON(http.StatusCreated, group)
}

// UpdateGroup 更新分组
// PUT /api/groups/:id
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var group model.DeviceGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var req struct {
		Name string `json:"name"`
		Desc string `json:"desc"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Desc != "" {
		updates["desc"] = req.Desc
	}

	if err := h.DB.Model(&group).Updates(updates).Error; err != nil {
		groupLog.Error("failed to update group", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup 删除分组
// DELETE /api/groups/:id
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	// Delete members first
	h.DB.Where("group_id = ?", id).Delete(&model.DeviceGroupMember{})
	// Delete group
	if err := h.DB.Delete(&model.DeviceGroup{}, id).Error; err != nil {
		groupLog.Error("failed to delete group", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AddMember 添加组成员
// POST /api/groups/:id/members
func (h *GroupHandler) AddMember(c *gin.Context) {
	id := c.Param("id")
	var group model.DeviceGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var req struct {
		DeviceID string `json:"device_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if device exists
	var device model.Device
	if err := h.DB.Where("device_id = ?", req.DeviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	// Check if already a member
	var count int64
	h.DB.Model(&model.DeviceGroupMember{}).Where("group_id = ? AND device_id = ?", group.ID, req.DeviceID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "device already in group"})
		return
	}

	member := model.DeviceGroupMember{
		GroupID:  group.ID,
		DeviceID: req.DeviceID,
	}
	if err := h.DB.Create(&member).Error; err != nil {
		groupLog.Error("failed to add member", log.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	groupLog.Info("member added to group",
		log.String("group", group.Name),
		log.String("device_id", req.DeviceID),
	)
	c.JSON(http.StatusCreated, member)
}

// RemoveMember 移除组成员
// DELETE /api/groups/:id/members/:device_id
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	id := c.Param("id")
	deviceID := c.Param("device_id")

	result := h.DB.Where("group_id = ? AND device_id = ?", id, deviceID).Delete(&model.DeviceGroupMember{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ExecOnGroup 对组内所有设备执行命令
// POST /api/groups/:id/exec
func (h *GroupHandler) ExecOnGroup(c *gin.Context) {
	id := c.Param("id")
	var group model.DeviceGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	var req struct {
		Command string `json:"command" binding:"required"`
		Timeout int    `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// Get all members
	var members []model.DeviceGroupMember
	h.DB.Where("group_id = ?", group.ID).Find(&members)

	type execResult struct {
		DeviceID string `json:"device_id"`
		NodeID   string `json:"node_id"`
		CmdID    string `json:"cmd_id,omitempty"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
	}

	results := make([]execResult, 0, len(members))
	for _, m := range members {
		er := execResult{DeviceID: m.DeviceID}

		var device model.Device
		if err := h.DB.Where("device_id = ?", m.DeviceID).First(&device).Error; err != nil {
			er.Status = "failed"
			er.Error = "device not found"
			results = append(results, er)
			continue
		}
		er.NodeID = device.NodeID

		if device.NodeID == "" {
			er.Status = "failed"
			er.Error = "no managing node"
			results = append(results, er)
			continue
		}

		if !h.AgentWS.IsNodeConnected(device.NodeID) {
			er.Status = "failed"
			er.Error = "node not connected"
			results = append(results, er)
			continue
		}

		cmdID, err := h.AgentWS.SendExecCommand(device.NodeID, req.Command, req.Timeout)
		if err != nil {
			er.Status = "failed"
			er.Error = err.Error()
		} else {
			er.CmdID = cmdID
			er.Status = "pending"
		}
		results = append(results, er)
	}

	// Record audit log
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if err := h.DB.Create(&model.AuditLog{
		User:   usernameStr,
		Action: "group_exec",
		Target: fmt.Sprintf("group:%s", group.Name),
		Detail: fmt.Sprintf("command: %s, members: %d", req.Command, len(members)),
	}).Error; err != nil {
		groupLog.Warn("failed to create audit log", log.Err(err))
	}

	groupLog.Info("group exec initiated",
		log.String("group", group.Name),
		log.Int("members", len(members)),
	)

	c.JSON(http.StatusOK, gin.H{
		"group":   group.Name,
		"results": results,
	})
}
