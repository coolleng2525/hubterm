// Package service provides business logic services for the HubTerm center.
package service

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/coolleng2525/hubterm/internal/center/model"
	"github.com/coolleng2525/hubterm/internal/pkg/log"
)

// RouteHop 路由跳
type RouteHop struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Via      string `json:"via,omitempty"`
	Protocol string `json:"protocol"`
}

// TopologyNode 拓扑中的节点
type TopologyNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Status   string `json:"status"`
	Group    string `json:"group"`
	Hostname string `json:"hostname,omitempty"`
	OS       string `json:"os,omitempty"`
}

// TopologyEdge 拓扑中的边
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// Topology 完整拓扑
type Topology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// NetworkInfo 解析后的网卡信息
type NetworkInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// TopologyService 拓扑发现与路由服务
type TopologyService struct {
	DB *gorm.DB
}

var topoLog = log.New("topology")

// NewTopologyService 创建拓扑服务
func NewTopologyService(db *gorm.DB) *TopologyService {
	return &TopologyService{DB: db}
}

// GetTopology 返回所有节点间的连接关系（基于相同网段）
func (s *TopologyService) GetTopology() *Topology {
	var nodes []model.Node
	s.DB.Find(&nodes)

	topo := &Topology{
		Nodes: make([]TopologyNode, 0, len(nodes)),
		Edges: make([]TopologyEdge, 0),
	}

	// Build node map and node list
	nodeMap := make(map[string]model.Node)
	for _, n := range nodes {
		nodeMap[n.NodeID] = n
		group := "default"
		if n.Status == "online" {
			group = "online"
		} else {
			group = "offline"
		}
		topo.Nodes = append(topo.Nodes, TopologyNode{
			ID:       n.NodeID,
			Name:     n.Name,
			IP:       n.IP,
			Status:   n.Status,
			Group:    group,
			Hostname: n.Hostname,
			OS:       n.OS,
		})
	}

	// Build edges based on shared subnet
	for i := 0; i < len(nodes); i++ {
		networksA := s.parseInterfaces(nodes[i].Interfaces)
		for j := i + 1; j < len(nodes); j++ {
			networksB := s.parseInterfaces(nodes[j].Interfaces)
			for _, na := range networksA {
				for _, nb := range networksB {
					if sameSubnet(na.IP, nb.IP) {
						topo.Edges = append(topo.Edges, TopologyEdge{
							Source: nodes[i].NodeID,
							Target: nodes[j].NodeID,
							Label:  na.Name + "/" + nb.Name,
						})
					}
				}
			}
		}
	}

	return topo
}

// GetNodeReachability 返回某节点可达的其他节点
func (s *TopologyService) GetNodeReachability(nodeID string) []TopologyNode {
	topo := s.GetTopology()
	reachable := make(map[string]bool)

	// Find all nodes connected to this node
	for _, edge := range topo.Edges {
		if edge.Source == nodeID {
			reachable[edge.Target] = true
		}
		if edge.Target == nodeID {
			reachable[edge.Source] = true
		}
	}

	var result []TopologyNode
	for _, n := range topo.Nodes {
		if reachable[n.ID] {
			result = append(result, n)
		}
	}
	return result
}

