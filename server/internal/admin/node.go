package admin

/**
 * Node - 节点管理
 */

import (
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/**
 * 节点状态
 */
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusTesting = "testing"
)

/**
 * 节点数据结构
 */
type Node struct {
	ID          string  `json:"id"`
	Region      string  `json:"region"`
	Name        string  `json:"name"`         // 显示名称
	Host        string  `json:"host"`
	Port        int     `json:"port"`
	Protocol    string  `json:"protocol"`      // vmess, vless, trojan, shadowsocks
	Password    string  `json:"password,omitempty"` // 加密存储
	UUID        string  `json:"uuid,omitempty"`
	AlterId     int     `json:"alterId,omitempty"`
	Flow        string  `json:"flow,omitempty"`
	Method      string  `json:"method,omitempty"`
	TLS         bool    `json:"tls"`
	Priority    int     `json:"priority"`      // 1 = 最高
	Status      string  `json:"status"`         // online, offline, testing
	Latency     int     `json:"latency"`        // ms, -1 = 未测试
	Load        float64 `json:"load"`           // 百分比
	Bandwidth   string  `json:"bandwidth"`     // 剩余带宽
	Description string  `json:"description"`    // 备注
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

// 节点存储（实际应该用数据库）
var (
	nodeStore = struct {
		sync.RWMutex
		m map[string]*Node
	}{
		m: make(map[string]*Node),
	}
)

// InitNodes 初始化测试节点
func init() {
	nodes := []*Node{
		{
			ID:          "hk-01",
			Region:      "HK",
			Name:        "香港节点 1",
			Host:        "hk-01.hqts.cn",
			Port:        443,
			Protocol:    "vmess",
			UUID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			AlterId:     4,
			TLS:         true,
			Priority:    1,
			Status:      NodeStatusOnline,
			Latency:     15,
			Load:        35.5,
			Bandwidth:   "100GB",
			Description: "香港主节点",
			CreatedAt:   1700000000,
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "hk-02",
			Region:      "HK",
			Name:        "香港节点 2",
			Host:        "hk-02.hqts.cn",
			Port:        443,
			Protocol:    "vless",
			UUID:        "b2c3d4e5-f6a7-8901-bcde-f12345678901",
			Flow:        "xtls-rprx-vision",
			TLS:         true,
			Priority:    2,
			Status:      NodeStatusOnline,
			Latency:     18,
			Load:        42.1,
			Bandwidth:   "80GB",
			Description: "香港备节点",
			CreatedAt:   1700000000,
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "sg-01",
			Region:      "SG",
			Name:        "新加坡节点",
			Host:        "sg-01.hqts.cn",
			Port:        443,
			Protocol:    "trojan",
			Password:    "trojan-password-123",
			TLS:         true,
			Priority:    3,
			Status:      NodeStatusOnline,
			Latency:     35,
			Load:        55.2,
			Bandwidth:   "60GB",
			Description: "新加坡节点",
			CreatedAt:   1700000000,
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "us-01",
			Region:      "US",
			Name:        "美国节点",
			Host:        "us-01.hqts.cn",
			Port:        443,
			Protocol:    "vmess",
			UUID:        "c3d4e5f6-a7b8-9012-cdef-123456789012",
			AlterId:     4,
			TLS:         true,
			Priority:    4,
			Status:      NodeStatusOffline,
			Latency:     -1,
			Load:        0,
			Bandwidth:   "0GB",
			Description: "美国节点（维护中）",
			CreatedAt:   1700000000,
			UpdatedAt:   time.Now().Unix(),
		},
	}

	for _, n := range nodes {
		nodeStore.m[n.ID] = n
	}
}

// HandleListNodes 获取节点列表
func HandleListNodes(c *gin.Context) {
	region := c.Query("region")
	status := c.Query("status")

	nodeStore.RLock()
	defer nodeStore.RUnlock()

	nodes := make([]*Node, 0)
	for _, n := range nodeStore.m {
		if region != "" && n.Region != region {
			continue
		}
		if status != "" && n.Status != status {
			continue
		}
		nodes = append(nodes, n)
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

// HandleGetNode 获取单个节点
func HandleGetNode(c *gin.Context) {
	nodeID := c.Param("id")

	nodeStore.RLock()
	node, exists := nodeStore.m[nodeID]
	nodeStore.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, node)
}

// HandleCreateNode 创建节点
func HandleCreateNode(c *gin.Context) {
	var node Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查ID是否已存在
	nodeStore.Lock()
	defer nodeStore.Unlock()

	if _, exists := nodeStore.m[node.ID]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "node id already exists"})
		return
	}

	// 设置默认值
	node.Status = NodeStatusOnline
	node.CreatedAt = time.Now().Unix()
	node.UpdatedAt = time.Now().Unix()

	nodeStore.m[node.ID] = &node

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"node":    &node,
	})
}

// HandleUpdateNode 更新节点
func HandleUpdateNode(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Name        string  `json:"name"`
		Host        string  `json:"host"`
		Port        int     `json:"port"`
		Protocol    string  `json:"protocol"`
		Password    string  `json:"password"`
		UUID        string  `json:"uuid"`
		AlterId     int     `json:"alterId"`
		Flow        string  `json:"flow"`
		Method      string  `json:"method"`
		TLS         bool    `json:"tls"`
		Priority    int     `json:"priority"`
		Status      string  `json:"status"`
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodeStore.Lock()
	defer nodeStore.Unlock()

	node, exists := nodeStore.m[nodeID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	// 更新字段
	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Host != "" {
		node.Host = req.Host
	}
	if req.Port > 0 {
		node.Port = req.Port
	}
	if req.Protocol != "" {
		node.Protocol = req.Protocol
	}
	if req.Password != "" {
		node.Password = req.Password
	}
	if req.UUID != "" {
		node.UUID = req.UUID
	}
	if req.AlterId > 0 {
		node.AlterId = req.AlterId
	}
	if req.Flow != "" {
		node.Flow = req.Flow
	}
	if req.Method != "" {
		node.Method = req.Method
	}
	node.TLS = req.TLS
	if req.Priority > 0 {
		node.Priority = req.Priority
	}
	if req.Status != "" {
		node.Status = req.Status
	}
	if req.Description != "" {
		node.Description = req.Description
	}
	node.UpdatedAt = time.Now().Unix()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"node":    node,
	})
}

// HandleDeleteNode 删除节点
func HandleDeleteNode(c *gin.Context) {
	nodeID := c.Param("id")

	nodeStore.Lock()
	defer nodeStore.Unlock()

	if _, exists := nodeStore.m[nodeID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	delete(nodeStore.m, nodeID)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleTestNode 测试节点连通性
func HandleTestNode(c *gin.Context) {
	nodeID := c.Param("id")

	nodeStore.RLock()
	node, exists := nodeStore.m[nodeID]
	nodeStore.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	// 模拟测试（实际应该TCP探测）
	nodeStore.Lock()
	node.Status = NodeStatusTesting
	node.UpdatedAt = time.Now().Unix()
	nodeStore.Unlock()

	// 模拟测试延迟
	go func() {
		time.Sleep(2 * time.Second)

		nodeStore.Lock()
		defer nodeStore.Unlock()

		// 随机模拟结果
		if rand.Float32() > 0.2 { // 80% 成功率
			node.Status = NodeStatusOnline
			node.Latency = 10 + rand.Intn(50)
			node.Load = float64(rand.Intn(80))
		} else {
			node.Status = NodeStatusOffline
			node.Latency = -1
		}
		node.UpdatedAt = time.Now().Unix()
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "node test started",
		"status":  NodeStatusTesting,
	})
}
