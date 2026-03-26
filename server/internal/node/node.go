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
	// 模拟节点数据 - 实际应从数据库或配置中心获取
	mockNodes = []models.Node{
		{
			ID:       "hk-01",
			Region:   "HK",
			Priority: 1,
			Host:     "hk-01.hqts.cn",
			Port:     443,
			Protocol: "vmess",
			UUID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			AlterId:  4,
			TLS:      true,
		},
		{
			ID:       "hk-02",
			Region:   "HK",
			Priority: 2,
			Host:     "hk-02.hqts.cn",
			Port:     443,
			Protocol: "vless",
			UUID:     "b2c3d4e5-f6a7-8901-bcde-f12345678901",
			Flow:     "xtls-rprx-vision",
			TLS:      true,
		},
		{
			ID:       "sg-01",
			Region:   "SG",
			Priority: 3,
			Host:     "sg-01.hqts.cn",
			Port:     443,
			Protocol: "trojan",
			Password: "trojan-password-123",
			TLS:      true,
		},
		{
			ID:       "us-01",
			Region:   "US",
			Priority: 4,
			Host:     "us-01.hqts.cn",
			Port:     443,
			Protocol: "vmess",
			UUID:     "c3d4e5f6-a7b8-9012-cdef-123456789012",
			AlterId:  4,
			TLS:      true,
		},
	}

	// 节点健康状态
	nodeHealth = map[string]*models.NodeHealth{
		"hk-01": {NodeID: "hk-01", Online: true, Latency: 15, Load: 35.5, Bandwidth: "100GB"},
		"hk-02": {NodeID: "hk-02", Online: true, Latency: 18, Load: 42.1, Bandwidth: "80GB"},
		"sg-01": {NodeID: "sg-01", Online: true, Latency: 35, Load: 55.2, Bandwidth: "60GB"},
		"us-01": {NodeID: "us-01", Online: true, Latency: 150, Load: 20.0, Bandwidth: "200GB"},
	}

	healthMu sync.RWMutex
)

/**
 * HandleListNodes 获取节点列表
 * GET /api/v1/nodes
 */
func HandleListNodes(c *gin.Context) {
	// 返回所有可用节点
	nodes := getAvailableNodes()
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
