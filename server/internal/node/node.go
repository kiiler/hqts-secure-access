package node

import (
	"sync"
	"time"

	"hqts-secure-access-server/pkg/models"

	"github.com/gin-gonic/gin"
)

/**
 * Node Directory - 节点目录
 * 
 * 职责：
 * - 区域节点管理
 * - 权重和优先级
 * - 健康状态
 * - 灰度控制
 */

var (
	// 节点数据 - 来自 HQTS 真实节点配置
	mockNodes = []models.Node{
		// Shadowsocks 节点 (c73s1)
		{
			ID:       "c73s1",
			Region:   "SG",
			Priority: 1,
			Host:     "c73s1.portablesubmarines.com",
			Port:     5079,
			Protocol: "shadowsocks",
			Password: "Ka8GnmuJL3WwfWSw",
			Method:   "aes-256-gcm",
		},
		// Shadowsocks 节点 (c73s2)
		{
			ID:       "c73s2",
			Region:   "SG",
			Priority: 2,
			Host:     "c73s2.portablesubmarines.com",
			Port:     5079,
			Protocol: "shadowsocks",
			Password: "Ka8GnmuJL3WwfWSw",
			Method:   "aes-256-gcm",
		},
		// VMess 节点 (c73s3)
		{
			ID:       "c73s3",
			Region:   "SG",
			Priority: 3,
			Host:     "c73s3.portablesubmarines.com",
			Port:     5079,
			Protocol: "vmess",
			UUID:     "badb8e19-e0e8-4bf7-8ce3-f60f59a379e7",
			AlterId:  0,
			TLS:      false,
		},
		// VMess 节点 (c73s4)
		{
			ID:       "c73s4",
			Region:   "SG",
			Priority: 4,
			Host:     "c73s4.portablesubmarines.com",
			Port:     5079,
			Protocol: "vmess",
			UUID:     "badb8e19-e0e8-4bf7-8ce3-f60f59a379e7",
			AlterId:  0,
			TLS:      false,
		},
		// VMess 节点 (c73s5)
		{
			ID:       "c73s5",
			Region:   "SG",
			Priority: 5,
			Host:     "c73s5.portablesubmarines.com",
			Port:     5079,
			Protocol: "vmess",
			UUID:     "badb8e19-e0e8-4bf7-8ce3-f60f59a379e7",
			AlterId:  0,
			TLS:      false,
		},
		// VMess 节点 (c73s4801)
		{
			ID:       "c73s4801",
			Region:   "SG",
			Priority: 6,
			Host:     "c73s4801.portablesubmarines.com",
			Port:     5079,
			Protocol: "vmess",
			UUID:     "badb8e19-e0e8-4bf7-8ce3-f60f59a379e7",
			AlterId:  0,
			TLS:      false,
		},
	}

	// 节点健康状态
	nodeHealth = map[string]*models.NodeHealth{
		"c73s1":    {NodeID: "c73s1",    Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
		"c73s2":    {NodeID: "c73s2",    Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
		"c73s3":    {NodeID: "c73s3",    Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
		"c73s4":    {NodeID: "c73s4",    Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
		"c73s5":    {NodeID: "c73s5",    Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
		"c73s4801": {NodeID: "c73s4801", Online: true, Latency: 0, Load: 0, Bandwidth: "unknown"},
	}

	healthMu sync.RWMutex
)

/**
 * HandleListNodes 获取节点列表
 * GET /api/v1/nodes
 */
func HandleListNodes(c *gin.Context) {
	// 返回所有可用节点
	nodes := GetAvailableNodes()
	c.JSON(200, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

/**
 * HandleHealth 获取节点健康状态
 * GET /api/v1/nodes/health
 */
func HandleHealth(c *gin.Context) {
	healthMu.RLock()
	defer healthMu.RUnlock()

	healthList := make([]models.NodeHealth, 0, len(nodeHealth))
	for _, h := range nodeHealth {
		healthList = append(healthList, *h)
	}

	c.JSON(200, gin.H{
		"health": healthList,
		"checked_at": time.Now().Unix(),
	})
}

/**
 * 获取所有可用节点
 */
func GetAvailableNodes() []models.Node {
	// 实际应从数据库读取，并检查灰度策略
	return mockNodes
}

/**
 * 获取指定节点
 */
func GetNode(nodeID string) *models.Node {
	for _, n := range mockNodes {
		if n.ID == nodeID {
			return &n
		}
	}
	return nil
}

/**
 * 获取节点健康状态
 */
func GetNodeHealth(nodeID string) *models.NodeHealth {
	healthMu.RLock()
	defer healthMu.RUnlock()
	return nodeHealth[nodeID]
}

/**
 * 按优先级排序获取节点
 */
func GetNodesSortedByPriority() []models.Node {
	nodes := make([]models.Node, len(mockNodes))
	copy(nodes, mockNodes)

	// 按优先级排序
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Priority < nodes[i].Priority {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	return nodes
}

/**
 * 根据区域获取节点
 */
func GetNodesByRegion(region string) []models.Node {
	var result []models.Node
	for _, n := range mockNodes {
		if n.Region == region {
			result = append(result, n)
		}
	}
	return result
}
