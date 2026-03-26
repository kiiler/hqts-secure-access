# HQTS Secure Access Client

基于 sing-box 内核的企业级 VPN 客户端 + 控制平台

## 项目概述

- **客户端**：Windows 桌面应用（Electron）
- **服务端**：Linux 服务器（Go）
- **核心引擎**：sing-box
- **认证**：泛微 OA CAS（Central Authentication Service）

## 功能特性

- ✅ CAS 单点登录认证
- ✅ 分流模式（全局 / 绕过大陆）
- ✅ DNS 防泄露（TUN 模式）
- ✅ 配置中心下发（节点 + 分流）
- ✅ 节点健康感知 + 自动故障转移
- ✅ 定时拉取最新配置
- ✅ 用户无节点选择权限（企业可控）
- ✅ 审计日志

## 项目结构

```
hqts-secure-access/
├── client/                    # Electron 客户端
│   ├── src/
│   │   ├── main/              # 主进程
│   │   ├── preload/           # 预加载脚本
│   │   ├── core/              # 本地控制层
│   │   │   ├── authManager.js    # CAS 认证管理
│   │   │   ├── configManager.js  # 配置管理
│   │   │   └── policyEngine.js   # 策略引擎（核心）
│   │   ├── singbox-adapter/      # sing-box 适配器
│   │   └── ui/                # UI 层（极简）
│   ├── package.json
│   └── electron.vite.config.mjs
│
├── server/                    # Go 服务端
│   ├── cmd/server/            # 主入口
│   ├── internal/
│   │   ├── auth/              # 认证服务（CAS + JWT）
│   │   ├── config/            # 配置中心
│   │   ├── node/              # 节点目录
│   │   ├── policy/            # 策略中心
│   │   └── audit/             # 审计日志
│   └── pkg/models/            # 数据模型
│
└── docs/                      # 项目文档
```

## 认证流程

```
┌─────────────────────────────────────────────────────────────┐
│ 1. 用户点击「使用企业账号登录」                               │
│           ↓                                                  │
│ 2. 弹出 CAS 登录窗口，加载泛微 OA 登录页                    │
│           ↓                                                  │
│ 3. 用户输入泛微 OA 账号密码                                  │
│           ↓                                                  │
│ 4. CAS 验证成功，重定向到 hqts://auth/callback?ticket=xxx    │
│           ↓                                                  │
│ 5. 客户端提取 ticket，调用 /api/v1/auth/cas/exchange         │
│           ↓                                                  │
│ 6. 服务端返回 JWT token，登录完成                            │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 前置依赖

- Node.js >= 18
- Go >= 1.21
- Windows 10/11

### 配置

**服务端** `server/internal/auth/auth.go`：
```go
casConfig = struct {
    CasServerURL: "https://oa.hqts.cn/cas",  // 替换为实际泛微OA地址
    ServiceURL:   "hqts://auth/callback",
}
```

**客户端** `client/src/core/authManager.js`：
```javascript
const CAS_CONFIG = {
  casServerUrl: 'https://oa.hqts.cn/cas',  // 替换为实际泛微OA地址
  serviceUrl: 'hqts://auth/callback',
}
```

### 服务端启动

```bash
cd server
go mod tidy
go run cmd/server/main.go
```

服务将运行在 `http://localhost:8080`

### 客户端开发

```bash
cd client
npm install
npm run dev
```

### 客户端打包

```bash
cd client
npm run package
```

打包后的安装程序位于 `client/release/` 目录

## 技术架构

### 客户端架构

```
UI 层（极简）
    ↓
本地控制层（核心）
    ├── AuthManager      # CAS 认证 + JWT 管理
    ├── ConfigManager    # 配置拉取/缓存/定时更新
    └── PolicyEngine     # 策略→sing-box配置编译
    ↓
sing-box 运行层（TUN 模式）
```

### 服务端 API

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/auth/login` | GET | CAS 登录入口 |
| `/api/v1/auth/validate` | GET | CAS Ticket 验证 |
| `/api/v1/auth/cas/exchange` | POST | CAS Ticket 换取内部 Token |
| `/api/v1/auth/token` | POST | 获取/刷新 Token |
| `/api/v1/auth/logout` | POST | 登出 |
| `/api/v1/config` | GET | 获取用户配置 |
| `/api/v1/nodes` | GET | 节点列表 |
| `/api/v1/nodes/health` | GET | 节点健康状态 |
| `/api/v1/policy/user` | GET | 用户策略 |
| `/api/v1/audit/log` | POST | 记录审计日志 |

## 配置模型

### 产品级配置（服务端下发）

```json
{
  "version": "v1",
  "user": {
    "id": "u001",
    "group": "CN_EMPLOYEE"
  },
  "mode": "BYPASS_CN",
  "dns_policy": "STANDARD",
  "nodes": [
    {
      "id": "hk-01",
      "region": "HK",
      "priority": 1,
      "host": "hk-01.hqts.cn",
      "port": 443,
      "protocol": "vmess"
    }
  ],
  "features": {
    "allow_mode_switch": true
  }
}
```

### 分流模式

| 模式 | 描述 |
|------|------|
| `GLOBAL` | 所有流量走代理，仅本地地址直连 |
| `BYPASS_CN` | 中国 IP 直连，海外流量走代理 |

### 节点健康状态

```json
{
  "node_id": "hk-01",
  "online": true,
  "latency": 15,
  "load": 35.5,
  "bandwidth": "100GB"
}
```

## 故障处理

- **定时拉取**：每 5 分钟自动拉取最新配置和节点健康状态
- **故障转移**：当前节点连接失败时，自动切换到下一个健康节点
- **离线缓存**：配置本地缓存，断网时仍可使用

## 许可证

内部使用
