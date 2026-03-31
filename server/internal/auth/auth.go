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
 * Auth Service - 认证服务（CAS + JWT）
 * 
 * 职责：
 * - CAS Ticket 验证
 * - JWT Token 签发和验证
 * - Token存储和刷新
 */

var (
	// CAS 配置 - 需要替换为实际的泛微OA地址
	casConfig = struct {
		CasServerURL string // https://hubportaltest.hqts.cn
		ServiceURL   string // hqts://auth/callback
	}{
		CasServerURL: "https://hubportaltest.hqts.cn",
		ServiceURL:   "hqts://auth/callback",
	}

	// JWT配置
	jwtSecret = []byte("change-jwt-secret-in-production")
	jwtExpiry = time.Hour * 1 // 1小时
	refreshExpiry = time.Hour * 24 * 7 // 7天

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

// 用户存储（JIT模式 - 用户首次登录时自动创建）
var mockUsers = map[string]*models.User{}

func storeUser(id, nameCN, name, email, group string) {
	mockUsers[name] = &models.User{ID: id, Name: nameCN, Email: email, Group: group}
}

// 生成随机字符串
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// CAS登录入口 - 重定向到CAS登录页
func HandleLogin(c *gin.Context) {
	// 构建CAS登录URL
	casLoginURL := fmt.Sprintf("%s/login?service=%s",
		casConfig.CasServerURL,
		casConfig.ServiceURL)

	log.Printf("Redirecting to CAS login: %s", casLoginURL)
	c.Redirect(http.StatusFound, casLoginURL)
}

// CAS Service Validate - 验证ticket并获取用户信息
// 这个接口供客户端直接调用（票根验证模式）
func HandleServiceValidate(c *gin.Context) {
	ticket := c.Query("ticket")
	service := c.Query("service")

	if ticket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing ticket"})
		return
	}

	log.Printf("Validating CAS ticket: %s for service: %s", ticket, service)

	// 实际环境中，需要调用CAS服务器的/serviceValidate接口验证ticket
	// 这里简化处理：模拟验证成功
	//
	// 标准CAS /serviceValidate响应格式：
	// <cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
	//   <cas:authenticationSuccess>
	//     <cas:user>username</cas:user>
	//     <cas:attributes>
	//       <cas:name>Display Name</cas:name>
	//       <cas:email>user@example.com</cas:email>
	//     </cas:attributes>
	//   </cas:authenticationSuccess>
	// </cas:serviceResponse>

	// 模拟：使用ticket作为用户名（实际应该调用CAS服务器验证）
	username := "u001" // 模拟用户，实际应该解析CAS响应
	user := mockUsers[username]

	if user == nil {
		// 返回CAS失败响应
		c.JSON(http.StatusOK, gin.H{
			"serviceResponse": gin.H{
				"authenticationFailure": gin.H{
					"code":        "INVALID_TICKET",
					"description": "Ticket validation failed",
				},
			},
		})
		return
	}

	// 返回CAS成功响应
	c.JSON(http.StatusOK, gin.H{
		"serviceResponse": gin.H{
			"authenticationSuccess": gin.H{
				"user": username,
				"attributes": gin.H{
					"name":       user.Name,
					"email":      user.Email,
					"department": user.Group,
				},
			},
		},
	})
}

// CAS 票根换取内部 Token
// 客户端调用此接口，用 CAS ticket 换取内部 JWT token
func HandleCasExchange(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Department string `json:"department"`
		CasTicket string `json:"casTicket"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("CAS exchange for user: %s", req.Username)

	// 实际环境中，应该调用 CAS 服务器验证 ticket
	// 这里简化处理，假设 ticket 有效
	
	// JIT模式：自动创建或获取用户
	// 用户首次登录时自动添加到用户列表
	user, exists := mockUsers[req.Username]
	if !exists {
		// 为新用户创建记录
		newID := fmt.Sprintf("u%03d", len(mockUsers)+1)
		user = &models.User{
			ID:       newID,
			Name:     req.Name,
			Email:    req.Email,
			Group:    req.Department,
		}
		mockUsers[req.Username] = user
		log.Printf("JIT: Created new user %s (%s)", req.Username, req.Email)
	} else {
		log.Printf("JIT: Existing user %s logged in", req.Username)
	}

	// 生成内部 token
	tokenResp := generateTokenResponse(user)

	log.Printf("Token exchanged successfully for user: %s", user.Name)
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
	// 生成 JWT token
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": user.ID,
		"user": user,
		"exp": now.Add(jwtExpiry).Unix(),
		"iat": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Failed to sign JWT: %v", err)
		accessToken = generateRandomString(64)
	}

	refreshToken := generateRandomString(64)

	// 存储Token
	tokenStore.Lock()
	tokenStore.m[accessToken] = &tokenInfo{
		userID:       user.ID,
		refreshToken: refreshToken,
		expiresAt:    time.Now().Add(jwtExpiry),
	}
	// 同时存储refresh token
	tokenStore.m[refreshToken] = &tokenInfo{
		userID:    user.ID,
		expiresAt: time.Now().Add(refreshExpiry),
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
			userJSON, _ := json.Marshal(claims["user"])
			var userInfo models.User
			json.Unmarshal(userJSON, &userInfo)
			c.Set("user", &userInfo)
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
	userID, exists := c.Get("userID")
	if !exists {
		log.Printf("GetCurrentUserID: userID not found in context")
		return ""
	}
	switch v := userID.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		log.Printf("GetCurrentUserID: userID type=%T value=%v", userID, userID)
		return fmt.Sprintf("%v", userID)
	}
}

// 获取当前用户信息
func GetCurrentUser(c *gin.Context) *models.User {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}
	return user.(*models.User)
}
