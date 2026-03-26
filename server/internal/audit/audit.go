package audit

import (
	"log"
	"net/http"
	"time"

	"hqts-secure-access-server/internal/auth"
	"hqts-secure-access-server/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

/**
 * Audit - 审计日志
 * 
 * 职责：
 * - 记录登录/登出
 * - 记录连接/断开
 * - 记录模式切换
 * - 记录配置版本
 */

var db *gorm.DB

// 审计动作类型
const (
	ActionLogin        = "LOGIN"
	ActionLogout       = "LOGOUT"
	ActionConnect      = "CONNECT"
	ActionDisconnect   = "DISCONNECT"
	ActionModeSwitch   = "MODE_SWITCH"
	ActionConfigUpdate = "CONFIG_UPDATE"
)

/**
 * 初始化数据库
 */
func InitDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open("audit.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移
	err = db.AutoMigrate(&models.AuditLog{})
	if err != nil {
		return err
	}

	log.Println("Audit database initialized")
	return nil
}

/**
 * HandleLog 记录审计日志
 * POST /api/v1/audit/log
 */
func HandleLog(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
		Details string `json:"details"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetCurrentUserID(c)

	auditEntry := models.AuditLog{
		UserID:    userID,
		Action:    req.Action,
		Details:   req.Details,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		CreatedAt: time.Now(),
	}

	// 保存到数据库
	if err := db.Create(&auditEntry).Error; err != nil {
		log.Printf("Failed to save audit log: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save audit"})
		return
	}

	c.JSON(200, gin.H{"success": true, "id": auditEntry.ID})
}

/**
 * HandleGetLogs 查询审计日志
 * GET /api/v1/audit/logs
 */
func HandleGetLogs(c *gin.Context) {
	userID := auth.GetCurrentUserID(c)
	action := c.Query("action")
	limit := c.DefaultQuery("limit", "50")

	var logs []models.AuditLog
	query := db.Where("user_id = ?", userID)

	if action != "" {
		query = query.Where("action = ?", action)
	}

	query = query.Order("created_at DESC").Limit(50)
	if err := query.Find(&logs).Error; err != nil {
		log.Printf("Failed to query audit logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query logs"})
		return
	}

	c.JSON(200, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

/**
 * 记录登录事件
 */
func LogLogin(userID string, ip string, userAgent string) {
	db.Create(&models.AuditLog{
		UserID:    userID,
		Action:    ActionLogin,
		Details:   "{\"event\": \"user_login\"}",
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: time.Now(),
	})
}

/**
 * 记录登出事件
 */
func LogLogout(userID string, ip string, userAgent string) {
	db.Create(&models.AuditLog{
		UserID:    userID,
		Action:    ActionLogout,
		Details:   "{\"event\": \"user_logout\"}",
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: time.Now(),
	})
}

/**
 * 记录连接事件
 */
func LogConnect(userID string, nodeID string, mode string) {
	db.Create(&models.AuditLog{
		UserID:    userID,
		Action:    ActionConnect,
		Details:   "{\"node_id\": \"" + nodeID + "\", \"mode\": \"" + mode + "\"}",
		CreatedAt: time.Now(),
	})
}

/**
 * 记录断开事件
 */
func LogDisconnect(userID string, reason string) {
	db.Create(&models.AuditLog{
		UserID:    userID,
		Action:    ActionDisconnect,
		Details:   "{\"reason\": \"" + reason + "\"}",
		CreatedAt: time.Now(),
	})
}

/**
 * 记录模式切换事件
 */
func LogModeSwitch(userID string, oldMode string, newMode string) {
	db.Create(&models.AuditLog{
		UserID:    userID,
		Action:    ActionModeSwitch,
		Details:   "{\"old_mode\": \"" + oldMode + "\", \"new_mode\": \"" + newMode + "\"}",
		CreatedAt: time.Now(),
	})
}
