export interface logicalRule {
  type: 'logical' | 'simple'
  mode: 'and' | 'or'
  rules: rule[]
  invert: boolean
  outbound: string
}

export interface rule {
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
  package_name?: string[]
  user?: string[]
  user_id?: number[]
  clash_mode?: string
  rule_set?: string[]
  rule_set_ipcidr_match_source?: boolean
  invert?: boolean
  outbound?: string
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