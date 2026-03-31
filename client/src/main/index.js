import { app, BrowserWindow, ipcMain, Tray, Menu, nativeImage, shell } from 'electron'
import { join } from 'path'
import { AuthManager, CAS_CONFIG } from '../core/authManager'
import { ConfigManager } from '../core/configManager'
import { PolicyEngine } from '../core/policyEngine'
import { NodeHealthMonitor } from '../core/nodeHealthMonitor'
import { SingboxAdapter } from '../singbox-adapter'
import { AutoUpdater } from '../core/autoUpdater'
import log from 'electron-log'
import { API_CONFIG, getApiUrl } from '../config.js'

// 配置日志
log.transports.file.level = 'info'
log.transports.console.level = 'debug'

// 主进程本地日志记录（不上报到服务器，由用户手动报告）
process.on('uncaughtException', (error) => {
  log.error('Uncaught Exception:', error)
})

process.on('unhandledRejection', (reason) => {
  log.error('Unhandled Rejection:', reason)
})

class HQTSClient {
  constructor() {
    this.mainWindow = null
    this.tray = null
    this.authManager = new AuthManager()
    this.configManager = new ConfigManager()
    this.policyEngine = new PolicyEngine()
    this.nodeHealthMonitor = new NodeHealthMonitor()
    this.singboxAdapter = new SingboxAdapter()
    this.autoUpdater = new AutoUpdater()
    this.isConnected = false
    this.currentMode = 'BYPASS_CN'

    // 设置节点健康监控回调
    this.setupHealthMonitorCallbacks()

    // 设置自动更新回调
    this.setupAutoUpdaterCallbacks()
  }

  setupAutoUpdaterCallbacks() {
    this.autoUpdater.onUpdateAvailable = (info) => {
      log.info('Update available:', info.version)
      // 通知UI有可用更新
      if (this.mainWindow) {
        this.mainWindow.webContents.send('update:available', info)
      }
    }

    this.autoUpdater.onUpdateError = (error) => {
      log.warn('Update check failed:', error.message)
    }
  }

  setupHealthMonitorCallbacks() {
    // 当所有节点不可用时的回调
    this.nodeHealthMonitor.onAllNodesUnhealthy = async (retryCount) => {
      log.info(`All nodes unhealthy, retry attempt ${retryCount}`)
      // 尝试拉取新配置
      if (this.authManager.isLoggedIn()) {
        const success = await this.configManager.loadConfig(this.authManager.getAccessToken())
        if (success) {
          // 新配置可能包含新节点，重新初始化健康监控
          const nodes = this.configManager.getAllNodes()
          this.nodeHealthMonitor.init(nodes)
          return nodes.length > 0
        }
      }
      return false
    }

    // 当节点健康状态变化时的回调
    this.nodeHealthMonitor.onNodeHealthChanged = (healthyNodes) => {
      log.info('Node health changed, healthy nodes:', healthyNodes.length)
    }
  }

  async createWindow() {
    this.mainWindow = new BrowserWindow({
      width: 420,
      height: 520,
      resizable: false,
      maximizable: false,
      fullscreenable: false,
      autoHideMenuBar: true,
      frame: true,
      webPreferences: {
        preload: join(__dirname, '../preload/index.js'),
        sandbox: false,
        contextIsolation: true,
        nodeIntegration: false
      }
    })

    // 加载UI
    if (process.env.ELECTRON_RENDERER_URL) {
      await this.mainWindow.loadURL(process.env.ELECTRON_RENDERER_URL)
    } else {
      await this.mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
    }

    log.info('Main window created')
  }

  createTray() {
    // 创建系统托盘图标
    const iconPath = join(__dirname, '../../assets/icon.png')
    let trayIcon
    try {
      trayIcon = nativeImage.createFromPath(iconPath)
      if (trayIcon.isEmpty()) {
        trayIcon = nativeImage.createEmpty()
      }
    } catch {
      trayIcon = nativeImage.createEmpty()
    }

    this.tray = new Tray(trayIcon)

    const contextMenu = Menu.buildFromTemplate([
      {
        label: '显示窗口',
        click: () => {
          this.mainWindow?.show()
        }
      },
      { type: 'separator' },
      {
        label: '连接',
        click: () => this.connect()
      },
      {
        label: '断开',
        click: () => this.disconnect()
      },
      { type: 'separator' },
      {
        label: '退出',
        click: () => {
          app.quit()
        }
      }
    ])

    this.tray.setToolTip('HQTS Secure Access')
    this.tray.setContextMenu(contextMenu)

    this.tray.on('click', () => {
      this.mainWindow?.show()
    })
  }

