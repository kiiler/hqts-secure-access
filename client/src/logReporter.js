/**
 * LogReporter - 客户端日志自动上报模块
 * 
 * 功能：
 * - 统一日志记录
 * - 错误/崩溃自动上报到服务端
 * - 用户可手动触发问题上报
 * - 日志本地也保留一份
 * 
 * 使用方式：
 * import logger from './logReporter'
 * logger.info('authManager', 'Login attempt')
 * logger.error('singboxAdapter', 'Failed to start', error.stack)
 * logger.reportIssue('authManager', 'Login failed', error.stack, '用户描述的问题')
 */

import log from 'electron-log'

class LogReporter {
  constructor() {
    this.buffer = []
    this.maxBuffer = 50
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
   * 发送日志到服务端（通过 IPC 到主进程统一上报）
   */
  async reportToServer(level, source, message, stack = '') {
    const entry = {
      level,
      source,
      message,
      stack,
      timestamp: new Date().toISOString(),
      userId: this.userId,
      clientVersion: this.clientVersion
    }

    try {
      if (window.hqtsAPI && window.hqtsAPI.logs) {
        // 通过 IPC 发到主进程，主进程再统一上报
        await window.hqtsAPI.logs.report(level, source, message, stack)
      } else {
        log.warn('[LogReporter] IPC not available')
      }
      log.info(`[LogReporter] [${level}] ${source} - ${message}`)
    } catch (e) {
      log.warn('[LogReporter] Report failed:', e.message)
    }
  }

  /**
   * 设置全局错误处理器
   */
  setupGlobalHandlers() {
    // 捕获未处理的 Promise 拒绝
    window.addEventListener('unhandledrejection', (event) => {
      const error = event.reason
      this.error('system', 'Unhandled Promise Rejection', error?.stack || String(error))
      event.preventDefault()
    })

    // 捕获全局 JS 错误
    window.addEventListener('error', (event) => {
      const error = event.error
      this.error('system', 'Uncaught Error', error?.stack || event.message)
      event.preventDefault()
    })

    log.info('[LogReporter] Initialized')
  }

  /**
   * 记录 error 级别日志
   */
  error(source, message, stack = '') {
    log.error(`[${source}] ${message}`, stack)
    this.reportToServer('error', source, message, stack)
  }

  /**
   * 记录 warn 级别日志
   */
  warn(source, message) {
    log.warn(`[${source}] ${message}`)
    this.reportToServer('warn', source, message)
  }

  /**
   * 记录 info 级别日志（不上报）
   */
  info(source, message) {
    log.info(`[${source}] ${message}`)
  }

  /**
   * 记录 debug 级别日志（不上报）
   */
  debug(source, message) {
    log.debug(`[${source}] ${message}`)
  }

  /**
   * 手动上报一条问题（用户点击"报告问题"时调用）
   * @param {string} source - 来源模块
   * @param {string} message - 问题描述
   * @param {string} stack - 错误堆栈
   * @param {string} description - 用户补充描述
   */
  async reportIssue(source, message, stack = '', description = '') {
    try {
      if (window.hqtsAPI && window.hqtsAPI.logs) {
        await window.hqtsAPI.logs.reportIssue(source, message, stack, description)
      }
      log.info('[LogReporter] Issue reported by user')
      return { success: true }
    } catch (e) {
      log.warn('[LogReporter] Issue report failed:', e.message)
      return { success: false, error: e.message }
    }
  }
}

// 导出单例
export default new LogReporter()
