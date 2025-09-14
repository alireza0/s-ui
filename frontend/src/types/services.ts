import { Listen } from "./inbounds"
import { iTls } from "./tls"

export const SrvTypes = {
  DERP: 'derp',
  Resolved: 'resolved',
  SSMAPI: 'ssm-api',
}

type SrvType = typeof SrvTypes[keyof typeof SrvTypes]

interface SrvBasics extends Listen {
  id: number
  type: SrvType
  tag: string
  tls_id: number
}

export interface DERP extends SrvBasics {
  tls: iTls
  config_path: string
  verify_client_endpoint?: string[]
  verify_client_url?: any[]
  home?: string
  mesh_with?: any[]
  mesh_psk?: string
  mesh_psk_file?: string
  stun?: any
}

export interface Resolved extends SrvBasics {}

export interface SSMAPI extends SrvBasics {
  servers: any
  tls?: iTls
}

type InterfaceMap = {
  derp: DERP
  resolved: Resolved
  'ssm-api': SSMAPI
}

export type Srv = InterfaceMap[keyof InterfaceMap]

const defaultValues: Record<SrvType, Srv> = {
  derp: <DERP>{ type: 'derp', config_path: '', tls_id:0 },
  resolved: <Resolved>{ type: 'resolved', listen: '::', listen_port: 53 },
  'ssm-api': <SSMAPI>{ type: 'ssm-api', tls_id: 0, servers: {} },
}

export function createSrv<T extends Srv>(type: string, json?: Partial<T>): Srv {
  const defaultObject: Srv = { ...defaultValues[type], ...(json || {}) }
  return defaultObject
}