export interface iTls {
  enabled?: boolean
  server_name?: string
  alpn?: string[]
  min_version?: string
  max_version?: string
  cipher_suites?: string[]
  certificate?: string[]
  certificate_path?: string
  key?: string[]
  key_path?: string
}

export const defaultInTls: iTls = {
  alpn: ['HTTP/3', 'HTTP/2', 'HTTP/1.1'],
  min_version: "1.2",
  max_version: "1.3",
  cipher_suites: [""],
}