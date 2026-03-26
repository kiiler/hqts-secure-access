package config

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

/**
 * 版本信息
 */
var (
	// 客户端最新版本
	clientVersion = "1.0.0"

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
		"version":       clientVersion,
		"downloadUrl":  clientDownloadUrl,
		"releaseNotes": clientReleaseNotes[clientVersion],
	})
}

/**
 * HandleSetVersion 设置最新版本（管理员调用）
 * POST /api/v1/version
 */
func HandleSetVersion(c *gin.Context) {
	var req struct {
		Version      string `json:"version"`
		DownloadUrl  string `json:"downloadUrl"`
		ReleaseNotes string `json:"releaseNotes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientVersion = req.Version
	if req.DownloadUrl != "" {
		clientDownloadUrl = req.DownloadUrl
	}
	if req.ReleaseNotes != "" {
		clientReleaseNotes[req.Version] = req.ReleaseNotes
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"version": clientVersion,
	})
}
