export interface Addr {
  server: string
  server_port: number
  tls?: boolean
  insecure?: boolean
  server_name?: string
  remark?: string
}

export interface InData {
  id: number
  tag: string
  addrs: Addr[]
  outJson: any
}
