export const TrspTypes = {
  HTTP: 'http',
  WebSocket: 'ws',
  QUIC: 'quic',
  gRPC: 'grpc',
  HTTPUpgrade: "httpupgrade"
}

export type TrspType = typeof TrspTypes[keyof typeof TrspTypes]

export type Transport = HTTP|WebSocket|QUIC|gRPC|HTTPUpgrade

interface TransportBasics {
  type: TrspType
}

export interface HTTP extends TransportBasics {
  host?: string[]
  path?: string
  method?: string
  headers?: {}
  idle_timeout?: string
  ping_timeout?: string
}

export interface WebSocket extends TransportBasics {
  path: string
  headers?: {
    Host: string
  }
  max_early_data?: number
  early_data_header_name?: string
}

export interface QUIC extends TransportBasics {}

export interface gRPC extends TransportBasics {
  service_name?: string
  idle_timeout?: string
  ping_timeout?: string
  permit_without_stream?: boolean
}

export interface HTTPUpgrade extends TransportBasics {
  host?: string
  path?: string
  headers?: {}
}
