package admin

/**
 * Admin - 管理员认证和授权
 * 
 * 支持两种管理员认证方式：
 * 1. 独立的管理员密码
 * 2. 企业CAS账号白名单
 */

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/**
 * 管理员账户配置
 */
var (
	// 独立的管理员密码（生产环境必须修改！）
	adminPassword = "hqts-admin-2024" // 建议设置强密码

	// 管理员白名单 - 企业CAS账号列表
	// 这些账号登录后自动获得管理员权限
	adminWhitelist = map[string]bool{
		"admin@hqts.cn":      true,
		"zhangsan@hqts.cn":    true,
		"tech-leader@hqts.cn": true,
	}
)
var (
	// JWT配置
	jwtSecret     = []byte("change-admin-jwt-secret-in-production")
	adminJwtExpiry = time.Hour * 8 // 8小时

	// Token存储
	tokenStore = struct {
		sync.RWMutex
		m map[string]*adminTokenInfo
	}{
		m: make(map[string]*adminTokenInfo),
	}
)

// SetupAdmin 设置管理员密码
func SetupAdmin(password string) {
	adminPassword = password
	log.Println("Admin password updated from config")
}

type AdminUser struct {
	Username string `json:"username"`
	Password string `json:"-"` // 不返回密码
	Role     string `json:"role"` // super_admin, admin
	IsCAS    bool   `json:"isCas"` // 是否是CAS账号
}

type adminTokenInfo struct {
	username  string
	isCAS     bool // 是否是CAS账号
	expiresAt time.Time
}

// GenerateRandomString 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// HandleAdminLogin 管理员登录（独立密码）
func HandleAdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证管理员密码
	if req.Password != adminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// 使用用户名作为管理员身份
	username := req.Username
	if username == "" {
		username = "admin"
	}

	// 生成JWT token
	token := generateRandomString(64)

	// 存储token
	tokenStore.Lock()
	tokenStore.m[token] = &adminTokenInfo{
		username:  username,
		isCAS:    false,
		expiresAt: time.Now().Add(adminJwtExpiry),
	}
	tokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"username": username,
			"role":     "super_admin",
			"isCas":    false,
		},
		"expiresIn": int(adminJwtExpiry.Seconds()),
	})
}

// HandleCASAdminLogin CAS管理员登录
// 当企业用户通过CAS登录后，如果该用户在白名单中，可以获取管理员token
func HandleCASAdminLogin(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否在白名单中
	if !adminWhitelist[strings.ToLower(req.Email)] {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "此账号没有管理员权限",
		})
		return
	}

	// 生成JWT token
	token := generateRandomString(64)

	// 存储token
	tokenStore.Lock()
	tokenStore.m[token] = &adminTokenInfo{
		username:  req.Email,
		isCAS:    true,
		expiresAt: time.Now().Add(adminJwtExpiry),
	}
	tokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"username": req.Email,
			"name":     req.Name,
			"role":     "admin",
			"isCas":    true,
		},
		"expiresIn": int(adminJwtExpiry.Seconds()),
	})
}

// HandleAdminLogout 管理员登出
func HandleAdminLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	tokenStore.Lock()
	delete(tokenStore.m, token)
	tokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleGetAdminWhitelist 获取管理员白名单
func HandleGetAdminWhitelist(c *gin.Context) {
	whitelist := make([]string, 0, len(adminWhitelist))
	for email := range adminWhitelist {
		whitelist = append(whitelist, email)
	}

	c.JSON(http.StatusOK, gin.H{
		"whitelist": whitelist,
		"total":     len(whitelist),
	})
}

// HandleAddAdminWhitelist 添加管理员白名单
func HandleAddAdminWhitelist(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.ToLower(req.Email)
	adminWhitelist[email] = true

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已添加管理员: " + email,
	})
}

// HandleRemoveAdminWhitelist 移除管理员白名单
func HandleRemoveAdminWhitelist(c *gin.Context) {
	email := c.Param("email")

	if _, exists := adminWhitelist[email]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found in whitelist"})
		return
	}

	delete(adminWhitelist, email)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已移除管理员: " + email,
	})
}

// AdminAuthMiddleware 管理员认证中间件
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		tokenStore.RLock()
		info, exists := tokenStore.m[token]
		tokenStore.RUnlock()

		if !exists || time.Now().After(info.expiresAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("adminUsername", info.username)
		c.Set("adminIsCAS", info.isCAS)
		c.Next()
	}
}

// GetAdminUsername 获取当前管理员用户名
func GetAdminUsername(c *gin.Context) string {
	username, _ := c.Get("adminUsername")
	return username.(string)
}

// IsCASAdmin 是否是CAS账号管理员
func IsCASAdmin(c *gin.Context) bool {
	isCAS, _ := c.Get("adminIsCAS")
	return isCAS.(bool)
}
