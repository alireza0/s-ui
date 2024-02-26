import { Hysteria, Hysteria2, InTypes, Inbound, Naive, Shadowsocks, TUIC, Trojan, VLESS, VMess } from "@/types/inbounds"
import { HTTP, WebSocket, QUIC, gRPC, HTTPUpgrade, Transport, TrspTypes } from "@/types/transport";

export interface Link {
  type: "local" | "external" | "sub"
  remark?: string
  uri: string
}

function utf8ToBase64(utf8String: string): string {
  const encodedUtf8 = encodeURIComponent(utf8String).replace(/%([0-9A-F]{2})/g, (_, p1) => String.fromCharCode(parseInt(p1, 16)));
  return btoa(encodedUtf8);
}

export namespace LinkUtil {
  export function linkGenerator(user: string, inbound: Inbound): string {
    const addr = location.hostname
    switch(inbound.type){
      case InTypes.Shadowsocks:
        return shadowsocksLink(user,<Shadowsocks>inbound,addr)
      case InTypes.Naive:
        return naiveLink(user,<Naive>inbound,addr)
      case InTypes.Hysteria:
        return hysteriaLink(user,<Hysteria>inbound,addr)
      case InTypes.Hysteria2:
        return hysteria2Link(user,<Hysteria2>inbound,addr)
      case InTypes.TUIC:
        return tuicLink(user,<TUIC>inbound,addr)
      case InTypes.VLESS:
        return vlessLink(user,<VLESS>inbound,addr)
      case InTypes.Trojan:
        return trojanLink(user,<Trojan>inbound,addr)
      case InTypes.VMess:
        return vmessLink(user,<VMess>inbound,addr)
    }
    return ''
  }

  function shadowsocksLink(user: string, inbound: Shadowsocks, addr: string): string {
    const userPass = inbound.users?.find(i => i.name == user)?.password
    const password = [userPass]
    if (inbound.method.startsWith('2022')) password.push(inbound.password)

    const params = {
      tfo: inbound.tcp_fast_open? 1 : null,
      network: inbound.network?? null
    } 

    const uri = new URL(`ss://${utf8ToBase64(inbound.method + ':' + password.join(':'))}@${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries(params)){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function hysteriaLink(user: string, inbound: Hysteria, addr: string): string {
    const auth = inbound.users.find(i => i.name == user)?.auth_str
    const params = {
      upmbps: inbound.up_mbps?? null,
      downmbps: inbound.down_mbps?? null,
      auth: auth?? null,
      peer: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      obfsParam: inbound.obfs?? null,
      fastopen: inbound.tcp_fast_open? 1 : 0
    }
    const uri = new URL(`hysteria://${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries(params)){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function hysteria2Link(user: string, inbound: Hysteria2, addr: string): string {
    const password = inbound.users.find(i => i.name == user)?.password
    const params = {
      upmbps: inbound.up_mbps?? null,
      downmbps: inbound.down_mbps?? null,
      sni: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      obfs: inbound.obfs?.type?? null,
      'obfs-password': inbound.obfs?.password?? null,
      fastopen: inbound.tcp_fast_open? 1 : 0
    }
    const uri = new URL(`hysteria2://${password}@${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries(params)){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function naiveLink(user: string, inbound: Naive, addr: string): string {
    const password = inbound.users.find(i => i.username == user)?.password
    const params = {
      padding: 1,
      peer: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      tfo: inbound.tcp_fast_open? 1 : 0
    }
    const uri = `http2://${utf8ToBase64(user + ":" + password + "@" + addr + ":" + inbound.listen_port)}`
    const paramsArray = []
    for (const [key, value] of Object.entries(params)){
      if (value) {
        paramsArray.push(`${key}=${encodeURIComponent(value.toString())}`)
      }
    }
    return uri.toString() + "?" + paramsArray.join('&') + "#" + inbound.tag
  }

  function tuicLink(user: string, inbound: TUIC, addr: string): string {
    const u = inbound.users.find(i => i.name == user)
    const params = {
      sni: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      congestion_control: inbound.congestion_control?? null
    }
    const uri = new URL(`tuic://${u?.uuid}:${u?.password}@${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries(params)){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function getTransportParams(t:Transport): any {
    if (Object.keys(t).length == 0) return {}

    const params = {
      host: <string|null>'',
      path: <string|null>'',
      serviceName: <string|null>'',
    }
    switch (t.type){
      case TrspTypes.HTTP:
        const th = <HTTP>t
        params.host = th.host?.join(',')?? null
        params.path = th.path?? null
        break
      case TrspTypes.WebSocket:
        const tw = <WebSocket>t
        params.path = tw.path?? null
        params.host = tw.headers?.Host?? null
        break
      case TrspTypes.gRPC:
        const tg = <gRPC>t
        params.serviceName = tg.service_name?? null
        break
      case TrspTypes.HTTPUpgrade:
        const tu = <HTTPUpgrade>t
        params.host = tu.host?? null
        params.path = tu.path?? null
        break
    }

    return params
  }

  function vlessLink(user: string, inbound: VLESS, addr: string): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)

    const params = {
      type: transport?.type?? 'none',
      security: inbound.tls?.enabled? 'tls' : null,
      alpn: inbound.tls?.alpn?.join(',')?? null,
      sni: inbound.tls?.server_name?? null,
      flow: inbound.tls?.enabled ? u?.flow?? null : null
    }
    const uri = new URL(`vless://${u?.uuid}@${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries({...params, ...tParams})){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function trojanLink(user: string, inbound: Trojan, addr: string): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)

    const params = {
      type: transport?.type?? 'none',
      security: inbound.tls?.enabled? 'tls' : null,
      alpn: inbound.tls?.alpn?.join(',')?? null,
      sni: inbound.tls?.server_name?? null,
    }
    const uri = new URL(`trojan://${u?.password}@${addr}:${inbound.listen_port}`)
    for (const [key, value] of Object.entries({...params, ...tParams})){
      if (value) {
        uri.searchParams.set(key, value.toString())
      }
    }
    uri.hash = encodeURIComponent(inbound.tag)
    return uri.toString()
  }

  function vmessLink(user: string, inbound: VMess, addr: string): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)
    if (transport.type == TrspTypes.gRPC) tParams.path = tParams.serviceName

    const params = {
      v: 2,
      add: addr,
      aid: u?.alterId,
      host:	tParams.host,
      id: u?.uuid,
      net:	transport.type,
      path:	tParams.path,
      port:	inbound.listen_port,
      ps:	inbound.tag,
      sni: inbound.tls.server_name?? '',
      tls: Object.keys(inbound.tls).length>0? 'tls' : 'none'
    }
    return 'vmess://' + utf8ToBase64(JSON.stringify(params))
  }
}