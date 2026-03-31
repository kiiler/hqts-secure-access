package policy

import (
	"log"
	"net/http"

	"hqts-secure-access-server/pkg/models"

	"github.com/gin-gonic/gin"
)

/**
 * Policy Center - 策略中心
 * 
 * 职责：
 * - 用户 -> 策略映射
 * - 设备 -> 权限
 * - 强制模式
 */

/**
 * 用户组策略配置
 */
var groupPolicies = map[string]models.UserPolicy{
	"CN_EMPLOYEE": {
		DefaultMode:  "BYPASS_CN",
		AllowedModes: []string{"GLOBAL", "BYPASS_CN"},
		Group:        "CN_EMPLOYEE",
	},
	"HK_EMPLOYEE": {
		DefaultMode:  "GLOBAL",
		AllowedModes: []string{"GLOBAL", "BYPASS_CN"},
		Group:        "HK_EMPLOYEE",
	},
	"GUEST": {
		DefaultMode:  "BYPASS_CN",
		AllowedModes: []string{"BYPASS_CN"}, // Guest只能使用绕过大陆
		Group:        "GUEST",
	},
}

/**
 * HandleGetUserPolicy 获取用户策略
 * GET /api/v1/policy/user
 */
func HandleGetUserPolicy(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	userPolicy := GetUserPolicy(userID, "")

	c.JSON(200, userPolicy)
}

/**
 * 根据用户ID和用户组获取策略
 */
func GetUserPolicy(userID string, group string) models.UserPolicy {
	// 实际应从数据库查询
	// 这里使用默认策略
	
	defaultPolicy := models.UserPolicy{
		UserID:       userID,
		DefaultMode:  "BYPASS_CN",
		AllowedModes: []string{"GLOBAL", "BYPASS_CN"},
		Group:        group,
	}

	if group == "" {
		return defaultPolicy
	}

	// 从用户组策略中获取
	if policy, ok := groupPolicies[group]; ok {
		policy.UserID = userID
		log.Printf("Found policy for group %s: %+v", group, policy)
		return policy
	}

	return defaultPolicy
}

/**
 * 检查用户是否允许使用指定模式
 */
func IsModeAllowed(userID string, group string, mode string) bool {
	policy := GetUserPolicy(userID, group)
	
	for _, allowed := range policy.AllowedModes {
		if allowed == mode {
			return true
		}
	}
	
	return false
}

/**
 * 获取强制模式（如果有）
 */
func GetForceMode(userID string, group string) *string {
	// 某些特殊用户可能被强制指定模式
	// 这里简化处理，返回nil表示不强制
	return nil
}