// FindRoute BFS 找最短路径
func (s *TopologyService) FindRoute(from, to string) []RouteHop {
	topo := s.GetTopology()

	// Build adjacency list
	adj := make(map[string][]string)
	for _, edge := range topo.Edges {
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
		adj[edge.Target] = append(adj[edge.Target], edge.Source)
	}

	// BFS
	visited := make(map[string]bool)
	prev := make(map[string]string)
	queue := []string{from}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == to {
			break
		}
		for _, neighbor := range adj[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				prev[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	// Reconstruct path
	if _, ok := prev[to]; !ok && from != to {
		return nil
	}

	var path []string
	for at := to; at != ""; at = prev[at] {
		path = append([]string{at}, path...)
	}

	// Convert to RouteHop
	var hops []RouteHop
	for i := 0; i < len(path)-1; i++ {
		hop := RouteHop{
			From:     path[i],
			To:       path[i+1],
			Protocol: "lan",
		}
		// Find the edge label for protocol hint
		for _, edge := range topo.Edges {
			if (edge.Source == path[i] && edge.Target == path[i+1]) ||
				(edge.Source == path[i+1] && edge.Target == path[i]) {
				hop.Via = edge.Label
				break
			}
		}
		hops = append(hops, hop)
	}

	return hops
}

// CheckHealth 检测离线节点，标记为 offline
func (s *TopologyService) CheckHealth() map[string]string {
	results := make(map[string]string)
	var nodes []model.Node
	s.DB.Find(&nodes)

	now := time.Now()
	timeout := 5 * time.Minute

	for _, n := range nodes {
		if now.Sub(n.LastSeen) > timeout {
			if n.Status != "offline" {
				s.DB.Model(&n).Update("status", "offline")
				topoLog.Info("node marked offline by health check",
					log.String("node_id", n.NodeID),
					log.String("last_seen", n.LastSeen.Format(time.RFC3339)),
				)
				results[n.NodeID] = "marked_offline"
			} else {
				results[n.NodeID] = "already_offline"
			}
		} else {
			if n.Status == "offline" {
				s.DB.Model(&n).Update("status", "online")
				topoLog.Info("node recovered",
					log.String("node_id", n.NodeID),
				)
				results[n.NodeID] = "recovered"
			} else {
				results[n.NodeID] = "healthy"
			}
		}
	}
	return results
}

// GetAffectedNodes 某节点离线影响哪些节点
func (s *TopologyService) GetAffectedNodes(nodeID string) []TopologyNode {
	topo := s.GetTopology()

	// BFS from nodeID, find all nodes that depend on it
	adj := make(map[string][]string)
	for _, edge := range topo.Edges {
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
		adj[edge.Target] = append(adj[edge.Target], edge.Source)
	}

	visited := make(map[string]bool)
	queue := []string{nodeID}
	visited[nodeID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range adj[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	// Find affected nodes that are only reachable through this node
	// For simplicity, return all nodes that are in the same connected component
	// but not directly connected (more than 1 hop away)
	var affected []TopologyNode
	for _, n := range topo.Nodes {
		if n.ID == nodeID {
			continue
		}
		if visited[n.ID] {
			// Check if there's an alternative path
			pathWithout := s.findRouteWithout(topo, nodeID, n.ID)
			if pathWithout == nil {
				affected = append(affected, n)
			}
		}
	}
	return affected
}

// findRouteWithout 检查在排除某节点后是否仍有路径
func (s *TopologyService) findRouteWithout(topo *Topology, excludeID, targetID string) []string {
	// Build adjacency excluding the given node
	adj := make(map[string][]string)
	for _, edge := range topo.Edges {
		if edge.Source == excludeID || edge.Target == excludeID {
			continue
		}
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
		adj[edge.Target] = append(adj[edge.Target], edge.Source)
	}

	// Find a start node that is not the excluded one
	var start string
	for _, n := range topo.Nodes {
		if n.ID != excludeID {
			start = n.ID
			break
		}
	}
	if start == "" {
		return nil
	}

	// BFS
	visited := make(map[string]bool)
	prev := make(map[string]string)
	queue := []string{start}
	visited[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == targetID {
			break
		}
		for _, neighbor := range adj[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				prev[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	if _, ok := prev[targetID]; !ok && start != targetID {
		return nil
	}

	var path []string
	for at := targetID; at != ""; at = prev[at] {
		path = append([]string{at}, path...)
	}
	// Reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// GetGraphData 返回 D3.js/vis.js 兼容的图数据
func (s *TopologyService) GetGraphData() ginH {
	topo := s.GetTopology()
	nodes := make([]ginH, 0, len(topo.Nodes))
	for _, n := range topo.Nodes {
		nodes = append(nodes, ginH{
			"id":     n.ID,
			"name":   n.Name,
			"group":  n.Group,
			"ip":     n.IP,
			"status": n.Status,
		})
	}
	edges := make([]ginH, 0, len(topo.Edges))
	for _, e := range topo.Edges {
		edges = append(edges, ginH{
			"source": e.Source,
			"target": e.Target,
			"label":  e.Label,
		})
	}
	return ginH{"nodes": nodes, "edges": edges}
}

// parseInterfaces 解析节点网卡 JSON
func (s *TopologyService) parseInterfaces(ifaceJSON string) []NetworkInfo {
	if ifaceJSON == "" {
		return nil
	}
	var infos []NetworkInfo
	if err := json.Unmarshal([]byte(ifaceJSON), &infos); err != nil {
		topoLog.Warn("failed to parse interfaces", log.Err(err))
		return nil
	}
	return infos
}

// sameSubnet 判断两个 IP 是否在同一网段（简单实现：比较前三个八位）
func sameSubnet(ip1, ip2 string) bool {
	if ip1 == "" || ip2 == "" {
		return false
	}
	parts1 := strings.Split(ip1, ".")
	parts2 := strings.Split(ip2, ".")
	if len(parts1) < 3 || len(parts2) < 3 {
		return false
	}
	return parts1[0] == parts2[0] && parts1[1] == parts2[1] && parts1[2] == parts2[2]
}

// ginH is a shorthand for gin.H to avoid importing gin in service layer
type ginH map[string]interface{}

// SortTopologyNodes sorts nodes by name
func SortTopologyNodes(nodes []TopologyNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
}
