import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { push } from 'notivue'
import { i18n } from '@/locales'
import { Inbound } from '@/types/inbounds'
import { Outbound } from '@/types/outbounds'
import { Endpoint } from '@/types/endpoints'

const Data = defineStore('Data', {
  state: () => ({ 
    lastLoad: 0,
    reloadItems: localStorage.getItem("reloadItems")?.split(',')?? <string[]>[],
    subURI: "",
    onlines: {inbound: <string[]>[], outbound: <string[]>[], user: <string[]>[]},
    config: <any>{},
    inbounds: <Inbound[]>[],
    outbounds: <Outbound[]>[],
    endpoints: <Endpoint[]>[],
    clients: [],
    tlsConfigs: <any[]>[],
  }),
  actions: {
    async loadData() {
      const msg = await HttpUtils.get('api/load', this.lastLoad >0 ? {lu: this.lastLoad} : {} )
      if(msg.success) {
        this.onlines = msg.obj.onlines
        if (msg.obj.lastLog) {
          push.error({
            title: i18n.global.t('error.core'),
            duration: 5000,
            message: msg.obj.lastLog
          })
        }
        
        if (msg.obj.config) {
          this.setNewData(msg.obj)
        }
      }
    },
    setNewData(data: any) {
      this.lastLoad = Math.floor((new Date()).getTime()/1000)
      if (data.subURI) this.subURI = data.subURI
      if (data.config) this.config = data.config
      if (data.clients) this.clients = data.clients
      if (data.inbounds) this.inbounds = data.inbounds
      if (data.outbounds) this.outbounds = data.outbounds
      if (data.endpoints) this.endpoints = data.endpoints
      if (data.tls) this.tlsConfigs = data.tls
    },
    async loadInbounds(ids: number[]): Promise<Inbound[]> {
      const options = ids.length > 0 ? {id: ids.join(",")} : {}
      const msg = await HttpUtils.get('api/inbounds', options)
      if(msg.success) {
        return msg.obj.inbounds
      }
      return <Inbound[]>[]
    },
    async save (object: string, action: string, data: any): Promise<boolean> {
      let postData = {
        object: object,
        action: action,
        data: JSON.stringify(data, null, 2),
      }
      const msg = await HttpUtils.post('api/save', postData)
      if (msg.success) {
        const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
        push.success({
          title: i18n.global.t('success'),
          duration: 5000,
          message: i18n.global.t('actions.' + action) + " " + i18n.global.t('objects.' + objectName)
        })
        this.setNewData(msg.obj)
      }
      return msg.success
    }
  }
})

export default Data