  setupIPC() {
    // 获取 CAS 登录 URL
    ipcMain.handle('auth:getCasLoginUrl', async () => {
      return this.authManager.getCasLoginUrl()
    })

    // 登录 - 在隐藏窗口中打开 CAS 登录页面
    ipcMain.handle('auth:login', async () => {
      try {
        log.info('Starting CAS login...')
        
        return new Promise((resolve) => {
          // 构建 CAS 登录 URL
          const casLoginUrl = this.authManager.getCasLoginUrl()
          log.info('Opening CAS login URL:', casLoginUrl)
          
          // 创建隐藏的登录窗口
          const loginWindow = new BrowserWindow({
            width: 480,
            height: 640,
            resizable: false,
            autoHideMenuBar: true,
            webPreferences: {
              sandbox: false,
              contextIsolation: true,
              nodeIntegration: false
            }
          })

          // 监听 URL 变化，检查是否包含 ticket
          loginWindow.webContents.on('will-navigate', async (event, url) => {
            log.info('Login window navigating to:', url)
            
            // 检查是否是回调 URL
            if (url.startsWith(CAS_CONFIG.serviceUrl)) {
              event.preventDefault()
              
              // 从 URL 中提取 ticket
              const urlObj = new URL(url)
              const ticket = urlObj.searchParams.get('ticket')
              
              if (ticket) {
                log.info('Got CAS ticket, closing login window...')
                loginWindow.close()
                
                // 处理 ticket
                try {
                  const result = await this.authManager.handleCasCallback(ticket)
                  if (result.success) {
                    await this.configManager.loadConfig(result.accessToken)
                    await this.start()
                    resolve(result)
                  } else {
                    resolve(result)
                  }
                } catch (error) {
                  resolve({ success: false, error: error.message })
                }
              }
            }
          })

          // 登录页面加载完成后检查
          loginWindow.webContents.on('did-finish-load', () => {
            log.info('Login window finished loading')
          })

          // 窗口关闭时，如果还没完成登录，认为用户取消了
          loginWindow.on('closed', () => {
            log.info('Login window closed')
            resolve({ success: false, error: 'Login cancelled' })
          })

          // 加载 CAS 登录页面
          loginWindow.loadURL(casLoginUrl)
        })
      } catch (error) {
        log.error('Login failed:', error)
        return { success: false, error: error.message }
      }
    })

    // 登出
    ipcMain.handle('auth:logout', async () => {
      try {
        await this.stop()
        await this.authManager.logout()
        this.nodeHealthMonitor.destroy()
        return { success: true }
      } catch (error) {
        log.error('Logout failed:', error)
        return { success: false, error: error.message }
      }
    })

    // 获取登录状态
    ipcMain.handle('auth:status', async () => {
      return this.authManager.isLoggedIn()
    })

    // 获取用户信息
    ipcMain.handle('auth:userInfo', async () => {
      return this.authManager.getUserInfo()
    })

    // 连接
    ipcMain.handle('vpn:connect', async () => {
      return await this.connect()
    })

    // 断开
    ipcMain.handle('vpn:disconnect', async () => {
      return await this.disconnect()
    })

    // 获取连接状态
    ipcMain.handle('vpn:status', async () => {
      return {
        connected: this.isConnected,
        mode: this.currentMode
      }
    })

    // 切换模式
    ipcMain.handle('vpn:switchMode', async (_, mode) => {
      try {
        log.info(`Switching mode to: ${mode}`)
        this.currentMode = mode
        
        // 重新选择节点并编译配置
        const healthyNodes = this.nodeHealthMonitor.getHealthyNodes(this.configManager.getAllNodes())
        if (healthyNodes.length === 0) {
          return { success: false, error: 'No healthy nodes available' }
        }
        
        const selectedNode = this.policyEngine.selectNode(healthyNodes)
        const config = await this.policyEngine.compileConfigWithNode(
          mode,
          this.configManager.getConfig(),
          selectedNode
        )
        await this.singboxAdapter.reload(config)
        this.notifyStatus()
        return { success: true }
      } catch (error) {
        log.error('Switch mode failed:', error)
        return { success: false, error: error.message }
      }
    })

    // 导出日志
    ipcMain.handle('logs:export', async () => {
      const logs = log.transports.file.getFile().path
      return { path: logs }
    })

    // 日志上报（渲染进程通过此通道上报日志到服务端）
    ipcMain.handle('logs:report', async (_, { level, source, message, stack }) => {
      await reportLogToServer(level, source, message, stack)
      return { success: true }
    })

    // 手动报告问题
    ipcMain.handle('logs:reportIssue', async (_, { source, message, stack, description }) => {
      const fullMessage = description
        ? `[用户描述] ${description}\n[原消息] ${message}`
        : message
      await reportLogToServer('error', source, fullMessage, stack)
      return { success: true }
    })

    // 网络诊断
    ipcMain.handle('diagnose:run', async () => {
      return await this.runDiagnosis()
    })

    // 获取配置版本
    ipcMain.handle('config:version', async () => {
      const config = this.configManager.getConfig()
      return config?.version || null
    })

    // 获取客户端版本
    ipcMain.handle('app:version', async () => {
      return {
        clientVersion: app.getVersion(),
        singboxVersion: this.singboxAdapter.getVersion()
      }
    })

    // 检查更新
    ipcMain.handle('app:checkUpdate', async () => {
      return await this.autoUpdater.checkForUpdates()
    })

    // 打开更新下载页面
    ipcMain.handle('app:downloadUpdate', async (_, url) => {
      this.autoUpdater.openDownloadPage(url)
      return { success: true }
    })
  }

