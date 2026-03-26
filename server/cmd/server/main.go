package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"hqts-secure-access-server/internal/auth"
	"hqts-secure-access-server/internal/config"
	"hqts-secure-access-server/internal/node"
	"hqts-secure-access-server/internal/policy"
	"hqts-secure-access-server/internal/audit"

	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("HQTS Secure Access Server starting...")

	// 初始化数据库
	if err := audit.InitDB(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
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

	// API 路由组
	api := r.Group("/api/v1")
	{
		// 认证
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/login", auth.HandleLogin)              // CAS 登录入口
			authGroup.GET("/validate", auth.HandleServiceValidate) // CAS Ticket 验证
			authGroup.POST("/cas/exchange", auth.HandleCasExchange) // CAS Ticket 换取内部 Token
			authGroup.POST("/token", auth.HandleToken)             // 获取/刷新 Token
			authGroup.POST("/logout", auth.HandleLogout)            // 登出
			authGroup.POST("/revoke", auth.HandleRevoke)            // 撤销 Token
		}

		// 配置中心
		configGroup := api.Group("/config")
		configGroup.Use(auth.AuthMiddleware())
		{
			configGroup.GET("", config.HandleGetConfig)   // 获取配置
		}

		// 节点目录
		nodeGroup := api.Group("/nodes")
		nodeGroup.Use(auth.AuthMiddleware())
		{
			nodeGroup.GET("", node.HandleListNodes)       // 节点列表
			nodeGroup.GET("/health", node.HandleHealth)   // 节点健康状态
		}

		// 策略中心
		policyGroup := api.Group("/policy")
		policyGroup.Use(auth.AuthMiddleware())
		{
			policyGroup.GET("/user", policy.HandleGetUserPolicy)   // 获取用户策略
		}

		// 审计
		auditGroup := api.Group("/audit")
		auditGroup.Use(auth.AuthMiddleware())
		{
			auditGroup.POST("/log", audit.HandleLog)   // 记录审计日志
			auditGroup.GET("/logs", audit.HandleGetLogs) // 查询审计日志
		}
	}

	// 启动服务器
	go func() {
		if err := r.Run(":8080"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Println("HQTS Secure Access Server started on :8080")

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
