import { app } from 'electron'
import { join } from 'path'
import { existsSync, writeFileSync, mkdirSync } from 'fs'
import { spawn, execSync } from 'child_process'
import log from 'electron-log'

/**
 * SingboxAdapter - sing-box 运行层适配器
 *
 * 职责：
 * - 管理sing-box二进制文件的生命周期
 * - 接收config.json并启动
 * - 提供启停reload能力
 * - 状态查询
 * - 版本管理
 */

/**
 * sing-box 版本配置
 * 注意：必须与服务端节点配置使用的 sing-box 版本兼容
 */
const SINGBOX_VERSION = '1.9.4'
const SINGBOX_DOWNLOAD_BASE = 'https://github.com/SagerNet/sing-box/releases/download'

class SingboxAdapter {
  constructor() {
    this.process = null
    this.configPath = null
    this.isRunning = false

    // sing-box二进制路径
    this.binDir = join(app.getPath('userData'), 'bin')
    if (!existsSync(this.binDir)) {
      mkdirSync(this.binDir, { recursive: true })
    }
    this.binPath = join(this.binDir, 'sing-box.exe')
    this.configFile = join(app.getPath('userData'), 'config.json')
  }

  /**
   * 获取当前 sing-box 版本
   */
  getVersion() {
    return SINGBOX_VERSION
  }

  /**
   * 检查sing-box是否已安装
   */
  async isInstalled() {
    // 检查本地二进制
    if (existsSync(this.binPath)) {
      return true
    }
    
    // 检查系统PATH
    try {
      execSync('sing-box --version', { stdio: 'ignore' })
      return true
    } catch {
      return false
    }
  }

  /**
   * 下载sing-box二进制（Windows）
   */
  async downloadBinary() {
    log.info('Downloading sing-box binary...')
    
    // sing-box官方下载地址（需要替换为实际版本）
    const version = '1.9.4'
    const downloadUrl = `https://github.com/SagerNet/sing-box/releases/download/v${version}/sing-box-${version}-windows-amd64.zip`
    
    try {
      const downloadPath = join(this.binDir, 'sing-box.zip')
      
      // 使用PowerShell下载
      const psScript = `
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri '${downloadUrl}' -OutFile '${downloadPath}'
      `
      
      const { writeFileSync } = require('fs')
      const psFile = join(this.binDir, 'download.ps1')
      writeFileSync(psFile, psScript)
      
      execSync(`powershell -ExecutionPolicy Bypass -File "${psFile}"`, {
        stdio: 'inherit'
      })
      
      // 解压
      // 使用PowerShell解压
      const extractScript = `
        Expand-Archive -Path '${downloadPath}' -DestinationPath '${this.binDir}' -Force
        Move-Item -Path '${this.binDir}\\sing-box-${version}-windows-amd64\\sing-box.exe' -Destination '${this.binPath}' -Force
        Remove-Item -Path '${downloadPath}' -Force
        Remove-Item -Path '${this.binDir}\\sing-box-${version}-windows-amd64' -Recurse -Force
        Remove-Item -Path '${psFile}' -Force
      `
      
      writeFileSync(psFile, extractScript)
      execSync(`powershell -ExecutionPolicy Bypass -File "${psFile}"`, {
        stdio: 'inherit'
      })
      
      log.info('sing-box binary downloaded successfully')
      return true
    } catch (error) {
      log.error('Failed to download sing-box binary:', error)
      return false
    }
  }

