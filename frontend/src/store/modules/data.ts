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
    oldData: <{config: any, clients: any[], tlsConfigs: any[], inData: any[]}>{},
    config: <any>{},
    clients: [],
    tlsConfigs: [],
    inData: [],
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
        if (msg.obj.inData) this.oldData.inData = msg.obj.inData
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
          if (data.inData) this.inData = data.inData
        }
      }
    },
    async pushData() {
      const diff = {
        config: JSON.stringify(FindDiff.Config(this.config,this.oldData.config), null, 2),
        clients: JSON.stringify(FindDiff.ArrObj(this.clients,this.oldData.clients, "clients"), null, 2),
        tls: JSON.stringify(FindDiff.ArrObj(this.tlsConfigs,this.oldData.tlsConfigs, "tls"), null, 2),
        inData: JSON.stringify(FindDiff.ArrObj(this.inData,this.oldData.inData, "inData"), null, 2),
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.lastLoad = 0
        this.loadData()
      }
    },
    async delInbound(index: number) {
      const diff = {
        config: JSON.stringify([{key: "inbounds", action: "del", index: index, obj: null}]),
        clients: JSON.stringify(FindDiff.ArrObj(this.clients,this.oldData.clients, "clients"), null, 2),
        tls: JSON.stringify(FindDiff.ArrObj(this.tlsConfigs,this.oldData.tlsConfigs, "tls"), null, 2),
        inData: <string|undefined> undefined,
      }

      // Validate inData
      let invalidInData = <any[]>[]
      this.inData.forEach((d:any) => {
        const inboundIndex = this.config.inbounds.findIndex((i:any) => i.tag == d.tag)
        if (inboundIndex == -1) invalidInData.push({key: "inData", action: "del", index: d.id, obj: null})
      })
      if (invalidInData.length>0) {
        diff.inData = JSON.stringify(invalidInData)
      }
      const msg = await HttpUtils.post('api/save',diff)
      if(msg.success) {
        this.loadData()
      }
    },
    async delInData(id: number) {
      const diff = {
        inData: JSON.stringify([{key: "inData", action: "del", index: id, obj: null}])
      }
      await HttpUtils.post('api/save',diff)
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