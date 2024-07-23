import { Hysteria, Hysteria2, InTypes, Inbound, Naive, Shadowsocks, TUIC, Trojan, VLESS, VMess } from "@/types/inbounds"
import { HTTP, WebSocket, gRPC, HTTPUpgrade, Transport, TrspTypes } from "@/types/transport"
import RandomUtil from "./randomUtil"
import { Client } from "@/types/clients"

export interface Link {
  type: "local" | "external" | "sub"
  remark?: string
  uri: string
}

function utf8ToBase64(utf8String: string): string {
  const encodedUtf8 = encodeURIComponent(utf8String).replace(/%([0-9A-F]{2})/g, (_, p1) => String.fromCharCode(parseInt(p1, 16)))
  return btoa(encodedUtf8)
}

export namespace LinkUtil {
  export function linkGenerator(user: Client, inbound: Inbound, tlsClient: any = {}, addrs: any[] = []): string[] {
    switch(inbound.type){
      case InTypes.Shadowsocks:
        return shadowsocksLink(user,<Shadowsocks>inbound, addrs)
      case InTypes.Naive:
        return naiveLink(user,<Naive>inbound, addrs, tlsClient)
      case InTypes.Hysteria:
        return hysteriaLink(user,<Hysteria>inbound, addrs, tlsClient)
      case InTypes.Hysteria2:
        return hysteria2Link(user,<Hysteria2>inbound, addrs, tlsClient)
      case InTypes.TUIC:
        return tuicLink(user,<TUIC>inbound, addrs, tlsClient)
      case InTypes.VLESS:
        return vlessLink(user,<VLESS>inbound, addrs, tlsClient)
      case InTypes.Trojan:
        return trojanLink(user,<Trojan>inbound, addrs, tlsClient)
      case InTypes.VMess:
        return vmessLink(user,<VMess>inbound, addrs, tlsClient)
    }
    return []
  }

  function shadowsocksLink(user: Client, inbound: Shadowsocks, addrs: any[]): string[] {
    const userPass = user.config.shadowsocks?.password
    const password = [userPass]
    if (inbound.method.startsWith('2022')) password.push(inbound.password)
    const params = {
      tfo: inbound.tcp_fast_open? 1 : null,
      network: inbound.network?? null
    }

    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`ss://${utf8ToBase64(inbound.method + ':' + password.join(':'))}@${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries(params)){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`ss://${utf8ToBase64(inbound.method + ':' + password.join(':'))}@${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries(params)){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
  }

  function hysteriaLink(user: Client, inbound: Hysteria, addrs: any[], tlsClient: any): string[] {
    const auth = user.config.hysteria.auth_str
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

    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`hysteria://${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries(params)){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`hysteria://${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries(params)){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        if (a.server_name?.length>0) {
          uri.searchParams.set('peer', a.server_name)
        } else {
          inbound.tls.server_name ? uri.searchParams.set('peer', inbound.tls.server_name) : uri.searchParams.delete('peer')
        }
        if (a.insecure) {
          uri.searchParams.set('insecure', '1')
        } else {
          tlsClient.insecure ? uri.searchParams.set('insecure', '1') : uri.searchParams.delete('insecure')
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
  }

  function hysteria2Link(user: Client, inbound: Hysteria2, addrs: any[], tlsClient: any): string[] {
    const password = user.config.hysteria2.password
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

    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`hysteria2://${password}@${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries(params)){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`hysteria2://${password}@${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries(params)){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        if (a.server_name?.length>0) {
          uri.searchParams.set('sni', a.server_name)
        } else {
          inbound.tls.server_name ? uri.searchParams.set('sni', inbound.tls.server_name) : uri.searchParams.delete('sni')
        }
        if (a.insecure) {
          uri.searchParams.set('insecure', '1')
        } else {
          tlsClient.insecure ? uri.searchParams.set('insecure', '1') : uri.searchParams.delete('insecure')
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
  }

