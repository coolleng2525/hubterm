package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/coolleng2525/hubterm/internal/center/model"
)

type SerialPortHandler struct {
	DB *gorm.DB
}

func (h *SerialPortHandler) List(c *gin.Context) {
	nodeID := c.Query("node_id")
	query := h.DB.Model(&model.SerialPort{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	var ports []model.SerialPort
	query.Order("node_id, port_name").Find(&ports)
	c.JSON(http.StatusOK, ports)
}
