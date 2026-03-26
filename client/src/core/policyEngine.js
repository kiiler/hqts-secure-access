import log from 'electron-log'

/**
 * PolicyEngine - 策略引擎
 * 
 * 职责：
 * - 分流模式 → 策略转换
 * - 策略 → sing-box 配置编译
 * - DNS策略处理
 */

/**
 * 分流模式枚举
 */
const Mode = {
  GLOBAL: 'GLOBAL',           // 全局模式：所有流量走代理
  BYPASS_CN: 'BYPASS_CN'      // 绕过大陆：中国IP直连，其他走代理
}

/**
 * DNS策略枚举
 */
const DNSPolicy = {
  STANDARD: 'STANDARD',       // 标准DNS
  CUSTOM: 'CUSTOM'            // 自定义DNS
}

/**
 * sing-box配置结构
 * @typedef {Object} SingboxConfig
 * @property {Object} dns - DNS配置
 * @property {Array} inbounds - 入站配置
 * @property {Array} outbounds - 出站配置
 * @property {Object} route - 路由配置
 */

class PolicyEngine {
  constructor() {
    this.currentMode = Mode.BYPASS_CN
  }

  /**
   * 编译sing-box配置
   * @param {string} mode - 分流模式
   * @param {Object} productConfig - 产品级配置
   * @returns {SingboxConfig}
   */
  async compileConfig(mode, productConfig) {
    if (!productConfig || !productConfig.nodes || productConfig.nodes.length === 0) {
      throw new Error('Invalid product config or no nodes available')
    }

    this.currentMode = mode
    log.info(`Compiling sing-box config for mode: ${mode}`)

    // 选择最优节点（这里简单选择最高优先级的节点）
    const selectedNode = this.selectNode(productConfig.nodes)
    log.info(`Selected node: ${selectedNode.id} (${selectedNode.host})`)

    // 根据模式编译配置
    const config = {
      dns: this.buildDNS(mode, productConfig),
      inbounds: this.buildInbounds(),
      outbounds: this.buildOutbounds(selectedNode),
      route: this.buildRoute(mode, selectedNode)
    }

    log.info('Sing-box config compiled successfully')
    return config
  }

  /**
   * 选择节点（简单策略：选择最高优先级且可用的节点）
   * @param {Array} nodes - 节点列表
   */
  selectNode(nodes) {
    // 按优先级排序，选择第一个
    const sorted = [...nodes].sort((a, b) => a.priority - b.priority)
    return sorted[0]
  }

  /**
   * 构建DNS配置
   * @param {string} mode - 分流模式
   * @param {Object} config - 产品配置
   */
  buildDNS(mode, config) {
    if (mode === Mode.GLOBAL) {
      // 全局模式：强制走代理DNS
      return {
        'servers': [
          {
            'tag': 'proxy',
            'type': 'dns',
            'address': 'tls://1.1.1.1',
            'detour': 'proxy'
          },
          {
            'tag': 'local',
            'type': 'dns',
            'address': 'https://doh.pub/dns-query',
            'detour': 'direct'
          }
        ],
        'disableCache': false,
        'disableExpire': false
      }
    } else {
      // BYPASS_CN模式：分域名DNS
      return {
        'servers': [
          {
            'tag': 'proxy',
            'type': 'dns',
            'address': 'tls://1.1.1.1',
            'detour': 'proxy'
          },
          {
            'tag': 'cn',
            'type': 'dns',
            'address': 'https://doh.pub/dns-query',
            'detour': 'direct'
          },
          {
            'tag': 'block',
            'type': 'dns',
            'address': 'rcode://success'
          }
        ],
        'rules': [
          {
            'geosite': ['geosite-cn'],
            'server': 'cn'
          },
          {
            'geosite': ['category-ads-all'],
            'server': 'block'
          }
        ],
        'disableCache': false,
        'disableExpire': false
      }
    }
  }

  /**
   * 构建入站配置 - 仅 TUN 模式
   * 不使用 mixed 模式，因为企业 VPN 需要网络层完整代理
   */
  buildInbounds() {
    return [
      {
        'tag': 'tun-in',
        'type': 'tun',
        'interface_name': 'tun0',
        'mtu': 9000,
        'stack': 'system',
        'auto_route': true,
        'strict_route': true,
        'dns_independent': true,
        'sniff': true,
        'sniff_override_destination': true
      }
    ]
  }

