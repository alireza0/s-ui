import { FindDiff } from '@/plugins/utils'
import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { onMounted } from 'vue'

const Data = defineStore('Data', {
  state: () => ({ 
    lastLoad: 0,
    reloadItems: localStorage.getItem("reloadItems")?.split(',')?? <string[]>[],
    subURI: "",
    onlines: {inbound: <string[]>[], outbound: <string[]>[], user: <string[]>[]},
    oldData: <{config: any, clients: any[]}>{},
    config: {},
    clients: [],
  }),
  actions: {
    async loadData() {
      const msg = await HttpUtils.get('api/load', this.lastLoad >0 ? {lu: this.lastLoad} : {} )
      if(msg.success) {
        this.lastLoad = Math.floor((new Date()).getTime()/1000)

        // Set new data
        const data = JSON.parse(msg.obj)
        if (data.config) this.config = data.config
        if (data.clients) this.clients = data.clients
        if (data.subURI) this.subURI = data.subURI
        this.onlines = data.onlines

        // To avoid ref copy
        if (data.config) this.oldData.config = { ...JSON.parse(msg.obj).config }
        if (data.clients) this.oldData.clients = [ ...JSON.parse(msg.obj).clients ]
      }
    },
    async pushData() {
      const diff = {
        config: JSON.stringify(FindDiff.Config(this.config,this.oldData.config)),
        clients: JSON.stringify(FindDiff.Clients(this.clients,this.oldData.clients)),
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
    }
  },
})

export default Data