package admin

/**
 * Admin - 管理员认证和授权
 */

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/**
 * 管理员账户配置
 * 生产环境应该从配置文件或数据库读取
 */
var (
	// 默认管理员账号（生产环境应更换）
	adminUsers = map[string]*AdminUser{
		"admin": {
			Username: "admin",
			Password: "admin123", // 生产环境必须修改！
			Role:     "super_admin",
		},
	}

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

type AdminUser struct {
	Username string `json:"username"`
	Password string `json:"-"` // 不返回密码
	Role     string `json:"role"`
}

type adminTokenInfo struct {
	username  string
	expiresAt time.Time
}

// GenerateRandomString 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// HandleAdminLogin 管理员登录
func HandleAdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证账号密码
	user, exists := adminUsers[req.Username]
	if !exists || user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	// 生成JWT token
	token := generateRandomString(64)
	
	// 存储token
	tokenStore.Lock()
	tokenStore.m[token] = &adminTokenInfo{
		username:  user.Username,
		expiresAt: time.Now().Add(adminJwtExpiry),
	}
	tokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"username": user.Username,
			"role":     user.Role,
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
		c.Next()
	}
}

// GetAdminUsername 获取当前管理员用户名
func GetAdminUsername(c *gin.Context) string {
	username, _ := c.Get("adminUsername")
	return username.(string)
}
