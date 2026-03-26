import { app } from 'electron'
import { join } from 'path'
import { writeFileSync, existsSync, readFileSync, mkdirSync } from 'fs'
import log from 'electron-log'

/**
 * AuthManager - 负责OAuth2登录态管理
 * 
 * 职责：
 * - 与泛微OA OAuth2对接
 * - Token存储/刷新
 * - 安全存储（使用Keytar或文件加密）
 */
class AuthManager {
  constructor() {
    this.accessToken = null
    this.refreshToken = null
    this.userInfo = null
    this.tokenExpiry = null
    
    // 存储路径
    this.storageDir = join(app.getPath('userData'), 'auth')
    if (!existsSync(this.storageDir)) {
      mkdirSync(this.storageDir, { recursive: true })
    }
    this.tokenFile = join(this.storageDir, 'tokens.json')
    
    // 加载已保存的token
    this.loadStoredTokens()
  }

  /**
   * 加载本地存储的tokens
   */
  loadStoredTokens() {
    try {
      if (existsSync(this.tokenFile)) {
        const data = JSON.parse(readFileSync(this.tokenFile, 'utf-8'))
        this.accessToken = data.accessToken
        this.refreshToken = data.refreshToken
        this.userInfo = data.userInfo
        this.tokenExpiry = data.tokenExpiry
        
        // 检查是否过期
        if (this.tokenExpiry && Date.now() > this.tokenExpiry) {
          log.info('Token expired, attempting refresh...')
          this.refreshAccessToken()
        }
      }
    } catch (error) {
      log.error('Failed to load stored tokens:', error)
    }
  }

  /**
   * 保存tokens到本地
   */
  saveTokens() {
    try {
      const data = {
        accessToken: this.accessToken,
        refreshToken: this.refreshToken,
        userInfo: this.userInfo,
        tokenExpiry: this.tokenExpiry
      }
      writeFileSync(this.tokenFile, JSON.stringify(data, null, 2))
      log.info('Tokens saved successfully')
    } catch (error) {
      log.error('Failed to save tokens:', error)
    }
  }

  /**
   * OAuth2登录
   * @param {string} oauth2Code - 从泛微OA获取的授权码
   */
  async login(oauth2Code) {
    try {
      log.info('Processing OAuth2 login...')
      
      // TODO: 替换为实际的泛微OA OAuth2接口地址
      const response = await fetch('https://oauth2.hqts.cn/token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          grant_type: 'authorization_code',
          code: oauth2Code,
          client_id: 'hqts-secure-access-client',
          redirect_uri: 'hqts://oauth/callback'
        })
      })

      if (!response.ok) {
        throw new Error(`OAuth2 error: ${response.status}`)
      }

      const data = await response.json()
      
      this.accessToken = data.access_token
      this.refreshToken = data.refresh_token
      this.userInfo = data.user_info
      // token过期时间，默认1小时
      this.tokenExpiry = Date.now() + (data.expires_in || 3600) * 1000
      
      this.saveTokens()
      
      log.info('Login successful, user:', this.userInfo)
      
      return {
        success: true,
        accessToken: this.accessToken,
        userInfo: this.userInfo
      }
    } catch (error) {
      log.error('Login failed:', error)
      return {
        success: false,
        error: error.message
      }
    }
  }

  /**
   * 刷新AccessToken
   */
  async refreshAccessToken() {
    if (!this.refreshToken) {
      log.warn('No refresh token available')
      return false
    }

    try {
      // TODO: 替换为实际的泛微OA OAuth2接口地址
      const response = await fetch('https://oauth2.hqts.cn/token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          grant_type: 'refresh_token',
          refresh_token: this.refreshToken,
          client_id: 'hqts-secure-access-client'
        })
      })

      if (!response.ok) {
        throw new Error(`Token refresh failed: ${response.status}`)
      }

      const data = await response.json()
      
      this.accessToken = data.access_token
      if (data.refresh_token) {
        this.refreshToken = data.refresh_token
      }
      this.tokenExpiry = Date.now() + (data.expires_in || 3600) * 1000
      
      this.saveTokens()
      
      log.info('Token refreshed successfully')
      return true
    } catch (error) {
      log.error('Token refresh failed:', error)
      // 刷新失败，清除登录状态
      await this.clearAuth()
      return false
    }
  }

  /**
   * 登出
   */
  async logout() {
    try {
      // 通知服务端使token失效
      if (this.accessToken) {
        await fetch('https://oauth2.hqts.cn/revoke', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${this.accessToken}`
          }
        })
      }
    } catch (error) {
      log.error('Logout notification failed:', error)
    } finally {
      await this.clearAuth()
    }
  }

  /**
   * 清除认证信息
   */
  async clearAuth() {
    this.accessToken = null
    this.refreshToken = null
    this.userInfo = null
    this.tokenExpiry = null
    
    try {
      if (existsSync(this.tokenFile)) {
        const { unlinkSync } = await import('fs')
        unlinkSync(this.tokenFile)
      }
    } catch (error) {
      log.error('Failed to clear token file:', error)
    }
  }

  /**
   * 检查是否已登录
   */
  isLoggedIn() {
    return !!this.accessToken && !!this.userInfo
  }

  /**
   * 获取用户信息
   */
  getUserInfo() {
    return this.userInfo
  }

  /**
   * 获取AccessToken
   */
  getAccessToken() {
    return this.accessToken
  }
}

export { AuthManager }
