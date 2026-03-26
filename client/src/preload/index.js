import { contextBridge, ipcRenderer } from 'electron'

// 暴露安全的API给渲染进程
contextBridge.exposeInMainWorld('hqtsAPI', {
  // 认证
  auth: {
    // CAS 登录
    login: () => ipcRenderer.invoke('auth:login'),
    logout: () => ipcRenderer.invoke('auth:logout'),
    status: () => ipcRenderer.invoke('auth:status'),
    getUserInfo: () => ipcRenderer.invoke('auth:userInfo')
  },

  // VPN控制
  vpn: {
    connect: () => ipcRenderer.invoke('vpn:connect'),
    disconnect: () => ipcRenderer.invoke('vpn:disconnect'),
    status: () => ipcRenderer.invoke('vpn:status'),
    switchMode: (mode) => ipcRenderer.invoke('vpn:switchMode', mode),
    onStatusChanged: (callback) => {
      ipcRenderer.on('vpn:statusChanged', (_, status) => callback(status))
    }
  },

  // 日志
  logs: {
    export: () => ipcRenderer.invoke('logs:export')
  },

  // 诊断
  diagnose: {
    run: () => ipcRenderer.invoke('diagnose:run')
  },

  // 配置
  config: {
    getVersion: () => ipcRenderer.invoke('config:version')
  }
})
