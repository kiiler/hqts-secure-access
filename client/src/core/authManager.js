import { app } from 'electron'
import { join } from 'path'
import { writeFileSync, existsSync, readFileSync, mkdirSync } from 'fs'
import log from 'electron-log'

/**
 * AuthManager - 负责CAS认证登录态管理
 * 
 * 职责：
 * - CAS 认证流程处理
 * - Ticket 验证和用户信息获取
 * - Token存储/刷新
 * - 安全存储
 * 
 * CAS 认证流程：
 * 1. 重定向到 CAS Server 登录页
 * 2. 用户登录成功后 CAS 返回 ticket
 * 3. 客户端用 ticket 调用 /cas/serviceValidate 获取用户信息
 * 4. 获取到用户信息后生成内部 token
 */

/**
 * CAS 认证配置
 */
const CAS_CONFIG = {
  // CAS Server 地址 - https://hubportaltest.hqts.cn
  casServerUrl: 'https://hubportaltest.hqts.cn',
  // 客户端回调地址
  serviceUrl: 'hqts://auth/callback',
  // 是否验证 SSL 证书（生产环境应设为 true）
  rejectUnauthorized: false
}

class AuthManager {
  constructor() {
    this.accessToken = null
    this.refreshToken = null
    this.userInfo = null
    this.tokenExpiry = null
    this.casTicket = null  // CAS ticket 存储
    
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
        tokenExpiry: this.tokenExpiry,
        casTicket: this.casTicket
      }
      writeFileSync(this.tokenFile, JSON.stringify(data, null, 2))
      log.info('Tokens saved successfully')
    } catch (error) {
      log.error('Failed to save tokens:', error)
    }
  }

  /**
   * 构建 CAS 登录 URL
   * 用户访问此 URL 进行登录
   */
  buildCasLoginUrl() {
    const params = new URLSearchParams({
      service: CAS_CONFIG.serviceUrl
    })
    return `${CAS_CONFIG.casServerUrl}/login?${params.toString()}`
  }

  /**
   * 处理 CAS 回调
   * 当 CAS 登录成功后，会带着 ticket 返回
   * @param {string} ticket - CAS 返回的 ticket
   */
  async handleCasCallback(ticket) {
    try {
      log.info('Handling CAS callback with ticket:', ticket.substring(0, 20) + '...')
      this.casTicket = ticket

      // 1. 用 ticket 换取用户信息 (CAS Service Validate)
      const userInfo = await this.validateTicket(ticket)
      if (!userInfo) {
        throw new Error('Ticket validation failed')
      }

      // 2. 用用户信息换取内部 token
      const result = await this.exchangeForInternalToken(userInfo)
      
      return result
    } catch (error) {
      log.error('CAS callback handling failed:', error)
      return {
        success: false,
        error: error.message
      }
    }
  }

  /**
   * 验证 CAS Ticket
   * 调用 CAS /serviceValidate 端点验证ticket并获取用户信息
   * @param {string} ticket - CAS ticket
   */
  async validateTicket(ticket) {
    try {
      // CAS Service Validate URL
      // 泛微OA的CAS通常遵循标准CAS协议
      const validateUrl = `${CAS_CONFIG.casServerUrl}/serviceValidate?service=${encodeURIComponent(CAS_CONFIG.serviceUrl)}&ticket=${ticket}`

      log.info('Validating ticket with CAS server...')

      // 注意：实际环境中需要处理 HTTPS 证书验证
      const response = await fetch(validateUrl, {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json'
        }
      })

      if (!response.ok) {
        throw new Error(`CAS validation failed: ${response.status}`)
      }

      const data = await response.json()

      // 解析 CAS 响应
      // 标准 CAS 响应格式：
      // <cas:serviceResponse>
      //   <cas:authenticationSuccess>
      //     <cas:user>username</cas:user>
      //     <cas:attributes>...</cas:attributes>
      //   </cas:authenticationSuccess>
      // </cas:serviceResponse>
      
      if (data.serviceResponse?.authenticationSuccess) {
        const success = data.serviceResponse.authenticationSuccess
        return {
          username: success.user,
          attributes: success.attributes || {},
          // 泛微OA通常会在attributes中返回更多用户信息
          name: success.attributes?.name || success.user,
          email: success.attributes?.email || `${success.user}@hqts.cn`,
          department: success.attributes?.department || '',
          groups: success.attributes?.groups || []
        }
      } else if (data.serviceResponse?.authenticationFailure) {
        const failure = data.serviceResponse.authenticationFailure
        throw new Error(`CAS auth failed: ${failure.code} - ${failure.description}`)
      }

      return null
    } catch (error) {
      log.error('Ticket validation error:', error)
      throw error
    }
  }

  /**
   * 用 CAS 用户信息换取内部 token
   * 这是我们自己的 token 系统，用于后续 API 调用
   * @param {Object} casUser - CAS 返回的用户信息
   */
  async exchangeForInternalToken(casUser) {
    try {
      // TODO: 替换为实际的服务端地址
      const response = await fetch('https://config.hqts.cn/api/v1/auth/cas/exchange', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          username: casUser.username,
          name: casUser.name,
          email: casUser.email,
          department: casUser.department,
          groups: casUser.groups,
          casTicket: this.casTicket
        })
      })

      if (!response.ok) {
        throw new Error(`Token exchange failed: ${response.status}`)
      }

      const data = await response.json()

      this.accessToken = data.accessToken
      this.refreshToken = data.refreshToken
      this.userInfo = data.userInfo || {
        id: casUser.username,
        name: casUser.name,
        email: casUser.email
      }
      this.tokenExpiry = Date.now() + (data.expiresIn || 3600) * 1000

      this.saveTokens()

      log.info('Internal token obtained successfully for user:', this.userInfo)

      return {
        success: true,
        accessToken: this.accessToken,
        userInfo: this.userInfo
      }
    } catch (error) {
      log.error('Token exchange failed:', error)
      throw error
    }
  }

  /**
   * 刷新 AccessToken
   */
  async refreshAccessToken() {
    if (!this.refreshToken) {
      log.warn('No refresh token available')
      return false
    }

    try {
      const response = await fetch('https://config.hqts.cn/api/v1/auth/refresh', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          refreshToken: this.refreshToken
        })
      })

      if (!response.ok) {
        throw new Error(`Token refresh failed: ${response.status}`)
      }

      const data = await response.json()

      this.accessToken = data.accessToken
      if (data.refreshToken) {
        this.refreshToken = data.refreshToken
      }
      this.tokenExpiry = Date.now() + (data.expiresIn || 3600) * 1000

      this.saveTokens()

      log.info('Token refreshed successfully')
      return true
    } catch (error) {
      log.error('Token refresh failed:', error)
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
        await fetch('https://config.hqts.cn/api/v1/auth/logout', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${this.accessToken}`
          }
        })
      }

      // 通知 CAS 登出（可选）
      if (this.casTicket) {
        try {
          await fetch(`${CAS_CONFIG.casServerUrl}/logout?service=${encodeURIComponent(CAS_CONFIG.serviceUrl)}`)
        } catch {
          // CAS logout 失败不影响本地登出
        }
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
    this.casTicket = null

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

  /**
   * 获取 CAS 登录 URL（用于打开浏览器）
   */
  getCasLoginUrl() {
    return this.buildCasLoginUrl()
  }
}

export { AuthManager, CAS_CONFIG }
