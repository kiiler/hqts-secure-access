import { app } from 'electron'
import { join } from 'path'
import { writeFileSync, existsSync, readFileSync, mkdirSync } from 'fs'
import log from 'electron-log'
import { getApiUrl } from '../config.js'

/**
 * ConfigManager - 配置管理
 * 
 * 职责：
 * - 从Config Center拉取配置
 * - 本地缓存（离线可用）
 * - 配置版本管理
 * - 配置校验
 */

/**
 * 产品级配置模型
 * @typedef {Object} ProductConfig
 * @property {string} version - 配置版本
 * @property {Object} user - 用户信息
 * @property {string} user.id - 用户ID
 * @property {string} user.group - 用户组
 * @property {string} mode - 分流模式 GLOBAL | BYPASS_CN
 * @property {string} dns_policy - DNS策略 STANDARD | CUSTOM
 * @property {string} route_policy - 路由策略
 * @property {Array<NodeInfo>} nodes - 节点列表
 * @property {Object} features - 功能开关
 */

/**
 * @typedef {Object} NodeInfo
 * @property {string} id - 节点ID
 * @property {string} region - 区域
 * @property {number} priority - 优先级
 * @property {string} host - 主机地址
 * @property {number} port - 端口
 * @property {string} protocol - 协议 vmess | vless | trojan | shadowsocks
 */

class ConfigManager {
  constructor() {
    this.config = null
    this.serverVersion = null
    this.lastFetchTime = null
    this.fetchInterval = 5 * 60 * 1000 // 默认 5 分钟
    this.fetchTimer = null
    
    // 存储路径
    this.storageDir = join(app.getPath('userData'), 'config')
    if (!existsSync(this.storageDir)) {
      mkdirSync(this.storageDir, { recursive: true })
    }
    this.cacheFile = join(this.storageDir, 'config.json')
    this.versionFile = join(this.storageDir, 'version.json')
    
    // 加载缓存配置
    this.loadCachedConfig()
  }

  /**
   * 启动定时拉取配置
   * @param {string} accessToken
   * @param {number} intervalMs 拉取间隔（毫秒），默认5分钟
   */
  startPeriodicFetch(accessToken, intervalMs = 5 * 60 * 1000) {
    this.fetchInterval = intervalMs
    log.info(`Starting periodic config fetch, interval: ${intervalMs / 1000}s`)

    // 立即拉取一次
    this.loadConfig(accessToken)

    // 清除已有定时器
    this.stopPeriodicFetch()

    // 设置定时器
    this.fetchTimer = setInterval(async () => {
      log.info('Periodic config fetch triggered')
      try {
        const success = await this.loadConfig(accessToken)
        if (success) {
          // 触发配置更新事件
          if (this.onConfigUpdate) {
            this.onConfigUpdate(this.config)
          }
        }
      } catch (error) {
        log.error('Periodic fetch failed:', error)
      }
    }, this.fetchInterval)
  }

  /**
   * 停止定时拉取
   */
  stopPeriodicFetch() {
    if (this.fetchTimer) {
      clearInterval(this.fetchTimer)
      this.fetchTimer = null
      log.info('Periodic config fetch stopped')
    }
  }

  /**
   * 加载缓存的配置
   */
  loadCachedConfig() {
    try {
      if (existsSync(this.cacheFile)) {
        const data = JSON.parse(readFileSync(this.cacheFile, 'utf-8'))
        this.config = data.config
        this.serverVersion = data.version
        log.info('Loaded cached config, version:', this.serverVersion)
      }
    } catch (error) {
      log.error('Failed to load cached config:', error)
    }
  }

  /**
   * 保存配置到缓存
   */
  saveCachedConfig() {
    try {
      const data = {
        config: this.config,
        version: this.serverVersion,
        cachedAt: Date.now()
      }
      writeFileSync(this.cacheFile, JSON.stringify(data, null, 2))
      log.info('Config cached successfully')
    } catch (error) {
      log.error('Failed to cache config:', error)
    }
  }

