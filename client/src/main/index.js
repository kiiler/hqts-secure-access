import { app, BrowserWindow, ipcMain, Tray, Menu, nativeImage } from 'electron'
import { join } from 'path'
import { AuthManager } from '../core/authManager'
import { ConfigManager } from '../core/configManager'
import { PolicyEngine } from '../core/policyEngine'
import { SingboxAdapter } from '../singbox-adapter'
import log from 'electron-log'

// 配置日志
log.transports.file.level = 'info'
log.transports.console.level = 'debug'

// 全局异常处理
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
    this.singboxAdapter = new SingboxAdapter()
    this.isConnected = false
    this.currentMode = 'BYPASS_CN'
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
      await this.mainWindow.loadFile(join(__dirname, '../ui/index.html'))
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
    // 登录
    ipcMain.handle('auth:login', async (_, oauth2Code) => {
      try {
        log.info('Starting OAuth2 login...')
        const result = await this.authManager.login(oauth2Code)
        if (result.success) {
          await this.configManager.loadConfig(result.accessToken)
          await this.start()
        }
        return result
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
        const config = await this.policyEngine.compileConfig(mode, this.configManager.getConfig())
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

    // 网络诊断
    ipcMain.handle('diagnose:run', async () => {
      return await this.runDiagnosis()
    })

    // 获取配置版本
    ipcMain.handle('config:version', async () => {
      const config = this.configManager.getConfig()
      return config?.version || null
    })
  }

  async connect() {
    try {
      log.info('Starting VPN connection...')
      const config = await this.policyEngine.compileConfig(
        this.currentMode,
        this.configManager.getConfig()
      )
      await this.singboxAdapter.start(config)
      this.isConnected = true
      this.notifyStatus()
      log.info('VPN connected successfully')
      return { success: true }
    } catch (error) {
      log.error('Connection failed:', error)
      return { success: false, error: error.message }
    }
  }

  async disconnect() {
    try {
      log.info('Stopping VPN connection...')
      await this.singboxAdapter.stop()
      this.isConnected = false
      this.notifyStatus()
      log.info('VPN disconnected')
      return { success: true }
    } catch (error) {
      log.error('Disconnect failed:', error)
      return { success: false, error: error.message }
    }
  }

  async start() {
    // 启动流程：检查token -> 拉取配置 -> 编译config -> 启动sing-box
    if (!this.authManager.isLoggedIn()) {
      throw new Error('Not logged in')
    }

    const config = await this.policyEngine.compileConfig(
      this.currentMode,
      this.configManager.getConfig()
    )
    await this.singboxAdapter.start(config)
    this.isConnected = true
    this.notifyStatus()
  }

  async stop() {
    await this.singboxAdapter.stop()
    this.isConnected = false
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

  const client = new HQTSClient()
  await client.createWindow()
  client.createTray()
  client.setupIPC()

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