  /**
   * 构建出站配置
   * @param {Object} node - 选中的节点
   */
  buildOutbounds(node) {
    const outbounds = [
      {
        'tag': 'direct',
        'type': 'direct'
      },
      {
        'tag': 'dns-out',
        'type': 'dns'
      }
    ]

    // 根据协议类型构建节点出站
    const nodeOutbound = this.buildNodeOutbound(node)
    outbounds.unshift(nodeOutbound)

    return outbounds
  }

  /**
   * 根据节点协议构建出站配置
   * @param {Object} node - 节点信息
   */
  buildNodeOutbound(node) {
    const { protocol, host, port, ...protocolConfig } = node

    switch (protocol) {
      case 'vmess': {
        return {
          'tag': 'proxy',
          'type': 'vmess',
          'server': host,
          'port': port,
          'uuid': protocolConfig.uuid || '',
          'alterId': protocolConfig.alterId || 0,
          'security': protocolConfig.security || 'auto',
          'tls': {
            'enabled': protocolConfig.tls || true,
            'serverName': host,
            'insecure': false
          }
        }
      }

      case 'vless': {
        return {
          'tag': 'proxy',
          'type': 'vless',
          'server': host,
          'port': port,
          'uuid': protocolConfig.uuid || '',
          'flow': protocolConfig.flow || '',
          'tls': {
            'enabled': true,
            'serverName': host,
            'insecure': false,
            'alpn': ['h2', 'http/1.1']
          }
        }
      }

      case 'trojan': {
        return {
          'tag': 'proxy',
          'type': 'trojan',
          'server': host,
          'port': port,
          'password': protocolConfig.password || '',
          'tls': {
            'enabled': true,
            'serverName': host,
            'insecure': false
          }
        }
      }

      case 'shadowsocks': {
        return {
          'tag': 'proxy',
          'type': 'shadowsocks',
          'server': host,
          'port': port,
          'method': protocolConfig.method || 'aes-256-gcm',
          'password': protocolConfig.password || ''
        }
      }

      default:
        throw new Error(`Unsupported protocol: ${protocol}`)
    }
  }

  /**
   * 构建路由配置
   * @param {string} mode - 分流模式
   * @param {Object} node - 选中的节点
   */
  buildRoute(mode, node) {
    if (mode === Mode.GLOBAL) {
      return this.buildGlobalRoute()
    } else {
      return this.buildBypassCNRoute()
    }
  }

  /**
   * 全局模式路由 (TUN)
   * 所有流量走代理，仅保留本地网络直连
   */
  buildGlobalRoute() {
    return {
      'rules': [
        // 本地网络直连
        {
          'type': 'field',
          'ip_cidr': ['0.0.0.0/8', '10.0.0.0/8', '127.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '224.0.0.0/4'],
          'outbound': 'direct'
        },
        // DNS 请求特殊处理
        {
          'type': 'field',
          'port': [53],
          'outbound': 'dns-out'
        },
        // BT 协议直连（可选）
        {
          'type': 'field',
          'protocol': ['bittorrent'],
          'outbound': 'direct'
        },
        // 默认全部代理
        {
          'type': 'default',
          'outbound': 'proxy'
        }
      ],
      'auto_detect_interface': true
    }
  }

  /**
   * 绕过大陆路由 (TUN)
   * 中国IP直连，海外流量走代理
   */
  buildBypassCNRoute() {
    return {
      'rules': [
        // 本地网络直连
        {
          'type': 'field',
          'ip_cidr': ['0.0.0.0/8', '10.0.0.0/8', '127.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '224.0.0.0/4'],
          'outbound': 'direct'
        },
        // DNS 请求
        {
          'type': 'field',
          'port': [53],
          'outbound': 'dns-out'
        },
        // 中国IP直连
        {
          'type': 'field',
          'geoip': ['cn', 'private'],
          'outbound': 'direct'
        },
        // BT 协议直连
        {
          'type': 'field',
          'protocol': ['bittorrent'],
          'outbound': 'direct'
        },
        // 默认走代理
        {
          'type': 'default',
          'outbound': 'proxy'
        }
      ],
      'auto_detect_interface': true
    }
  }

  /**
   * 获取当前模式
   */
  getCurrentMode() {
    return this.currentMode
  }

  /**
   * 获取支持的模式列表
   */
  getSupportedModes() {
    return Object.values(Mode)
  }
}

export { PolicyEngine, Mode, DNSPolicy }
