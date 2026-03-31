package config

import (
	"log"
	"net/http"
	"sync"

	"hqts-secure-access-server/internal/auth"
	"hqts-secure-access-server/internal/node"
	"hqts-secure-access-server/internal/policy"
	"hqts-secure-access-server/pkg/models"

	"github.com/gin-gonic/gin"
)

/**
 * Config Center - 配置中心
 * 
 * 职责：
 * - 下发节点列表
 * - 下发用户策略
 * - 功能开关
 * - 配置版本管理
 */

var (
	configVersion = "v1.0.0"
	configMu      sync.RWMutex
)

const (
	singboxVersion     = "1.9.4"
	singboxDownloadURL = "https://github.com/SagerNet/sing-box/releases/download/v1.9.4/sing-box-1.9.4-windows-amd64.zip"
)

// 获取 sing-box 下载地址
func getSingboxDownloadURL() string {
	return singboxDownloadURL
}

/**
 * HandleGetConfig 获取用户配置
 * GET /api/v1/config
 * Authorization: Bearer <token>
 */
func HandleGetConfig(c *gin.Context) {
	// 获取当前用户
	userID := auth.GetCurrentUserID(c)
	user := auth.GetCurrentUser(c)

	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	log.Printf("Fetching config for user: %s", userID)

	// 获取用户策略
	userPolicy := policy.GetUserPolicy(userID, user.Group)

	// 获取可用节点
	nodes := node.GetAvailableNodes()

	// 构建配置
	productConfig := models.ProductConfig{
		Version:    configVersion,
		User:       *user,
		Mode:       userPolicy.DefaultMode,
		DNSPolicy:  "STANDARD",
		RoutePolicy: userPolicy.DefaultMode,
		Nodes:      nodes,
		Features: models.Features{
			AllowModeSwitch: contains(userPolicy.AllowedModes, "GLOBAL") && contains(userPolicy.AllowedModes, "BYPASS_CN"),
			ForceMode:       nil, // 不强制模式
		},
		DNSServers: []models.DNSPolicy{
			{Server: "1.1.1.1", Port: 53, Protocol: "doh"},
			{Server: "8.8.8.8", Port: 53, Protocol: "doh"},
		},
		Singbox: models.SingboxConfig{
			Version:     "1.9.4",
			DownloadURL: getSingboxDownloadURL(),
		},
	}

	// ETag支持
	configHash := hashConfig(productConfig)
	c.Header("ETag", configHash)
	c.Header("Cache-Control", "private, max-age=300")

	// 检查If-None-Match
	if c.GetHeader("If-None-Match") == configHash {
		c.Status(http.StatusNotModified)
		return
	}

	c.JSON(http.StatusOK, productConfig)
}

// 简单的配置哈希
func hashConfig(config models.ProductConfig) string {
	// 简化实现，实际应使用更健壮的哈希
	result := len(config.Nodes) * 100
	for _, n := range config.Nodes {
		result += len(n.ID) + n.Priority
	}
	return hashItoa(result)
}

// 简单哈希
func hashItoa(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[i] = chars[n%36]
		n /= 36
	}
	return string(b)
}

// 检查slice是否包含某个元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
