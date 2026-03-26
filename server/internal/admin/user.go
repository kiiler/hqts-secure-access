package admin

/**
 * User - 用户管理
 */

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/**
 * 用户状态
 */
const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

/**
 * 用户数据结构
 */
type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Group       string   `json:"group"`
	Status      string   `json:"status"`
	CreatedAt   int64    `json:"created_at"`
	LastLoginAt int64    `json:"last_login_at,omitempty"`
	LoginCount  int      `json:"login_count"`
}

// 用户存储（实际应该用数据库）
var (
	userStore = struct {
		sync.RWMutex
		m map[string]*User
	}{
		m: make(map[string]*User),
	}
)

// CreateOrGetUser 创建或获取用户（JIT模式）
// 用户首次登录时自动创建
func CreateOrGetUser(username, name, email, department string) *User {
	userStore.Lock()
	defer userStore.Unlock()

	// 查找是否已存在
	for _, u := range userStore.m {
		if u.Username == username || u.Email == email {
			// 更新登录信息
			u.LoginCount++
			u.LastLoginAt = time.Now().Unix()
			return u
		}
	}

	// 创建新用户
	id := fmt.Sprintf("u%03d", len(userStore.m)+1)
	newUser := &User{
		ID:         id,
		Username:   username,
		Name:       name,
		Email:      email,
		Group:      department,
		Status:     UserStatusActive, // 新用户默认可用
		CreatedAt:  time.Now().Unix(),
		LastLoginAt: time.Now().Unix(),
		LoginCount: 1,
	}

	userStore.m[newUser.ID] = newUser
	log.Printf("JIT: Created new user %s (%s)", username, email)
	return newUser
}

// HandleListUsers 获取用户列表
func HandleListUsers(c *gin.Context) {
	username := c.Query("username")
	group := c.Query("group")
	status := c.Query("status")

	userStore.RLock()
	defer userStore.RUnlock()

	users := make([]*User, 0)
	for _, u := range userStore.m {
		// 过滤条件
		if username != "" && u.Username != username {
			continue
		}
		if group != "" && u.Group != group {
			continue
		}
		if status != "" && u.Status != status {
			continue
		}
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// HandleGetUser 获取单个用户
func HandleGetUser(c *gin.Context) {
	userID := c.Param("id")

	userStore.RLock()
	user, exists := userStore.m[userID]
	userStore.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// HandleUpdateUser 更新用户（启用/禁用）
func HandleUpdateUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Status string `json:"status"` // active, disabled
		Name   string `json:"name"`
		Group  string `json:"group"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userStore.Lock()
	defer userStore.Unlock()

	user, exists := userStore.m[userID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 更新字段
	if req.Status != "" {
		if req.Status != UserStatusActive && req.Status != UserStatusDisabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		user.Status = req.Status
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Group != "" {
		user.Group = req.Group
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

// HandleDeleteUser 删除用户
func HandleDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	userStore.Lock()
	defer userStore.Unlock()

	if _, exists := userStore.m[userID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	delete(userStore.m, userID)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleGetStats 获取统计数据
func HandleGetStats(c *gin.Context) {
	userStore.RLock()
	nodeStore.RLock()
	defer userStore.RUnlock()
	defer nodeStore.RUnlock()

	totalUsers := len(userStore.m)
	activeUsers := 0
	for _, u := range userStore.m {
		if u.Status == UserStatusActive {
			activeUsers++
		}
	}

	totalNodes := len(nodeStore.m)
	onlineNodes := 0
	for _, n := range nodeStore.m {
		if n.Status == NodeStatusOnline {
			onlineNodes++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"totalUsers":   totalUsers,
		"activeUsers":  activeUsers,
		"totalNodes":   totalNodes,
		"onlineNodes":  onlineNodes,
	})
}

// HandleAdminAuditList 获取审计日志
func HandleAdminAuditList(c *gin.Context) {
	// TODO: 从数据库读取审计日志
	// 目前返回空列表
	c.JSON(http.StatusOK, gin.H{
		"logs": []interface{}{},
		"total": 0,
	})
}
