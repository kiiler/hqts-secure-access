package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"hqts-secure-access-server/internal/admin"
	"hqts-secure-access-server/internal/auth"
	"hqts-secure-access-server/internal/config"
	"hqts-secure-access-server/internal/node"
	"hqts-secure-access-server/internal/policy"
	"hqts-secure-access-server/internal/audit"

	"github.com/gin-gonic/gin"
)

// 获取服务端监听地址
func getListenAddr() string {
	addr := os.Getenv("HQTS_LISTEN")
	if addr != "" {
		return addr
	}
	return config.GetListenAddr()
}

// 获取服务器配置
func getServerConfig() *config.ServerConfig {
	return config.GetServerConfig()
}

func main() {
	log.Println("HQTS Secure Access Server starting...")

	// 初始化数据库
	if err := audit.InitDB(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 加载外部配置
	config.LoadConfig("server-config.json")

	// 设置服务端监听地址
	config.SetListenAddr(getListenAddr())

	// 设置 CAS 认证信息
	auth.SetupCAS(config.GetCasServerURL(), config.GetCasServiceURL())

	// 设置管理员密码
	if cfg := getServerConfig(); cfg != nil && cfg.Admin.Password != "" {
		admin.SetupAdmin(cfg.Admin.Password)
	}

	// 初始化 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ============================================
	// 客户端 API
	// ============================================
	api := r.Group("/api/v1")
	{
		// 认证
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/login", auth.HandleLogin)              // CAS 登录入口
			authGroup.GET("/validate", auth.HandleServiceValidate) // CAS Ticket 验证
			authGroup.POST("/cas/exchange", auth.HandleCasExchange)  // CAS Ticket 换取内部 Token
			authGroup.POST("/token", auth.HandleToken)              // 获取/刷新 Token
			authGroup.POST("/logout", auth.HandleLogout)            // 登出
			authGroup.POST("/revoke", auth.HandleRevoke)           // 撤销 Token
		}

		// 配置中心
		configGroup := api.Group("/config")
		configGroup.Use(auth.AuthMiddleware())
		{
			configGroup.GET("", config.HandleGetConfig) // 获取用户配置（需要认证）
		}

		// 版本信息（公开接口，客户端轮询）
		api.GET("/version", config.HandleGetVersion) // 获取最新版本（不需要认证）

		// sing-box 二进制下载（公开接口）
		api.GET("/singbox/:version", config.HandleDownloadSingbox) // 下载 sing-box 二进制

		// 节点目录
		nodeGroup := api.Group("/nodes")
		nodeGroup.Use(auth.AuthMiddleware())
		{
			nodeGroup.GET("", node.HandleListNodes) // 节点列表
		}

		// 策略中心
		policyGroup := api.Group("/policy")
		policyGroup.Use(auth.AuthMiddleware())
		{
			policyGroup.GET("/user", policy.HandleGetUserPolicy) // 获取用户策略
		}

		// 审计
		auditGroup := api.Group("/audit")
		auditGroup.Use(auth.AuthMiddleware())
		{
			auditGroup.POST("/log", audit.HandleLog)   // 记录审计日志
			auditGroup.GET("/logs", audit.HandleGetLogs) // 查询审计日志
		}
	}

	// ============================================
	// 管理员前端页面（使用 /admin-panel 避免与 /admin/api 路由冲突）
	// ============================================
	adminAbsPath, _ := filepath.Abs("../admin")
	r.Static("/admin-panel", adminAbsPath)

	// ============================================
	// 管理员 API
	// ============================================
	adminAPI := r.Group("/admin/api/v1")
	{
		// 管理员登录（不需要认证）
		adminAPI.POST("/login", admin.HandleAdminLogin)          // 独立密码登录
		adminAPI.POST("/cas-login", admin.HandleCASAdminLogin)   // CAS账号登录（白名单）
		adminAPI.POST("/logout", admin.HandleAdminLogout)

		// 需要管理员认证的接口
		adminProtected := adminAPI.Group("")
		adminProtected.Use(admin.AdminAuthMiddleware())
		{
			// 用户管理
			userGroup := adminProtected.Group("/users")
			{
				userGroup.GET("", admin.HandleListUsers)
				userGroup.GET("/:id", admin.HandleGetUser)
				userGroup.PUT("/:id", admin.HandleUpdateUser)
				userGroup.DELETE("/:id", admin.HandleDeleteUser)
			}

			// 节点管理
			nodeGroup := adminProtected.Group("/nodes")
			{
				nodeGroup.GET("", admin.HandleListNodes)
				nodeGroup.GET("/:id", admin.HandleGetNode)
				nodeGroup.POST("", admin.HandleCreateNode)
				nodeGroup.PUT("/:id", admin.HandleUpdateNode)
				nodeGroup.DELETE("/:id", admin.HandleDeleteNode)
				nodeGroup.POST("/:id/test", admin.HandleTestNode)
			}

			// 审计日志
			auditGroup := adminProtected.Group("/audit")
			{
				auditGroup.GET("", admin.HandleAdminAuditList)
			}

			// 管理员白名单管理
			whitelistGroup := adminProtected.Group("/whitelist")
			{
				whitelistGroup.GET("", admin.HandleGetAdminWhitelist)
				whitelistGroup.POST("", admin.HandleAddAdminWhitelist)
				whitelistGroup.DELETE("/:email", admin.HandleRemoveAdminWhitelist)
			}

			// 统计
			adminProtected.GET("/stats", admin.HandleGetStats)

			// 版本管理
			adminProtected.GET("/version", config.HandleGetVersion)
			adminProtected.POST("/version", config.HandleSetVersion)
		}
	}

	// 启动服务器
	go func() {
		listenAddr := config.GetListenAddr()
		if err := r.Run(listenAddr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Println("HQTS Secure Access Server started on :8080")
	log.Println("Admin panel: http://localhost:8080/admin-panel/")

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
