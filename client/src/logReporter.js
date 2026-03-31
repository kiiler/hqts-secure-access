/**
 * LogReporter - 客户端日志手动上报模块
 * 
 * 功能：
 * - 统一日志记录到本地文件
 * - 用户主动点击"报告问题"才上报到服务端
 * - 不自动上报任何内容
 * 
 * 使用方式：
 * import logger from './logReporter'
 * logger.error('singboxAdapter', 'Failed to start', error.stack)
 * // 用户点击"报告问题"按钮时：
 * logger.reportIssue('singboxAdapter', '连接失败', error.stack, '用户补充的描述')
 */

import log from 'electron-log'

class LogReporter {
  constructor() {
    this.clientVersion = '1.0.0'
    this.userId = 'anonymous'
  }

  /**
   * 设置用户ID（登录后调用）
   */
  setUserId(userId) {
    this.userId = userId
  }

  /**
   * 设置客户端版本
   */
  setClientVersion(version) {
    this.clientVersion = version
  }

  /**
   * 记录 error 级别日志（本地记录，不上报）
   */
  error(source, message, stack = '') {
    log.error(`[${source}] ${message}`, stack ? `\n${stack}` : '')
  }

  /**
   * 记录 warn 级别日志（本地记录，不上报）
   */
  warn(source, message) {
    log.warn(`[${source}] ${message}`)
  }

  /**
   * 记录 info 级别日志（本地记录，不上报）
   */
  info(source, message) {
    log.info(`[${source}] ${message}`)
  }

  /**
   * 记录 debug 级别日志（本地记录，不上报）
   */
  debug(source, message) {
    log.debug(`[${source}] ${message}`)
  }

  /**
   * 手动上报一条问题到服务端（用户主动点击"报告问题"时调用）
   * @param {string} source - 来源模块
   * @param {string} message - 问题描述
   * @param {string} stack - 错误堆栈（可选）
   * @param {string} description - 用户补充描述（可选）
   */
  async reportIssue(source, message, stack = '', description = '') {
    const entry = {
      level: 'error',
      source,
      message,
      stack,
      description,
      timestamp: new Date().toISOString(),
      userId: this.userId,
      clientVersion: this.clientVersion
    }

    try {
      if (window.hqtsAPI && window.hqtsAPI.logs) {
        await window.hqtsAPI.logs.reportIssue(source, message, stack || '', description || '')
        return { success: true }
      } else {
        return { success: false, error: 'IPC不可用' }
      }
    } catch (e) {
      return { success: false, error: e.message }
    }
  }
}

// 导出单例
export default new LogReporter()
