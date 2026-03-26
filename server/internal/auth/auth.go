package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"hqts-secure-access-server/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/**
 * Auth Service - 认证服务
 * 
 * 职责：
 * - 对接泛微OA OAuth2
 * - JWT Token签发和验证
 * - Token存储和刷新
 */

var (
	// 模拟OAuth2配置 - 实际应从环境变量读取
	oauth2Config = struct {
		ClientID     string
		ClientSecret string
		AuthURL      string
		TokenURL     string
		RedirectURI  string
	}{
		ClientID:     "hqts-secure-access-client",
		ClientSecret: "change-me-in-production",
		AuthURL:      "https://oauth2.hqts.cn/authorize",
		TokenURL:     "https://oauth2.hqts.cn/token",
		RedirectURI:  "hqts://oauth/callback",
	}

	// JWT配置
	jwtSecret = []byte("change-jwt-secret-in-production")
	jwtExpiry = time.Hour * 1 // 1小时

	// Token存储 (实际应使用Redis)
	tokenStore = struct {
		sync.RWMutex
		m map[string]*tokenInfo
	}{
		m: make(map[string]*tokenInfo),
	}
)

type tokenInfo struct {
	userID       string
	refreshToken string
	expiresAt    time.Time
}

// 初始化 - 注册模拟用户
func init() {
	// 添加一些测试用户
	storeUser("u001", "zhangsan", "zhangsan@hqts.cn", "CN_EMPLOYEE")
	storeUser("u002", "lisi", "lisi@hqts.cn", "HK_EMPLOYEE")
}

// 存储用户信息
var mockUsers = map[string]*models.User{
	"u001": {ID: "u001", Name: "张三", Email: "zhangsan@hqts.cn", Group: "CN_EMPLOYEE"},
	"u002": {ID: "u002", Name: "李四", Email: "lisi@hqts.cn", Group: "HK_EMPLOYEE"},
}

func storeUser(id, name, email, group string) {
	mockUsers[id] = &models.User{ID: id, Name: name, Email: email, Group: group}
}

// 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// HandleLogin 处理 OAuth2 登录入口
func HandleLogin(c *gin.Context) {
	// 生成state防止CSRF
	state := generateRandomString(32)

	// 构建授权URL
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		oauth2Config.AuthURL, oauth2Config.ClientID, oauth2Config.RedirectURI, state)

	// 重定向到泛微OA
	c.Redirect(http.StatusFound, authURL)
}

// HandleCallback 处理 OAuth2 回调
// 注意：实际情况下，泛微OA会回调到这里，我们需要用code换token
func HandleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	// 实际场景：在这里用code调用泛微OA获取token
	// 简化处理：模拟登录成功
	user := mockUsers["u001"] // 默认第一个用户

	tokenResp := generateTokenResponse(user)

	c.JSON(http.StatusOK, tokenResp)
}

// HandleToken 处理 Token 请求（获取/刷新）
func HandleToken(c *gin.Context) {
	var req struct {
		GrantType    string `json:"grant_type"`    // authorization_code, refresh_token
		Code         string `json:"code"`          // authorization_code时需要
		RefreshToken string `json:"refresh_token"` // refresh_token时需要
		ClientID     string `json:"client_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.GrantType {
	case "authorization_code":
		// 用授权码换Token - 这里简化处理
		user := mockUsers["u001"] // 默认用户
		c.JSON(http.StatusOK, generateTokenResponse(user))

	case "refresh_token":
		// 刷新Token
		tokenStore.RLock()
		info, ok := tokenStore.m[req.RefreshToken]
		tokenStore.RUnlock()

		if !ok || time.Now().After(info.expiresAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}

		user := mockUsers[info.userID]
		c.JSON(http.StatusOK, generateTokenResponse(user))

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported grant_type"})
	}
}

// HandleLogout 处理登出
func HandleLogout(c *gin.Context) {
	// 从Header获取Token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// 解析Token获取用户ID
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	claims := token.Claims.(jwt.MapClaims)
	userID := claims["sub"].(string)

	// 从存储中移除
	tokenStore.Lock()
	delete(tokenStore.m, userID)
	tokenStore.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HandleRevoke 处理Token撤销
func HandleRevoke(c *gin.Context) {
	HandleLogout(c)
}

// 生成Token响应
func generateTokenResponse(user *models.User) models.TokenResponse {
	accessToken := generateRandomString(64)
	refreshToken := generateRandomString(64)

	// 存储Token
	tokenStore.Lock()
	tokenStore.m[accessToken] = &tokenInfo{
		userID:       user.ID,
		refreshToken: refreshToken,
		expiresAt:    time.Now().Add(jwtExpiry),
	}
	tokenStore.Unlock()

	return models.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(jwtExpiry.Seconds()),
		UserInfo:     user,
	}
}

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 解析和验证Token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil {
			log.Printf("Token parse error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("userID", claims["sub"])
			c.Set("user", claims["user"])
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}
	}
}

// 获取当前用户ID
func GetCurrentUserID(c *gin.Context) string {
	userID, _ := c.Get("userID")
	return userID.(string)
}

// 获取当前用户信息
func GetCurrentUser(c *gin.Context) *models.User {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}
	userBytes, _ := json.Marshal(user)
	var userInfo models.User
	json.Unmarshal(userBytes, &userInfo)
	return &userInfo
}
