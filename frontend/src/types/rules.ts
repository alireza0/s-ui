interface generalRule {
  invert: boolean
  action: 'route' | 'route-options' | 'reject' | 'hijack-dns' | 'sniff' | 'resolve'
  outbound?: string
  override_address?: string
  override_port?: number
  udp_disable_domain_unmapping?: boolean
  udp_connect?: boolean
  udp_timeout?: string
  method?: string
  no_drop?: boolean
  sniffer: string[]
  timeout: string
  strategy: string
  server: string
}

export const actionKeys = [
  'invert',
  'action',
  'outbound',
  'override_address',
  'override_port',
  'udp_disable_domain_unmapping',
  'udp_connect',
  'udp_timeout',
  'method',
  'no_drop',
  'sniffer',
  'timeout',
  'strategy',
  'server'
]
export interface logicalRule extends generalRule {
  type: 'logical' | 'simple'
  mode: 'and' | 'or'
  rules: rule[]
}

export interface rule extends generalRule {
  inbound?: string[]
  ip_version?: 4 | 6
  network?: string[]
  auth_user?: string[]
  protocol?: string[]
  domain?: string[]
  domain_suffix?: string[]
  domain_keyword?: string[]
  domain_regex?: string[]
  source_ip_cidr?: string[]
  source_ip_is_private?: boolean
  ip_cidr?: string[]
  ip_is_private?: boolean
  source_port?: number[]
  source_port_range?: string[]
  port?: number[]
  port_range?: string[]
  process_name?: string[]
  process_path?: string[]
  process_path_regex?: string[]
  package_name?: string[]
  user?: string[]
  user_id?: number[]
  clash_mode?: string
  rule_set?: string[]
  rule_set_ip_cidr_match_source?: boolean
}

export interface ruleset {
  type: 'local' | 'remote'
  tag: string
  format: 'source' | 'binary'
  path?: string
  url?: string
  download_detour?: string
  update_interval?: string
}