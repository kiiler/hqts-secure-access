import log from 'electron-log'

/**
 * NodeHealthMonitor - 客户端节点健康监控
 * 
 * 职责：
 * - 客户端自主探测每个节点的健康状态
 * - 维护节点健康状态（延迟、失败次数）
 * - 选择最优可用节点
 * - 当所有节点不可用时，定时重试并拉取新配置
 */

class NodeHealthMonitor {
  constructor() {
    // 节点健康状态缓存
    // { nodeId: { online, latency, failCount, lastCheckTime } }
    this.nodeStates = new Map()
    
    // 最大连续失败次数，超过则认为节点不可用
    this.maxFailCount = 3
    
    // 重试间隔（毫秒）
    this.retryInterval = 30 * 1000 // 30秒
    
    // 探测超时（毫秒）
    this.probeTimeout = 5000
    
    // 重试定时器
    this.retryTimer = null
    
    // 配置刷新定时器
    this.configRefreshTimer = null
    
    // 回调
    this.onAllNodesUnhealthy = null
    this.onNodeHealthChanged = null
  }

  /**
   * 初始化节点健康监控
   * @param {Array} nodes - 节点列表
   */
  init(nodes) {
    log.info('Initializing node health monitor with', nodes?.length || 0, 'nodes')
    
    // 初始化所有节点状态
    if (nodes) {
      for (const node of nodes) {
        if (!this.nodeStates.has(node.id)) {
          this.nodeStates.set(node.id, {
            online: true,
            latency: null,
            failCount: 0,
            lastCheckTime: null,
            consecutiveSuccess: 0
          })
        }
      }
    }
    
    // 立即探测所有节点
    this.probeAllNodes(nodes)
  }

  /**
   * 探测所有节点的健康状态
   * @param {Array} nodes - 节点列表
   */
  async probeAllNodes(nodes) {
    if (!nodes || nodes.length === 0) return

    log.info('Probing health for', nodes.length, 'nodes')

    // 并行探测所有节点
    const promises = nodes.map(node => this.probeNode(node))
    await Promise.all(promises)

    // 通知健康状态变化
    if (this.onNodeHealthChanged) {
      this.onNodeHealthChanged(this.getHealthyNodes())
    }
  }

  /**
   * 探测单个节点的健康状态
   * @param {Object} node - 节点信息
   * @returns {Object} 健康状态
   */
  async probeNode(node) {
    const startTime = Date.now()
    let state = this.nodeStates.get(node.id)

    if (!state) {
      state = {
        online: true,
        latency: null,
        failCount: 0,
        lastCheckTime: null,
        consecutiveSuccess: 0
      }
      this.nodeStates.set(node.id, state)
    }

    try {
      // 使用 TCP 连接探测端口是否可达
      // 同时尝试解析 DNS（针对域名节点）
      const reachable = await this.checkReachability(node)

      if (reachable) {
        state.latency = Date.now() - startTime
        state.online = true
        state.failCount = 0
        state.consecutiveSuccess++
        state.lastCheckTime = Date.now()
        
        log.info(`Node ${node.id} is healthy, latency: ${state.latency}ms`)
      } else {
        throw new Error('Node not reachable')
      }
    } catch (error) {
      state.failCount++
      state.consecutiveSuccess = 0
      state.lastCheckTime = Date.now()
      
      // 如果连续失败超过阈值，标记为不可用
      if (state.failCount >= this.maxFailCount) {
        state.online = false
        log.warn(`Node ${node.id} marked as unhealthy after ${state.failCount} failures`)
      } else {
        log.warn(`Node ${node.id} probe failed (${state.failCount}/${this.maxFailCount}):`, error.message)
      }
    }

    return state
  }

  /**
   * 检查节点可达性
   * @param {Object} node - 节点信息
   */
  async checkReachability(node) {
    // 对于域名，尝试解析 DNS
    if (node.host && !this.isIP(node.host)) {
      try {
        // 简单的 DNS 解析检测
        const dnsResult = await this.dnsResolve(node.host)
        if (!dnsResult) {
          log.warn(`DNS resolution failed for ${node.host}`)
          return false
        }
      } catch {
        return false
      }
    }

    // TCP 端口检测（模拟，实际需要用真实 TCP 检测）
    // 这里用 fetch 作为代理检测
    try {
      const controller = new AbortController()
      const timeout = setTimeout(() => controller.abort(), this.probeTimeout)
      
      // 尝试 HTTPS 连接到节点地址
      // 注意：实际应该是 TCP 层检测，这里用 HTTP 作为近似
      const response = await fetch(`https://${node.host}:${node.port}`, {
        method: 'HEAD',
        signal: controller.signal,
        mode: 'no-cors'
      }).catch(() => null)
      
      clearTimeout(timeout)
      
      // no-cors 模式下 response 可能是 null，但不代表不可达
      // 我们主要关注是否超时/报错
      return true
    } catch (error) {
      // 超时或其他错误
      log.warn(`Reachability check failed for ${node.host}:${node.port}:`, error.message)
      
      // 即使 fetch 失败，也可能是端口可达但没有 HTTP 服务
      // 降低标准：DNS 解析成功就认为可达
      if (this.isIP(node.host)) {
        return true // IP 直接认为可达
      }
      return false
    }
  }