  /**
   * 从服务端拉取最新配置
   * @param {string} accessToken - 访问令牌
   */
  async loadConfig(accessToken) {
    try {
      log.info('Fetching config from server...')
      
      const response = await fetch(getApiUrl('/api/v1/config'), {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${accessToken}`,
          'Accept': 'application/json'
        }
      })

      // 如果 token 过期
      if (response.status === 401) {
        log.warn('Token expired during config fetch')
        return false
      }

      if (!response.ok && response.status !== 304) {
        throw new Error(`Config fetch failed: ${response.status}`)
      }

      // 如果配置未变更
      if (response.status === 304) {
        log.info('Config not modified, using cached version')
        return true
      }

      const newConfig = await response.json()
      const newVersion = newConfig.version

      // 版本比较
      if (this.serverVersion && this.compareVersions(newVersion, this.serverVersion) <= 0) {
        log.info('Server config version not newer, skipping update')
        return true
      }

      // 校验配置
      if (!this.validateConfig(newConfig)) {
        throw new Error('Invalid config from server')
      }

      // 更新配置
      this.config = newConfig
      this.serverVersion = newVersion
      this.lastFetchTime = Date.now()
      this.saveCachedConfig()

      log.info('Config updated successfully, version:', newVersion, ', nodes:', newConfig.nodes?.length)
      return true
    } catch (error) {
      log.error('Failed to load config:', error)
      // 加载失败，使用缓存
      if (this.config) {
        log.warn('Using cached config due to fetch failure')
        return true
      }
      return false
    }
  }

  /**
   * 获取所有节点（由 NodeHealthMonitor 负责过滤健康节点）
   */
  getAllNodes() {
    return this.config?.nodes || []
  }

  /**
   * 校验配置格式
   * @param {ProductConfig} config - 待校验的配置
   */
  validateConfig(config) {
    if (!config) return false
    
    // 必需字段检查
    const requiredFields = ['version', 'user', 'mode', 'nodes']
    for (const field of requiredFields) {
      if (!config[field]) {
        log.error(`Missing required field: ${field}`)
        return false
      }
    }

    // 检查节点列表
    if (!Array.isArray(config.nodes) || config.nodes.length === 0) {
      log.error('Nodes must be a non-empty array')
      return false
    }

    // 检查每个节点的必需字段
    for (const node of config.nodes) {
      const nodeRequired = ['id', 'host', 'port', 'protocol']
      for (const field of nodeRequired) {
        if (!node[field]) {
          log.error(`Node missing required field: ${field}`)
          return false
        }
      }
    }

    return true
  }

  /**
   * 比较版本号
   * @param {string} v1 - 版本1
   * @param {string} v2 - 版本2
   * @returns {number} -1: v1<v2, 0: v1=v2, 1: v1>v2
   */
  compareVersions(v1, v2) {
    const parts1 = v1.replace(/^v/, '').split('.').map(Number)
    const parts2 = v2.replace(/^v/, '').split('.').map(Number)
    
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
      const p1 = parts1[i] || 0
      const p2 = parts2[i] || 0
      if (p1 > p2) return 1
      if (p1 < p2) return -1
    }
    return 0
  }

  /**
   * 获取当前配置
   */
  getConfig() {
    return this.config
  }

  /**
   * 获取配置版本
   */
  getVersion() {
    return this.serverVersion
  }

  /**
   * 检查配置是否过期（超过24小时）
   */
  isConfigStale() {
    if (!this.config) return true
    
    try {
      if (existsSync(this.cacheFile)) {
        const data = JSON.parse(readFileSync(this.cacheFile, 'utf-8'))
        const age = Date.now() - data.cachedAt
        return age > 24 * 60 * 60 * 1000 // 24小时
      }
    } catch {
      return true
    }
    return true
  }

  /**
   * 清除缓存的配置
   */
  clearCache() {
    try {
      if (existsSync(this.cacheFile)) {
        const { unlinkSync } = require('fs')
        unlinkSync(this.cacheFile)
      }
      this.config = null
      this.serverVersion = null
    } catch (error) {
      log.error('Failed to clear config cache:', error)
    }
  }
}

export { ConfigManager }
