import { oTls } from "./outTls"

export const OutTypes = {
  Direct: 'direct',
  Block: 'block',
  SOCKS: 'socks',
  HTTP: 'http',
  Shadowsocks: 'shadowsocks',
  VMess: 'vmess',
  Trojan: 'trojan',
  Wireguard: 'wireguard',
  Hysteria: 'hysteria',
  VLESS: 'vless',
  ShadowTLS: 'shadowtls',
  TUIC: 'tuic',
  Hysteria2: 'hysteria2',
  Tur: 'tur',
  SSH: 'ssh',
  DNS: 'dns',
  Selector: 'selector',
  URLTest: 'urltest',
}

type OutType = typeof OutTypes[keyof typeof OutTypes]

export interface Dial {
  detour?: string
  bind_interface?: string
  inet4_bind_address?: string
  inet6_bind_address?: string
  routing_mark?: number
  reuse_addr?: boolean
  connect_timeout?: string
  tcp_fast_open?: boolean
  tcp_multi_path?: boolean
  udp_fragment?: boolean
  domain_strategy?: string
  fallback_delay?: string
}

interface OutboundBasics {
  type: OutType
  tag: string
}

export interface Direct extends OutboundBasics, Dial {
  override_address?: string
  override_port?: number
  proxy_protocol?: 0 | 1 | 2
}

// Create interfaces dynamically based on OutTypes keys
type InterfaceMap = {
  [Key in keyof typeof OutTypes]: {
    type: string
    [otherProperties: string]: any; // You can add other properties as needed
  }
}

// Create union type from InterfaceMap
export type Outbound = InterfaceMap[keyof InterfaceMap]

// Create defaultValues object dynamically
const defaultValues: Record<OutType, Outbound> = {
  direct: { type: OutTypes.Direct },
  block: { type: OutTypes.Block },
  socks: { type: OutTypes.SOCKS },
  http: { type: OutTypes.HTTP },
  shadowsocks: { type: OutTypes.Shadowsocks },
  vmess: { type: OutTypes.VMess, tls: { enabled: true } },
  trojan: { type: OutTypes.Trojan },
  wireguard: { type: OutTypes.Wireguard },
  hysteria: { type: OutTypes.Hysteria },
  vless: { type: OutTypes.VLESS },
  shadowtls: { type: OutTypes.ShadowTLS },
  tuic: { type: OutTypes.TUIC },
  hysteria2: { type: OutTypes.Hysteria2, users: [], tls: {} },
  tur: { type: OutTypes.Tur },
  ssh: { type: OutTypes.SSH },
  dns: { type: OutTypes.DNS },
  selector: { type: OutTypes.Selector },
  urltest: { type: OutTypes.URLTest },
}

export function createOutbound<T extends Outbound>(type: string,json?: Partial<T>): Outbound {
  const defaultObject: Outbound = { ...defaultValues[type], ...(json || {}) }
  return defaultObject
}