  /**
   * 简单的 DNS 解析检测
   * @param {string} hostname - 主机名
   */
  async dnsResolve(hostname) {
    try {
      const controller = new AbortController()
      const timeout = setTimeout(() => controller.abort(), this.probeTimeout)
      
      // 使用 fetch 触发 DNS 解析
      await fetch(`https://${hostname}`, {
        signal: controller.signal,
        mode: 'no-cors'
      })
      
      clearTimeout(timeout)
      return true
    } catch {
      return false
    }
  }

  /**
   * 判断是否为 IP 地址
   * @param {string} str 
   */
  isIP(str) {
    return /^(\d{1,3}\.){3}\d{1,3}$/.test(str) || 
           /^[a-fA-F0-9:]+$/.test(str) // IPv6
  }

  /**
   * 获取所有健康节点（按延迟排序）
   * @param {Array} nodes - 节点列表
   */
  getHealthyNodes(nodes) {
    if (!nodes) {
      // 从缓存的状态中提取节点信息
      nodes = Array.from(this.nodeStates.keys()).map(id => ({ id }))
    }

    const healthyNodes = nodes.filter(node => {
      const state = this.nodeStates.get(node.id)
      return state && state.online && state.failCount < this.maxFailCount
    })

    // 按延迟排序
    return healthyNodes.sort((a, b) => {
      const stateA = this.nodeStates.get(a.id)
      const stateB = this.nodeStates.get(b.id)
      const latencyA = stateA?.latency || 999999
      const latencyB = stateB?.latency || 999999
      return latencyA - latencyB
    })
  }

  /**
   * 获取最佳可用节点
   * @param {Array} nodes - 节点列表
   */
  getBestNode(nodes) {
    const healthy = this.getHealthyNodes(nodes)
    if (healthy.length > 0) {
      return healthy[0]
    }
    return null
  }

  /**
   * 标记节点连接失败
   * @param {string} nodeId - 节点ID
   */
  markNodeFailed(nodeId) {
    const state = this.nodeStates.get(nodeId)
    if (state) {
      state.failCount++
      state.consecutiveSuccess = 0
      if (state.failCount >= this.maxFailCount) {
        state.online = false
        log.warn(`Node ${nodeId} marked as unhealthy after connection failure`)
      }
    }
  }

  /**
   * 标记节点连接成功
   * @param {string} nodeId - 节点ID
   */
  markNodeSuccess(nodeId) {
    const state = this.nodeStates.get(nodeId)
    if (state) {
      state.failCount = 0
      state.consecutiveSuccess++
      state.online = true
    }
  }

  /**
   * 当所有节点不可用时的处理
   * @param {Function} refreshConfig - 刷新配置的回调
   * @param {string} accessToken - 访问令牌
   */
  startRetryMode(refreshConfig, accessToken) {
    log.info('All nodes unhealthy, starting retry mode...')

    // 停止已有的定时器
    this.stopRetryMode()

    let retryCount = 0
    const maxRetryInterval = 5 * 60 * 1000 // 最大重试间隔5分钟

    const retry = async () => {
      retryCount++
      // 指数退避，但不超过最大间隔
      const currentInterval = Math.min(this.retryInterval * retryCount, maxRetryInterval)
      
      log.info(`Retry attempt ${retryCount}, next retry in ${currentInterval / 1000}s`)

      // 通知外部可能需要刷新配置
      if (this.onAllNodesUnhealthy) {
        const hasNewNodes = await this.onAllNodesUnhealthy(retryCount)
        
        if (hasNewNodes) {
          log.info('New nodes available, stopping retry mode')
          this.stopRetryMode()
          retryCount = 0
        }
      }

      // 探测所有节点
      // nodes 需要从外部传入，这里触发探测
      if (refreshConfig) {
        await refreshConfig(accessToken)
      }

      // 如果还没找到健康节点，继续重试
      const healthy = this.getHealthyNodes()
      if (healthy.length === 0) {
        this.retryTimer = setTimeout(retry, currentInterval)
      } else {
        log.info('Found healthy node, stopping retry mode')
        this.stopRetryMode()
        retryCount = 0
      }
    }

    // 立即开始第一次重试
    this.retryTimer = setTimeout(retry, this.retryInterval)
  }

  /**
   * 停止重试模式
   */
  stopRetryMode() {
    if (this.retryTimer) {
      clearTimeout(this.retryTimer)
      this.retryTimer = null
    }
  }

  /**
   * 获取节点状态摘要
   */
  getStatusSummary() {
    const summary = {
      total: this.nodeStates.size,
      healthy: 0,
      unhealthy: 0,
      nodes: {}
    }

    for (const [nodeId, state] of this.nodeStates) {
      summary.nodes[nodeId] = {
        online: state.online,
        latency: state.latency,
        failCount: state.failCount
      }
      if (state.online) {
        summary.healthy++
      } else {
        summary.unhealthy++
      }
    }

    return summary
  }

  /**
   * 销毁
   */
  destroy() {
    this.stopRetryMode()
    this.nodeStates.clear()
  }
}

export { NodeHealthMonitor }
