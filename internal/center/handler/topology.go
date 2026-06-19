package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/service"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// TopologyHandler 拓扑 API 处理器
type TopologyHandler struct {
	TopologySvc *service.TopologyService
}

var topoHandlerLog = log.New("topology_handler")

// NewTopologyHandler 创建拓扑处理器
func NewTopologyHandler(db *gorm.DB) *TopologyHandler {
	return &TopologyHandler{
		TopologySvc: service.NewTopologyService(db),
	}
}

// GetTopology 返回完整拓扑
// GET /api/topology
func (h *TopologyHandler) GetTopology(c *gin.Context) {
	topo := h.TopologySvc.GetTopology()
	c.JSON(http.StatusOK, topo)
}

// GetNodeTopology 返回节点拓扑详情
// GET /api/topology/nodes/:id
func (h *TopologyHandler) GetNodeTopology(c *gin.Context) {
	id := c.Param("id")
	reachable := h.TopologySvc.GetNodeReachability(id)
	if reachable == nil {
		reachable = []service.TopologyNode{}
	}
	c.JSON(http.StatusOK, gin.H{
		"node_id":   id,
		"reachable": reachable,
	})
}

// FindRoute 查找路径
// GET /api/topology/route?from=X&to=Y
func (h *TopologyHandler) FindRoute(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query params required"})
		return
	}
	hops := h.TopologySvc.FindRoute(from, to)
	if hops == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no route found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"from":  from,
		"to":    to,
		"hops":  hops,
		"count": len(hops),
	})
}

// CheckHealth 健康状态
// GET /api/topology/health
func (h *TopologyHandler) CheckHealth(c *gin.Context) {
	results := h.TopologySvc.CheckHealth()
	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// Heal 触发自愈
// POST /api/topology/heal
func (h *TopologyHandler) Heal(c *gin.Context) {
	results := h.TopologySvc.CheckHealth()
	topoHandlerLog.Info("heal triggered", log.Int("nodes_checked", len(results)))
	c.JSON(http.StatusOK, gin.H{
		"message": "heal completed",
		"results": results,
	})
}

// GetGraph 返回拓扑可视化数据
// GET /api/topology/graph
func (h *TopologyHandler) GetGraph(c *gin.Context) {
	graph := h.TopologySvc.GetGraphData()
	c.JSON(http.StatusOK, graph)
}
