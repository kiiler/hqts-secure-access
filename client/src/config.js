/**
 * 客户端配置文件
 * 所有需要修改的服务器地址都在这里
 */

export const API_CONFIG = {
  // API 服务器地址（服务端部署的地址）
  apiBaseUrl: 'http://43.133.255.232:8080',
  
  // CAS SSO 认证服务器地址（兼容原 CAS_CONFIG.casServerUrl）
  casServerUrl: 'https://hubportaltest.hqts.cn',
  
  // 客户端本地回调地址（用于 CAS 认证完成后跳转）
  serviceUrl: 'hqts://auth/callback',
  
  // 是否验证 SSL 证书（生产环境应为 true）
  rejectUnauthorized: false,
  
  // 配置更新检查间隔（毫秒）
  configUpdateInterval: 5 * 60 * 1000, // 5分钟
}

// 便捷方法
export function getApiUrl(path) {
  return `${API_CONFIG.apiBaseUrl}${path}`
}
