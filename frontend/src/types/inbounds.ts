import { iMultiplex } from "./multiplex"
import { iTls } from "./inTls"
import { Dial } from "./outbounds"
import { Transport } from "./transport"

export const InTypes = {
  Direct: 'direct',
  Mixed: 'mixed',
  SOCKS: 'socks',
  HTTP: 'http',
  Shadowsocks: 'shadowsocks',
  VMess: 'vmess',
  Trojan: 'trojan',
  Naive: 'naive',
  Hysteria: 'hysteria',
  ShadowTLS: 'shadowtls',
  TUIC: 'tuic',
  Hysteria2: 'hysteria2',
  VLESS: 'vless',
  // Tun: 'tun',
  Redirect: 'redirect',
  TProxy: 'tproxy',
}

type InType = typeof InTypes[keyof typeof InTypes]

export interface Listen {
  listen: string
  listen_port: number
  tcp_fast_open?: boolean
  tcp_multi_path?: boolean
  udp_fragment?: boolean
  udp_timeout?: string
  detour?: string
  sniff?: boolean
  sniff_override_destination?: boolean
  sniff_timeout?: string
  domain_strategy?: string
}

interface InboundBasics extends Listen {
  type: InType
  tag: string
}

interface UsernamePass {
  username: string
  password: string
}
interface NamePass {
  name: string
  password: string
}
interface NameUUID {
  name: string
  uuid: string
}
interface NameAuth {
  name: string
  auth_str: string
}
interface VmessUser extends NameUUID {
  alterId: number
}
interface VlessUser extends NameUUID {
  flow: string
}
interface TuicUser extends NameUUID {
  password?: string
}

interface ShadowTLSHandShake extends Dial {
  server: string
  server_port: number
}

export interface Direct extends InboundBasics {
  network?: "udp" | "tcp"
  override_address?: string
  override_port?: number
}
export interface Mixed extends InboundBasics {
  users?: UsernamePass[]
}
export interface SOCKS extends InboundBasics {
  users?: UsernamePass[]
}
export interface HTTP extends InboundBasics {
  users?: UsernamePass[]
  tls?: iTls,
}
export interface Shadowsocks extends InboundBasics {
  method: string
  password: string
  network?: "udp" | "tcp"
  users?: NamePass[]
  multiplex?: iMultiplex
}
export interface VMess extends InboundBasics {
  users: VmessUser[]
  tls: iTls
  multiplex?: iMultiplex
  transport?: Transport
}
export interface Trojan extends InboundBasics {
  users: NamePass[]
  tls: iTls
  fallback?: {
    server: string
    server_port: number
  }
  multiplex?: iMultiplex
  transport?: Transport
}
export interface Naive extends InboundBasics {
  users: UsernamePass[]
  tls: iTls,
}
export interface Hysteria extends InboundBasics {
  up_mbps: number
  down_mbps: number
  obfs?: string
  users: NameAuth[]
  recv_window_conn?: number
  recv_window_client?: number
  max_conn_client?: number
  disable_mtu_discovery?: boolean
  tls: iTls
}
export interface ShadowTLS extends InboundBasics {
  version: 1|2|3
  password?: string
  users?: NamePass[]
  handshake: ShadowTLSHandShake
  handshake_for_server_name?: {
    [server_name: string]: ShadowTLSHandShake
  }
  strict_mode?: boolean
}
export interface VLESS extends InboundBasics {
  users: VlessUser[]
  tls?: iTls
  multiplex?: iMultiplex
  transport?: Transport
}
export interface TUIC extends InboundBasics {
  users: TuicUser[]
  congestion_control: ""|"cubic"|"new_reno"|"bbr"
  auth_timeout?: string
  zero_rtt_handshake?: boolean
  heartbeat?: string
  tls: iTls
}
export interface Hysteria2 extends InboundBasics {
  up_mbps?: number
  down_mbps?: number
  obfs?: {
    type?: "salamander"
    password: string
  }
  users: NamePass[]
  ignore_client_bandwidth?: boolean
  tls: iTls
  masquerade?: string
  brutal_debug?: boolean
}
export interface Tun extends InboundBasics {
  [otherProperties: string]: any
}
export interface Redirect extends InboundBasics {}
export interface TProxy extends InboundBasics {
  network?: "udp" | "tcp"
}

// Create interfaces dynamically based on InTypes keys
type InterfaceMap = {
  direct: Direct
  mixed: Mixed
  socks: SOCKS
  http: SOCKS
  shadowsocks: Shadowsocks
  vmess: VMess
  trojan: Trojan
  naive: Naive
  hysteria: Hysteria
  shadowtls: ShadowTLS
  tuic: TUIC
  hysteria2: Hysteria2
  vless: VLESS
  // tun: Tun
  redirect: Redirect
  tproxy: TProxy
}

// Create union type from InterfaceMap
export type Inbound = InterfaceMap[keyof InterfaceMap]

type userEnabledTypes = {
  mixed: Mixed
  socks: SOCKS
  http: SOCKS
  shadowsocks: Shadowsocks
  vmess: VMess
  trojan: Trojan
  naive: Naive
  hysteria: Hysteria
  shadowtls: ShadowTLS
  tuic: TUIC
  hysteria2: Hysteria2
  vless: VLESS
}

// Create union type from userEnabledTypes
export type InboundWithUser = userEnabledTypes[keyof userEnabledTypes]

// Create defaultValues object dynamically
const defaultValues: Record<InType, Inbound> = {
  direct: <Direct>{ type: InTypes.Direct },
  mixed: <Mixed>{ type: InTypes.Mixed },
  socks: <SOCKS>{ type: InTypes.SOCKS },
  http: <HTTP>{ type: InTypes.HTTP, tls: {} },
  shadowsocks: <Shadowsocks>{ type: InTypes.Shadowsocks, method: 'none', multiplex: {} },
  vmess: <VMess>{ type: InTypes.VMess, users: <VmessUser[]>[], tls: {}, multiplex: {}, transport: {} },
  trojan: <Trojan>{ type: InTypes.Trojan, users: <NamePass[]>[], tls: {}, multiplex: {}, transport: {} },
  naive: <Naive>{ type: InTypes.Naive, users: <UsernamePass[]>[], tls: { enabled: true } },
  hysteria: <Hysteria>{ type: InTypes.Hysteria, users: <NameAuth[]>[], up_mbps: 100, down_mbps: 100, tls: { enabled: true } },
  shadowtls: <ShadowTLS>{ type: InTypes.ShadowTLS, version: 3, users: <NamePass[]>[], handshake: {}, handshake_for_server_name: {} },
  tuic: <TUIC>{ type: InTypes.TUIC, users: <TuicUser[]>[], congestion_control: "cubic", tls: { enabled: true } },
  hysteria2: <Hysteria2>{ type: InTypes.Hysteria2, users: <NamePass[]>[], tls: { enabled: true } },
  vless: <VLESS>{ type: InTypes.VLESS, users: <VlessUser[]>[], tls: {}, multiplex: {}, transport: {} },
  // tun: <Tun>{ type: InTypes.Tun },
  redirect: <Redirect>{ type: InTypes.Redirect },
  tproxy: <TProxy>{ type: InTypes.TProxy },
}

export function createInbound<T extends Inbound>(type: InType,json?: Partial<T>): Inbound {
  const defaultObject: Inbound = { ...defaultValues[type] ?? {}, ...(json ?? {}) }
  return defaultObject
}