  async connect() {
    try {
      log.info('Starting VPN connection...')
      
      // 获取健康节点列表
      const nodes = this.configManager.getAllNodes()
      const healthyNodes = this.nodeHealthMonitor.getHealthyNodes(nodes)
      
      if (healthyNodes.length === 0) {
        // 没有健康节点，开始重试模式
        log.warn('No healthy nodes, starting retry mode...')
        this.nodeHealthMonitor.startRetryMode(
          async () => {
            await this.configManager.loadConfig(this.authManager.getAccessToken())
          },
          this.authManager.getAccessToken()
        )
        return { success: false, error: 'No healthy nodes available, retrying...' }
      }
      
      // 选择最优节点并编译配置
      const selectedNode = this.policyEngine.selectNode(healthyNodes)
      const serverConfig = this.configManager.getConfig()
      const config = await this.policyEngine.compileConfigWithNode(
        this.currentMode,
        serverConfig,
        selectedNode
      )
      
      // 设置 sing-box 下载配置（从服务端配置获取）
      if (serverConfig?.singbox) {
        this.singboxAdapter.setDownloadConfig(
          serverConfig.singbox.version,
          serverConfig.singbox.download_url
        )
      }
      
      await this.singboxAdapter.start(config)
      this.nodeHealthMonitor.markNodeSuccess(selectedNode.id)
      this.isConnected = true
      this.notifyStatus()
      log.info('VPN connected successfully')
      return { success: true }
    } catch (error) {
      log.error('Connection failed:', error)
      
      // 连接失败，标记节点失败
      const currentNodeId = this.policyEngine.getCurrentNodeId()
      if (currentNodeId) {
        this.nodeHealthMonitor.markNodeFailed(currentNodeId)
      }
      
      // 尝试故障转移
      await this.handleFailover()
      
      return { success: false, error: error.message }
    }
  }

  async disconnect() {
    try {
      log.info('Stopping VPN connection...')
      await this.singboxAdapter.stop()
      this.isConnected = false
      this.nodeHealthMonitor.stopRetryMode()
      this.notifyStatus()
      log.info('VPN disconnected')
      return { success: true }
    } catch (error) {
      log.error('Disconnect failed:', error)
      return { success: false, error: error.message }
    }
  }

