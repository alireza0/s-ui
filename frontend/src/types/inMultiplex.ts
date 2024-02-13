interface Brutal {
  enabled: boolean
  up_mbps: number
  down_mbps: number
}

export interface iMultiplex{
  enabled: boolean
  padding?: boolean
  brutal?: Brutal
}