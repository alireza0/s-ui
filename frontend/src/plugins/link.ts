import { Hysteria, Hysteria2, InTypes, Inbound, Naive, Shadowsocks, TUIC, Trojan, VLESS, VMess } from "@/types/inbounds"
import { HTTP, WebSocket, gRPC, HTTPUpgrade, Transport, TrspTypes } from "@/types/transport"
import RandomUtil from "./randomUtil"

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
  export function linkGenerator(user: string, inbound: Inbound, tlsClient: any = null): string {
    const addr = location.hostname
    switch(inbound.type){
      case InTypes.Shadowsocks:
        return shadowsocksLink(user,<Shadowsocks>inbound, addr)
      case InTypes.Naive:
        return naiveLink(user,<Naive>inbound, addr, tlsClient)
      case InTypes.Hysteria:
        return hysteriaLink(user,<Hysteria>inbound, addr, tlsClient)
      case InTypes.Hysteria2:
        return hysteria2Link(user,<Hysteria2>inbound, addr, tlsClient)
      case InTypes.TUIC:
        return tuicLink(user,<TUIC>inbound, addr, tlsClient)
      case InTypes.VLESS:
        return vlessLink(user,<VLESS>inbound, addr, tlsClient)
      case InTypes.Trojan:
        return trojanLink(user,<Trojan>inbound, addr, tlsClient)
      case InTypes.VMess:
        return vmessLink(user,<VMess>inbound, addr, tlsClient)
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

  function hysteriaLink(user: string, inbound: Hysteria, addr: string, tlsClient: any): string {
    const auth = inbound.users.find(i => i.name == user)?.auth_str
    const params = {
      upmbps: inbound.up_mbps?? null,
      downmbps: inbound.down_mbps?? null,
      auth: auth?? null,
      peer: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      obfsParam: inbound.obfs?? null,
      fastopen: inbound.tcp_fast_open? 1 : 0,
      insecure: tlsClient?.insecure ? 1 : null
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

  function hysteria2Link(user: string, inbound: Hysteria2, addr: string, tlsClient: any): string {
    const password = inbound.users.find(i => i.name == user)?.password
    const params = {
      upmbps: inbound.up_mbps?? null,
      downmbps: inbound.down_mbps?? null,
      sni: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      obfs: inbound.obfs?.type?? null,
      'obfs-password': inbound.obfs?.password?? null,
      fastopen: inbound.tcp_fast_open? 1 : 0,
      insecure: tlsClient?.insecure ? 1 : null
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

  function naiveLink(user: string, inbound: Naive, addr: string, tlsClient: any): string {
    const password = inbound.users.find(i => i.username == user)?.password
    const params = {
      padding: 1,
      peer: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      tfo: inbound.tcp_fast_open? 1 : 0,
      allowInsecure: tlsClient?.insecure ? 1 : null
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

  function tuicLink(user: string, inbound: TUIC, addr: string, tlsClient: any): string {
    const u = inbound.users.find(i => i.name == user)
    const params = {
      sni: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      congestion_control: inbound.congestion_control?? null,
      allowInsecure: tlsClient?.insecure ? 1 : null,
      disable_sni: tlsClient?.disable_sni ? 1 : null
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

  function vlessLink(user: string, inbound: VLESS, addr: string, tlsClient: any): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)

    const params = {
      type: transport?.type?? 'tcp',
      security: inbound.tls?.enabled? inbound.tls?.reality?.enabled ? 'reality' : 'tls' : null,
      alpn: inbound.tls?.alpn?.join(',')?? null,
      sni: inbound.tls?.server_name?? null,
      flow: inbound.tls?.enabled ? u?.flow?? null : null,
      allowInsecure: tlsClient?.insecure ? 1 : null,
      fp: tlsClient?.utls?.enabled ? tlsClient.utls.fingerprint : null,
      pbk: tlsClient?.reality?.public_key?? null,
      sid: inbound.tls?.reality?.enabled ? (inbound.tls?.reality?.short_id?.length>0 ?  inbound.tls.reality.short_id[RandomUtil.randomInt(inbound.tls.reality.short_id.length)] : null) : null
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

  function trojanLink(user: string, inbound: Trojan, addr: string, tlsClient: any): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)

    const params = {
      type: transport?.type?? 'tcp',
      security: inbound.tls?.enabled? inbound.tls?.reality?.enabled ? 'reality' : 'tls' : null,
      alpn: inbound.tls?.alpn?.join(',')?? null,
      sni: inbound.tls?.server_name?? null,
      allowInsecure: tlsClient?.insecure ? 1 : null,
      fp: tlsClient?.utls?.enabled ? tlsClient.utls.fingerprint : null,
      pbk: tlsClient?.reality?.public_key?? null,
      sid: inbound.tls?.reality?.enabled ? (inbound.tls?.reality?.short_id?.length>0 ?  inbound.tls.reality.short_id[RandomUtil.randomInt(inbound.tls.reality.short_id.length)] : null) : null
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

  function vmessLink(user: string, inbound: VMess, addr: string, tlsClient: any): string {
    const u = inbound.users.find(i => i.name == user)
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)
    if (transport.type == TrspTypes.gRPC) tParams.path = tParams.serviceName

    const params = {
      v: 2,
      add: addr,
      aid: u?.alterId,
      host:	tParams.host?? undefined,
      id: u?.uuid,
      net: transport?.type == undefined || transport?.type == 'http' ? 'tcp' : transport.type,
      type: transport?.type == 'http' ? 'http' : undefined,
      path:	tParams.path?? undefined,
      port:	inbound.listen_port,
      ps:	inbound.tag,
      sni: inbound.tls.server_name?? undefined,
      tls: Object.keys(inbound.tls).length>0? 'tls' : 'none',
      allowInsecure: tlsClient?.insecure ? 1 : undefined
    }
    return 'vmess://' + utf8ToBase64(JSON.stringify(params, null, 2))
  }
}