  function naiveLink(user: Client, inbound: Naive, addrs: any[], tlsClient: any): string[] {
    const password = user.config.naive.password

    let links = <string[]>[]
    if (addrs.length == 0) {
      const params = {
        padding: 1,
        peer: inbound.tls.server_name?? null,
        alpn: inbound.tls.alpn?.join(',')?? null,
        tfo: inbound.tcp_fast_open? 1 : 0,
        allowInsecure: tlsClient?.insecure ? 1 : null
      }
      const uri = `http2://${utf8ToBase64(user + ":" + password + "@" + location.hostname + ":" + inbound.listen_port)}`
      const paramsArray = []
      for (const [key, value] of Object.entries(params)){
        if (value) {
          paramsArray.push(`${key}=${encodeURIComponent(value.toString())}`)
        }
      }
      links.push(uri.toString() + "?" + paramsArray.join('&') + "#" + inbound.tag)
    } else {
      addrs.forEach(a => {
        const params = {
          padding: 1,
          peer: a.server_name?.length>0 ? a.server_name : inbound.tls.server_name?? null,
          alpn: inbound.tls.alpn?.join(',')?? null,
          tfo: inbound.tcp_fast_open? 1 : 0,
          allowInsecure: a.insecure ? 1 : tlsClient?.insecure ? 1 : null
        }
        const uri = `http2://${utf8ToBase64(user + ":" + password + "@" + a.server + ":" + a.server_port)}`
        const paramsArray = []
        for (const [key, value] of Object.entries(params)){
          if (value) {
            paramsArray.push(`${key}=${encodeURIComponent(value.toString())}`)
          }
        }
        links.push(uri.toString() + "?" + paramsArray.join('&') + "#" + encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag))
      })
    }
    return links
  }

  function tuicLink(user: Client, inbound: TUIC, addrs: any[], tlsClient: any): string[] {
    const u = user.config.tuic
    const params = {
      sni: inbound.tls.server_name?? null,
      alpn: inbound.tls.alpn?.join(',')?? null,
      congestion_control: inbound.congestion_control?? null,
      allowInsecure: tlsClient?.insecure ? 1 : null,
      disable_sni: tlsClient?.disable_sni ? 1 : null
    }

    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`tuic://${u?.uuid}:${u?.password}@${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries(params)){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`tuic://${u?.uuid}:${u?.password}@${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries(params)){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        if (a.server_name?.length>0) {
          uri.searchParams.set('sni', a.server_name)
        } else {
          inbound.tls.server_name ? uri.searchParams.set('sni', inbound.tls.server_name) : uri.searchParams.delete('sni')
        }
        if (a.insecure) {
          uri.searchParams.set('allowInsecure', '1')
        } else {
          tlsClient.insecure ? uri.searchParams.set('allowInsecure', '1') : uri.searchParams.delete('allowInsecure')
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
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

  function vlessLink(user: Client, inbound: VLESS, addrs: any[], tlsClient: any): string[] {
    const u = user.config.vless
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
    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`vless://${u?.uuid}@${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries({...params, ...tParams})){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`vless://${u?.uuid}@${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries({...params, ...tParams})){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        if (a.tls != undefined){
          if (a.tls) {
            uri.searchParams.set('security','tls')
          } else {
            uri.searchParams.delete('security')
            uri.searchParams.delete('sni')
            uri.searchParams.delete('alpn')
            uri.searchParams.delete('allowInsecure')
          }
        }
        if (a.server_name?.length>0) {
          uri.searchParams.set('sni', a.server_name)
        } else {
          inbound.tls?.server_name ? uri.searchParams.set('sni', inbound.tls.server_name) : uri.searchParams.delete('sni')
        }
        if (a.insecure) {
          uri.searchParams.set('allowInsecure', '1')
        } else {
          tlsClient.insecure ? uri.searchParams.set('allowInsecure', '1') : uri.searchParams.delete('allowInsecure')
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
  }

  function trojanLink(user: Client, inbound: Trojan, addrs: any[], tlsClient: any): string[] {
    const u = user.config.trojan
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

    let links = <string[]>[]
    if (addrs.length == 0) {
      const uri = new URL(`trojan://${u?.password}@${location.hostname}:${inbound.listen_port}`)
      for (const [key, value] of Object.entries({...params, ...tParams})){
        if (value) {
          uri.searchParams.set(key, value.toString())
        }
      }
      uri.hash = encodeURIComponent(inbound.tag)
      links.push(uri.toString())
    } else {
      addrs.forEach(a => {
        const uri = new URL(`trojan://${u?.password}@${a.server}:${a.server_port}`)
        for (const [key, value] of Object.entries({...params, ...tParams})){
          if (value) {
            uri.searchParams.set(key, value.toString())
          }
        }
        if (a.tls != undefined){
          if (a.tls) {
            uri.searchParams.set('security','tls')
          } else {
            uri.searchParams.delete('security')
            uri.searchParams.delete('sni')
            uri.searchParams.delete('alpn')
            uri.searchParams.delete('allowInsecure')
          }
        }
        if (a.server_name?.length>0) {
          uri.searchParams.set('sni', a.server_name)
        } else {
          inbound.tls?.server_name ? uri.searchParams.set('sni', inbound.tls.server_name) : uri.searchParams.delete('sni')
        }
        if (a.insecure) {
          uri.searchParams.set('allowInsecure', '1')
        } else {
          tlsClient.insecure ? uri.searchParams.set('allowInsecure', '1') : uri.searchParams.delete('allowInsecure')
        }
        uri.hash = encodeURIComponent(a.remark ? inbound.tag + a.remark : inbound.tag)
        links.push(uri.toString())
      })
    }
    return links
  }

  function vmessLink(user: Client, inbound: VMess, addrs: any[], tlsClient: any): string[] {
    const u = user.config.vmess
    const transport = <Transport>inbound.transport

    const tParams = getTransportParams(transport)
    if (transport.type == TrspTypes.gRPC) tParams.path = tParams.serviceName

    const params = {
      v: 2,
      add: location.hostname,
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
    let links = <string[]>[]
    if (addrs.length == 0) {
      links.push('vmess://' + utf8ToBase64(JSON.stringify(params, null, 2)))
    } else {
      addrs.forEach(a => {
        let newParams = {...params}
        newParams.add = a.server
        newParams.port = a.server_port
        if (a.tls != undefined){
          if (a.tls) {
            newParams.tls = 'tls'
          } else {
            newParams.tls = 'none'
            delete newParams.sni
            delete newParams.allowInsecure
          }
        }
        if (a.server_name?.length>0) {
          newParams.sni = a.server_name
        }
        if (a.insecure) {
          newParams.allowInsecure = 1
        }
        newParams.ps = inbound.tag + (a.remark??'')
        links.push('vmess://' + utf8ToBase64(JSON.stringify(newParams, null, 2)))
      })
    }
    return links
  }
}