  /**
   * 启动sing-box
   * @param {Object} config - sing-box配置对象
   */
  async start(config) {
    if (this.isRunning) {
      log.warn('sing-box is already running')
      await this.stop()
    }

    // 写入配置
    writeFileSync(this.configFile, JSON.stringify(config, null, 2))
    this.configPath = this.configFile

    // 检查二进制
    if (!existsSync(this.binPath)) {
      log.info('sing-box binary not found, attempting to download...')
      const downloaded = await this.downloadBinary()
      if (!downloaded) {
        throw new Error('sing-box binary not available')
      }
    }

    return new Promise((resolve, reject) => {
      log.info('Starting sing-box...')

      this.process = spawn(this.binPath, [
        'run',
        '-c', this.configPath,
        '--disable-color'
      ], {
        stdio: ['ignore', 'pipe', 'pipe'],
        detached: false,
        windowsHide: true
      })

      let stderrData = ''
      let stdoutData = ''
      let startTimeout = null

      this.process.stderr.on('data', (data) => {
        const text = data.toString()
        stderrData += text
        // 监听错误关键词
        if (text.match(/connection refused|timeout|dial error/i)) {
          log.warn('[sing-box] Connection error detected:', text.substring(0, 100))
        }
      })

      this.process.stdout.on('data', (data) => {
        const line = data.toString().trim()
        stdoutData += line + '\n'
        if (line) {
          log.info('[sing-box]', line)
        }
      })

      this.process.on('error', (error) => {
        log.error('sing-box process error:', error)
        this.isRunning = false
        if (startTimeout) clearTimeout(startTimeout)
        reject(error)
      })

      this.process.on('exit', (code, signal) => {
        log.info(`sing-box exited with code ${code}, signal ${signal}`)
        this.isRunning = false
        this.process = null
        // 触发退出事件，供上层处理故障转移
        if (this.onExit) {
          this.onExit(code, signal)
        }
      })

      // 等待一小段时间确认启动成功
      startTimeout = setTimeout(() => {
        if (this.process && !this.process.killed) {
          this.isRunning = true
          this.lastStartTime = Date.now()
          log.info('sing-box started successfully')
          resolve(true)
        } else {
          // 检查错误信息
          if (stderrData.includes('not found') || stderrData.includes('cannot')) {
            reject(new Error('sing-box binary not found or not executable'))
          } else if (stderrData.match(/connection refused|timeout/i)) {
            reject(new Error('NODE_CONNECTION_FAILED'))
          } else {
            reject(new Error(stderrData || 'Failed to start sing-box'))
          }
        }
      }, 3000) // 增加超时时间到3秒，给节点连接更多时间
    })
  }

  /**
   * 停止sing-box
   */
  async stop() {
    if (!this.process && !this.isRunning) {
      log.warn('sing-box is not running')
      return true
    }

    return new Promise((resolve) => {
      log.info('Stopping sing-box...')

      if (this.process) {
        this.process.on('exit', () => {
          log.info('sing-box stopped')
          this.isRunning = false
          this.process = null
          resolve(true)
        })

        // 尝试优雅退出
        this.process.kill('SIGTERM')
        
        // 5秒后强制kill
        setTimeout(() => {
          if (this.process) {
            this.process.kill('SIGKILL')
          }
          resolve(true)
        }, 5000)
      } else {
        // 尝试通过命令行停止
        try {
          execSync(`taskkill /F /IM sing-box.exe`, { stdio: 'ignore' })
        } catch {
          // 忽略错误
        }
        this.isRunning = false
        resolve(true)
      }
    })
  }

  /**
   * 重载配置
   * @param {Object} config - 新的sing-box配置
   */
  async reload(config) {
    log.info('Reloading sing-box config...')
    
    // 写入新配置
    writeFileSync(this.configFile, JSON.stringify(config, null, 2))
    
    if (!this.isRunning) {
      log.warn('sing-box is not running, starting with new config...')
      return await this.start(config)
    }

    // 尝试发送SIGHUP信号重载配置
    if (this.process) {
      try {
        // Windows不支持SIGHUP，使用restart方式
        await this.stop()
        await this.start(config)
        log.info('sing-box config reloaded successfully')
        return true
      } catch (error) {
        log.error('Failed to reload config:', error)
        throw error
      }
    }

    throw new Error('sing-box process not available')
  }

  /**
   * 获取运行状态
   */
  getStatus() {
    return {
      running: this.isRunning,
      pid: this.process?.pid || null,
      configPath: this.configPath
    }
  }
}

export { SingboxAdapter }
