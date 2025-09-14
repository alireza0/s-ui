import { Inbound } from './inbounds'
import { Outbound } from './outbounds'
import { Dns } from './dns'
import { Dial } from './dial'

interface Log {
  disabled?: boolean
  level?: string
  output?: string
  timestamp?: boolean
}

export interface Ntp extends Dial{
  enabled?: boolean
  server: string
  server_port?: number
  interval?: string
}

interface Route {
  rules: RouteRule[] | RouteRuleLogical[]
  rule_set: RouteRuleSet[]
  final?: string,
  auto_detect_interface?: boolean
  default_interface?: string
  default_mark?: number
  default_domain_resolver: string
}

interface RouteRule       {
  inbound?: string[] | string
  ip_version?: 4 | 6,
  network?: "tcp" | "udp"
  auth_user?: string[]
  protocol?: string[] | string
  domain?: string[] | string
  domain_suffix?: string[] | string
  domain_keyword?: string[] | string
  domain_regex?: string[] | string
  source_ip_cidr?: string[] | string
  source_ip_is_private?: boolean
  ip_cidr?: string[] | string
  ip_is_private?: boolean
  source_port?: number[] | number
  source_port_range?: string[] | string
  port?: number[] | number
  port_range?: string[] | string
  clash_mode?: string
  rule_set?: string[] | string
  invert?: boolean
  outbound: string
}

interface RouteRuleLogical {
  type: "logical"
  mode: "and" | "or"
  rules: RouteRule[]
  invert?: boolean
  outbound: string
}

interface RouteRuleSet {
  type: string
  tag: string
  format: string
  path?: string
  url?: string
  download_detour?: string
  update_interval?: string
}

interface Experimental {
  cache_file?: CacheFile
  clash_api?: ClashApi
  v2ray_api?: V2rayApi
}

interface CacheFile {
  enabled?: boolean
  path?: string
  cache_id?: string
  store_fakeip?: boolean
}

interface V2rayApi {
  listen: string
  stats: V2rayApiStats
}

export interface V2rayApiStats {
  enabled: boolean
  inbounds: string[]
  outbounds: string[]
  users: string[]
}

interface ClashApi {
  external_controller?: string
  external_ui?: string
  external_ui_download_url?: string
  external_ui_download_detour?: string
  secret?: string
  default_mode?: string
  access_control_allow_origin?: string[]
  access_control_allow_private_network?: boolean
}

export interface Config {
  log: Log
  dns: Dns
  ntp?: Ntp
  inbounds: Inbound[]
  outbounds: Outbound[]
  route: Route
  experimental: Experimental
}