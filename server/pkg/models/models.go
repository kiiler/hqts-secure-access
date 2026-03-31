package models

import "time"

/**
 * 产品级配置模型
 */

// 用户信息
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Group string `json:"group"` // CN_EMPLOYEE, HK_EMPLOYEE, etc.
}

// 节点信息
type Node struct {
	ID       string `json:"id"`       // hk-01, sg-01, etc.
	Region   string `json:"region"`   // HK, SG, US, etc.
	Priority int    `json:"priority"` // 1 = highest
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // vmess, vless, trojan, shadowsocks
	Password string `json:"password,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	AlterId  int    `json:"alterId,omitempty"`
	Method   string `json:"method,omitempty"`
	Flow     string `json:"flow,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
}

// 功能开关
type Features struct {
	AllowModeSwitch bool    `json:"allow_mode_switch"` // 是否允许用户切换模式
	ForceMode       *string `json:"force_mode"`        // 强制模式，nil表示不强制
}

// DNS策略
type DNSPolicy struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // doh, dot, udp
}

// 产品级配置
type ProductConfig struct {
	Version      string      `json:"version"`       // v1, v2, etc.
	User         User        `json:"user"`
	Mode         string      `json:"mode"`          // GLOBAL, BYPASS_CN
	DNSPolicy    string      `json:"dns_policy"`    // STANDARD, CUSTOM
	RoutePolicy  string      `json:"route_policy"`  // BYPASS_CN, GLOBAL
	Nodes        []Node      `json:"nodes"`
	Features     Features    `json:"features"`
	DNSServers   []DNSPolicy `json:"dns_servers,omitempty"`
	Singbox      SingboxConfig `json:"singbox"`     // sing-box 下载配置
}

// sing-box 下载配置
type SingboxConfig struct {
	Version     string `json:"version"`      // 版本号，如 "1.9.4"
	DownloadURL string `json:"download_url"` // 下载地址，如 "https://cdn.example.com/sing-box-1.9.4-windows-amd64.zip"
}

// 用户策略
type UserPolicy struct {
	UserID       string   `json:"user_id"`
	DefaultMode  string   `json:"default_mode"`
	AllowedModes []string `json:"allowed_modes"` // 用户可用的模式列表
	Group        string   `json:"group"`
	ExpireAt     *int64   `json:"expire_at,omitempty"`
}

// Token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // 秒
	UserInfo     *User  `json:"user_info,omitempty"`
}

// 审计日志
type AuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`    // LOGIN, LOGOUT, CONNECT, DISCONNECT, MODE_SWITCH
	Details   string    `json:"details"`   // JSON string
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

// 节点健康状态
type NodeHealth struct {
	NodeID   string  `json:"node_id"`
	Online   bool    `json:"online"`
	Latency  int     `json:"latency"`  // ms
	Load     float64 `json:"load"`      // 0-100%
	Bandwidth string `json:"bandwidth"` // GB
}
