import { FindDiff } from '@/plugins/utils'
import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { push } from 'notivue'
import { i18n } from '@/locales'

const Data = defineStore('Data', {
  state: () => ({ 
    lastLoad: 0,
    reloadItems: localStorage.getItem("reloadItems")?.split(',')?? <string[]>[],
    subURI: "",
    onlines: {inbound: <string[]>[], outbound: <string[]>[], user: <string[]>[]},
    oldData: <{config: any, clients: any[], tlsConfigs: any[]}>{},
    config: {},
    clients: [],
    tlsConfigs: [],
  }),
  actions: {
    async loadData() {
      const msg = await HttpUtils.get('api/load', this.lastLoad >0 ? {lu: this.lastLoad} : {} )
      if(msg.success) {
        this.lastLoad = Math.floor((new Date()).getTime()/1000)

        // Set new data
        if (msg.obj.config) this.oldData.config = msg.obj.config
        if (msg.obj.clients) this.oldData.clients = msg.obj.clients
        if (msg.obj.tls) this.oldData.tlsConfigs = msg.obj.tls
        this.onlines = msg.obj.onlines
        if (msg.obj.lastLog) {
          push.error({
            title: i18n.global.t('error.core'),
            duration: 5000,
            message: msg.obj.lastLog
          })
        }
        
        if (msg.obj.config) {
          // To avoid ref copy
          const data = JSON.parse(JSON.stringify(msg.obj))
          if (data.subURI) this.subURI = data.subURI
          if (data.config) this.config = data.config
          if (data.clients) this.clients = data.clients
          if (data.tls) this.tlsConfigs = data.tls
        }
      }
    },
    async pushData() {
      const diff = {
        config: JSON.stringify(FindDiff.Config(this.config,this.oldData.config)),
        clients: JSON.stringify(FindDiff.Clients(this.clients,this.oldData.clients)),
        tls: JSON.stringify(FindDiff.Clients(this.tlsConfigs,this.oldData.tlsConfigs)),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    },
    async delInbound(index: number) {
      const diff = {
        config: JSON.stringify([{key: "inbounds", action: "del", index: index, obj: null}]),
        clients: JSON.stringify(FindDiff.Clients(this.clients,this.oldData.clients)),
        tls: JSON.stringify(FindDiff.Clients(this.tlsConfigs,this.oldData.tlsConfigs)),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    },
    async delOutbound(index: number) {
      const diff = {
        config: JSON.stringify([{key: "outbounds", action: "del", index: index, obj: null}]),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    },
    async delClient(id: number) {
      const diff = {
        config: JSON.stringify(FindDiff.Config(this.config,this.oldData.config)),
        clients:JSON.stringify([{key: "clients", action: "del", index: id, obj: null}]),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    },
    async delTls(id: number) {
      const diff = {
        tls:JSON.stringify([{key: "tls", action: "del", index: id, obj: null}]),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    }
  },
})

export default Data