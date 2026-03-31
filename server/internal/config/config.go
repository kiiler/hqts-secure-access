package config

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	listenAddr   = "localhost:8080" // 服务端监听地址，用于生成singbox下载地址
)

// SetListenAddr 设置服务端监听地址
func SetListenAddr(addr string) {
	listenAddr = addr
}

// GetListenAddr 获取服务端监听地址
func GetListenAddr() string {
	return listenAddr
}

// GetServerConfig 获取当前服务器配置
func GetServerConfig() *ServerConfig {
	return serverConfig
}
type ServerConfig struct {
	Server  ServerInfo `json:"server"`
	CAS     CASInfo   `json:"cas"`
	Singbox SingboxInfo `json:"singbox"`
	Admin   AdminInfo `json:"admin"`
}

type ServerInfo struct {
	Listen string `json:"listen"`
}

type CASInfo struct {
	ServerUrl  string `json:"serverUrl"`
	ServiceUrl string `json:"serviceUrl"`
}

type SingboxInfo struct {
	Version       string `json:"version"`
	DownloadBase  string `json:"downloadBase"`
	LocalPath    string `json:"localPath"`
}

type AdminInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var serverConfig *ServerConfig

// LoadConfig 加载外部配置文件
func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Warning: failed to load config file %s: %v, using defaults", path, err)
		return err
	}
	
	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Warning: failed to parse config file: %v, using defaults", err)
		return err
	}
	
	serverConfig = &cfg
	log.Printf("Config loaded: singbox version=%s, cas=%s", cfg.Singbox.Version, cfg.CAS.ServerUrl)
	return nil
}

// GetSingboxVersion 获取 sing-box 版本
func GetSingboxVersion() string {
	if serverConfig != nil {
		return serverConfig.Singbox.Version
	}
	return "1.9.4"
}

// GetSingboxDownloadURL 获取 sing-box 下载地址（指向服务端自身）
func GetSingboxDownloadURL(listenAddr string) string {
	if serverConfig != nil {
		return "http://" + listenAddr + serverConfig.Singbox.DownloadBase + "/" + serverConfig.Singbox.Version
	}
	return "http://localhost:8080/api/v1/singbox/1.9.4"
}

// GetCasServerURL 获取 CAS 服务器地址
func GetCasServerURL() string {
	if serverConfig != nil {
		return serverConfig.CAS.ServerUrl
	}
	return "https://hubportaltest.hqts.cn"
}

// GetCasServiceURL 获取 CAS Service URL
func GetCasServiceURL() string {
	if serverConfig != nil {
		return serverConfig.CAS.ServiceUrl
	}
	return "hqts://auth/callback"
}

// GetSingboxLocalPath 获取 sing-box 本地存储路径
func GetSingboxLocalPath() string {
	if serverConfig != nil {
		return serverConfig.Singbox.LocalPath
	}
	return "./singbox-bin"
}

/**
 * HandleDownloadSingbox 下载 sing-box 二进制
 * GET /api/v1/singbox/:version
 */
func HandleDownloadSingbox(c *gin.Context) {
	version := c.Param("version")
	
	// 确保目录存在
	localPath := GetSingboxLocalPath()
	
	// 尝试多个可能的文件名
	possibleNames := []string{
		"sing-box-" + version + "-windows-amd64.zip",
		"sing-box-" + version + "-windows-amd64-signed.zip",
		"sing-box.exe",
		"sing-box-" + version + ".zip",
	}
	
	var filePath string
	for _, name := range possibleNames {
		testPath := filepath.Join(localPath, name)
		if _, err := os.Stat(testPath); err == nil {
			filePath = testPath
			break
		}
	}
	
	// 如果本地文件不存在，尝试下载
	if filePath == "" {
		c.JSON(404, gin.H{
			"error":   "sing-box binary not found",
			"version": version,
			"hint":    "请先通过管理后台上传 sing-box 二进制文件到 " + localPath + " 目录",
		})
		return
	}
	
	c.File(filePath)
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
			Version:     GetSingboxVersion(),
			DownloadURL: GetSingboxDownloadURL(GetListenAddr()),
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

// ClientLogEntry 客户端日志条目
type ClientLogEntry struct {
	Level     string `json:"level"`      // error, warn, info, debug
	Message   string `json:"message"`
	Stack     string `json:"stack,omitempty"`
	Source    string `json:"source"`     // authManager, configManager, singboxAdapter, etc.
	Timestamp string `json:"timestamp"`
	UserID    string `json:"userId,omitempty"`
	ClientVersion string `json:"clientVersion"`
}

// ClientLogs 客户端日志存储（内存中，生产环境建议用数据库
var clientLogs []ClientLogEntry
var clientLogsMu sync.Mutex

/**
 * HandleClientLog 接收客户端日志
 * POST /api/v1/client-logs
 */
func HandleClientLog(c *gin.Context) {
	var entry ClientLogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 存储日志
	clientLogsMu.Lock()
	clientLogs = append(clientLogs, entry)
	// 最多保留最近1000条
	if len(clientLogs) > 1000 {
		clientLogs = clientLogs[len(clientLogs)-1000:]
	}
	clientLogsMu.Unlock()

	log.Printf("[ClientLog] [%s] [%s] %s - %s", 
		entry.Level, entry.Source, entry.Timestamp, entry.Message)

	c.JSON(200, gin.H{"success": true, "logId": len(clientLogs)})
}

/**
 * HandleGetClientLogs 获取客户端日志（管理员）
 * GET /api/v1/client-logs
 */
func HandleGetClientLogs(c *gin.Context) {
	clientLogsMu.Lock()
	logs := make([]ClientLogEntry, len(clientLogs))
	copy(logs, clientLogs)
	clientLogsMu.Unlock()

	c.JSON(200, gin.H{
		"logs": logs,
		"total": len(logs),
	})
}
