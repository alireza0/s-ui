import { Brutal } from "./brutal"

export interface iMultiplex{
  enabled: boolean
  padding?: boolean
  brutal?: Brutal
}

export interface oMultiplex extends iMultiplex{
  protocol?: "smux" | "yamux" | "h2mux"
  max_connections?: number
  min_streams?: number
  max_streams?: number
}