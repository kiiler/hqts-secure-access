package config

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

/**
 * 版本信息
 */
var (
	// 客户端最新版本
	clientVersion = "1.0.0"

	// 最低支持版本（低于此版本强制更新）
	minimumVersion = "1.0.0"

	// 客户端下载URL
	clientDownloadUrl = "https://github.com/kiiler/hqts-secure-access/releases"

	// 客户端 release notes
	clientReleaseNotes = map[string]string{
		"1.0.0": "初始版本",
	}
)

/**
 * HandleGetVersion 获取最新版本信息
 * GET /api/v1/version
 */
func HandleGetVersion(c *gin.Context) {
	log.Printf("Version check requested")

	c.JSON(http.StatusOK, gin.H{
		"version":        clientVersion,
		"minimumVersion": minimumVersion,
		"downloadUrl":   clientDownloadUrl,
		"releaseNotes":   clientReleaseNotes[clientVersion],
	})
}

/**
 * HandleSetVersion 设置最新版本（管理员调用）
 * POST /api/v1/version
 */
func HandleSetVersion(c *gin.Context) {
	var req struct {
		Version       string `json:"version"`
		MinimumVersion string `json:"minimumVersion"`
		DownloadUrl   string `json:"downloadUrl"`
		ReleaseNotes  string `json:"releaseNotes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Version != "" {
		clientVersion = req.Version
	}
	if req.MinimumVersion != "" {
		minimumVersion = req.MinimumVersion
	}
	if req.DownloadUrl != "" {
		clientDownloadUrl = req.DownloadUrl
	}
	if req.ReleaseNotes != "" {
		clientReleaseNotes[req.Version] = req.ReleaseNotes
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"version":       clientVersion,
		"minimumVersion": minimumVersion,
	})
}

/**
 * CheckVersion 检查版本是否满足要求
 * 返回 (满足要求, 错误信息)
 */
func CheckVersion(clientVersionStr string) (bool, string) {
	if clientVersionStr == "" {
		return false, "缺少版本号"
	}

	// 解析版本号
	clientParts := parseVersion(clientVersionStr)
	minParts := parseVersion(minimumVersion)

	// 比较版本
	for i := 0; i < 3; i++ {
		c := getOrZero(clientParts, i)
		m := getOrZero(minParts, i)
		if c > m {
			return true, ""
		}
		if c < m {
			return false, "版本过低，需要更新"
		}
	}

	return true, ""
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		if num, err := strconv.Atoi(p); err == nil {
			result = append(result, num)
		}
	}
	return result
}

func getOrZero(parts []int, i int) int {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}