  async start() {
    // 启动流程：检查token -> 拉取配置 -> 探测节点健康 -> 选择节点 -> 启动sing-box
    if (!this.authManager.isLoggedIn()) {
      throw new Error('Not logged in')
    }

    // 确保有配置
    if (!this.configManager.getConfig()) {
      await this.configManager.loadConfig(this.authManager.getAccessToken())
    }

    // 初始化节点健康监控
    const nodes = this.configManager.getAllNodes()
    this.nodeHealthMonitor.init(nodes)

    // 启动定时拉取配置（每5分钟）
    this.configManager.startPeriodicFetch(
      this.authManager.getAccessToken(),
      5 * 60 * 1000
    )

    // 立即尝试连接
    await this.connect()
  }

  /**
   * 处理故障转移
   * 当当前节点连接失败时，自动切换到下一个可用节点
   */
  async handleFailover() {
    const allNodes = this.configManager.getAllNodes()
    const healthyNodes = this.nodeHealthMonitor.getHealthyNodes(allNodes)
    
    if (healthyNodes.length === 0) {
      log.error('Failover failed: no healthy nodes available')
      // 开始重试模式
      this.nodeHealthMonitor.startRetryMode(
        async () => {
          await this.configManager.loadConfig(this.authManager.getAccessToken())
        },
        this.authManager.getAccessToken()
      )
      return false
    }

    // 获取当前节点ID
    const currentNodeId = this.policyEngine.getCurrentNodeId()
    
    // 找到下一个可用的健康节点
    const currentIndex = healthyNodes.findIndex(n => n.id === currentNodeId)
    const nextIndex = (currentIndex + 1) % healthyNodes.length
    const nextNode = healthyNodes[nextIndex]

    log.info(`Failover: switching from ${currentNodeId} to ${nextNode.id}`)

    // 重新编译并启动
    try {
      const serverConfig = this.configManager.getConfig()
      const config = await this.policyEngine.compileConfigWithNode(
        this.currentMode,
        serverConfig,
        nextNode
      )
      
      if (serverConfig?.singbox) {
        this.singboxAdapter.setDownloadConfig(
          serverConfig.singbox.version,
          serverConfig.singbox.download_url
        )
      }
      
      await this.singboxAdapter.start(config)
      this.nodeHealthMonitor.markNodeSuccess(nextNode.id)
      this.isConnected = true
      this.notifyStatus()
      log.info('Failover successful')
      return true
    } catch (error) {
      log.error('Failover failed:', error)
      // 标记当前节点失败，继续尝试下一个
      this.nodeHealthMonitor.markNodeFailed(nextNode.id)
      return await this.handleFailover()
    }
  }

  async stop() {
    await this.singboxAdapter.stop()
    this.isConnected = false
    this.nodeHealthMonitor.stopRetryMode()
    this.notifyStatus()
  }

  notifyStatus() {
    this.mainWindow?.webContents.send('vpn:statusChanged', {
      connected: this.isConnected,
      mode: this.currentMode
    })
  }

  async runDiagnosis() {
    const diagnosis = {
      timestamp: new Date().toISOString(),
      singboxInstalled: await this.singboxAdapter.isInstalled(),
      configValid: this.configManager.getConfig() !== null,
      authValid: this.authManager.isLoggedIn(),
      nodeHealth: this.nodeHealthMonitor.getStatusSummary(),
      dnsLeakTest: 'pending',
      connectionTest: 'pending'
    }

    // DNS泄露测试 - 简单检测
    try {
      const dnsResult = await fetch('https://dns.google/resolve?name=myip.opendns.com&type=A')
      if (dnsResult.ok) {
        diagnosis.dnsLeakTest = 'pass'
      } else {
        diagnosis.dnsLeakTest = 'fail'
      }
    } catch {
      diagnosis.dnsLeakTest = 'error'
    }

    // 连接测试
    if (this.isConnected) {
      diagnosis.connectionTest = 'connected'
    } else {
      diagnosis.connectionTest = 'disconnected'
    }

    return diagnosis
  }
}

// 应用入口
app.whenReady().then(async () => {
  log.info('HQTS Secure Access Client starting...')
  log.info('Client version:', app.getVersion())

  const client = new HQTSClient()
  await client.createWindow()
  client.createTray()
  client.setupIPC()

  // 启动时检查一次更新
  client.autoUpdater.startPeriodicCheck()

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      client.createWindow()
    }
  })
})

app.on('window-all-closed', () => {
  // 不退出，保持后台运行
})

app.on('before-quit', async () => {
  log.info('Application quitting...')
})
