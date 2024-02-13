import api from './api'
import { i18n } from '@/locales'
import Message from "@/store/modules/message"

export interface Msg {
  success: boolean
  msg: string
  obj: any | null
}

function _handleMsg(msg: any): void {
  const sb = Message()
  if (!isMsg(msg)) {
    return
  }
  if(msg.msg){
    const message = msg.success ? i18n.global.t('success') + ": " + i18n.global.t('actions.' + msg.msg) : i18n.global.t('failed') + ": " + msg.msg
    sb.showMessage(message, msg.success ? 'success' : 'error', 5000)
  }
}

function _respToMsg(resp: any): Msg {
  const data = resp.data
  if (data == null) {
    return { success: true, msg: "", obj: null }
  } else if (isMsg(data)) {
    if (data.hasOwnProperty('success')) {
        return { success: data.success, msg: data.msg, obj: data.obj || null }
    } else {
        return data
    }
  } else {
    return { success: false, msg: `unknown data: ${data}`, obj: null }
  }
}

function isMsg(obj: any): obj is Msg {
  return 'success' in obj && 'msg' in obj && 'obj' in obj
}
  
const HttpUtils = {
  async get(url: string, data: object = {}, options: any[] = []): Promise<Msg> {
    let msg: Msg
    try {
        const resp = await api.get(url, { params: data, ...options })
        msg = _respToMsg(resp)
    } catch (e: any) {
        msg = { success: false, msg: e.toString(), obj: null }
    }
    _handleMsg(msg)
    return msg
  },
  async post(url: string, data: object | null, options: any = undefined): Promise<Msg> {
    let msg: Msg
    try {
        const resp = await api.post(url, data, options)
        msg = _respToMsg(resp)
    } catch (e: any) {
        msg = { success: false, msg: e.toString(), obj: null }
    }
    _handleMsg(msg)
    return msg
  },
}

export default HttpUtils;