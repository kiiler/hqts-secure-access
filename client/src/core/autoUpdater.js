import { app } from 'electron'
import log from 'electron-log'
import { getApiUrl } from '../config.js'

/**
 * AutoUpdater - 客户端自动更新模块
 * 
 * 职责：
 * - 检查客户端更新
 * - 下载新版本
 * - 安装更新
 */

class AutoUpdater {
  constructor() {
    // 当前版本
    this.currentVersion = app.getVersion() || '1.0.0'
    
    // 更新服务器地址
    this.updateServerUrl = getApiUrl('/api/v1/version')
    
    // 检查更新间隔（毫秒）
    this.checkInterval = 60 * 60 * 1000 // 1小时
    this.checkTimer = null
    
    // 回调
    this.onUpdateAvailable = null
    this.onDownloadProgress = null
    this.onUpdateReady = null
    this.onUpdateError = null
  }

  /**
   * 设置更新服务器地址
   * @param {string} url - 更新服务器URL
   */
  setUpdateServer(url) {
    this.updateServerUrl = url
  }

  /**
   * 获取当前版本
   */
  getCurrentVersion() {
    return this.currentVersion
  }

  /**
   * 检查更新
   */
  async checkForUpdates() {
    try {
      log.info('Checking for updates...')

      const response = await fetch(this.updateServerUrl, {
        method: 'GET',
        headers: {
          'Accept': 'application/json'
        }
      })

      if (!response.ok) {
        log.warn('Update server returned status:', response.status)
        return { hasUpdate: false }
      }

      const data = await response.json()
      
      const latestVersion = data.version
      const downloadUrl = data.downloadUrl
      const releaseNotes = data.releaseNotes

      log.info(`Current: ${this.currentVersion}, Latest: ${latestVersion}`)

      const hasUpdate = this.compareVersions(latestVersion, this.currentVersion) > 0

      if (hasUpdate) {
        log.info('Update available:', latestVersion)
        
        if (this.onUpdateAvailable) {
          this.onUpdateAvailable({
            version: latestVersion,
            releaseNotes: releaseNotes,
            downloadUrl: downloadUrl
          })
        }
      }

      return {
        hasUpdate,
        currentVersion: this.currentVersion,
        latestVersion,
        downloadUrl,
        releaseNotes
      }
    } catch (error) {
      log.error('Failed to check for updates:', error)
      
      if (this.onUpdateError) {
        this.onUpdateError(error)
      }
      
      return { hasUpdate: false, error: error.message }
    }
  }

  /**
   * 比较版本号
   * @returns {number} 1: v1>v2, 0: v1=v2, -1: v1<v2
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
   * 开始定时检查更新
   */
  startPeriodicCheck() {
    log.info('Starting periodic update check...')

    // 立即检查一次
    this.checkForUpdates()

    // 设置定时器
    this.checkTimer = setInterval(() => {
      this.checkForUpdates()
    }, this.checkInterval)
  }

  /**
   * 停止定时检查
   */
  stopPeriodicCheck() {
    if (this.checkTimer) {
      clearInterval(this.checkTimer)
      this.checkTimer = null
      log.info('Stopped periodic update check')
    }
  }

  /**
   * 打开下载页面让用户手动下载
   * @param {string} url - 下载URL
   */
  openDownloadPage(url) {
    if (url) {
      require('electron').shell.openExternal(url)
    } else {
      // 默认打开 GitHub releases 页面
      require('electron').shell.openExternal(
        'https://github.com/kiiler/hqts-secure-access/releases'
      )
    }
  }
}

export { AutoUpdater }
