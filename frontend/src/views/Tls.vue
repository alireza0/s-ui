<template>
    <TlsVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :index="modal.index"
    :data="modal.data"
    @close="closeModal"
    @save="saveModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showModal(-1)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>tlsConfigs" :key="item.id">
      <v-card rounded="xl" elevation="5" min-width="200" :title="(item.id? item.id + '. ' : '*') + item.name">
        <v-card-subtitle style="margin-top: -20px;">
          {{ item.server?.server_name?.length>0 ? item.server.server_name : "-" }}
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('pages.inbounds') }}</v-col>
            <v-col dir="ltr">
              <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="item.inbounds?.length>0">
                <span v-for="i in item.inbounds">{{ i }}<br /></span>
              </v-tooltip>
              {{ item.inbounds?.length }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>ACME</v-col>
            <v-col dir="ltr">
              {{ $t(item.server?.acme == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>ECH</v-col>
            <v-col dir="ltr">
              {{ $t(item.server?.ech == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Reality</v-col>
            <v-col dir="ltr">
              {{ $t(item.server?.reality == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn v-if="item.inbounds?.length == 0" icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delTls(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-content-duplicate" @click="clone(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.clone')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import TlsVue from '@/layouts/modals/Tls.vue'
import Data from '@/store/modules/data'
import { computed, ref } from 'vue'
import { Config } from '@/types/config'
import { Inbound } from '@/types/inbounds'
import { Client } from '@/types/clients'
import { Link, LinkUtil } from '@/plugins/link'
import { fillData } from '@/plugins/outJson'

const tlsConfigs = computed((): any[] => {
  return Data().tlsConfigs
})

const inbounds = computed((): any[] => {
  return <any[]>(<Config>Data().config)?.inbounds
})

const inData = computed((): any[] => {
  return <any[]> Data().inData
})

const clients = computed((): any[] => {
  return <Client[]>Data().clients
})

const modal = ref({
  visible: false,
  index: -1,
  data: "",
})

const delOverlay = ref(new Array<boolean>(tlsConfigs.value.length).fill(false))

const showModal = (index: number) => {
  modal.value.index = index
  modal.value.data = index == -1 ? '{}' : JSON.stringify(tlsConfigs.value[index])
  modal.value.visible = true
}
const clone = (index: number) => {
  let data = JSON.parse(JSON.stringify(tlsConfigs.value[index]))
  data.id = 0
  data.inbounds = []
  while (tlsConfigs.value.findIndex(t => t.name == data.name) != -1){
    data.name += "-copy"
  }
  saveModal(data)
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = (data:any) => {
  // New or Edit
  if (modal.value.index == -1) {
    tlsConfigs.value.push(data)
  } else {
    tlsConfigs.value[modal.value.index] = data
    inbounds?.value.filter(i => tlsConfigs.value[modal.value.index].inbounds.includes(i.tag)).forEach(i =>{
      if (i.tls != undefined) i.tls = data.server
      updateInData(i,data.client)
      updateLinks(i,data.client)
    })
  }
  modal.value.visible = false
}

const delTls = (index: number) => {
  if (index < Data().oldData.tlsConfigs.length){
    Data().delTls(tlsConfigs.value[index].id)
  }
  tlsConfigs.value.splice(index,1)
  delOverlay.value[index] = false
}

const updateLinks = (i:any,tlsClient:any) => {
  if(i.users){
    const uClients = clients.value.filter(c => c.inbounds.includes(i.tag))
    uClients.forEach((client:any) => {
      const clientInbounds = <Inbound[]>inbounds.value.filter(inb => client?.inbounds.includes(inb.tag))
      const newLinks = <Link[]>[]
      clientInbounds.forEach(i =>{
        const cData = <any>Data().inData?.findLast((d:any) => d.tag == i.tag)
        const addrs = cData ? <any[]>cData.addrs : []
        const uris = LinkUtil.linkGenerator(client,i, tlsClient, addrs)
        if (uris.length>0){
          uris.forEach(uri => {
            newLinks.push(<Link>{ type: 'local', remark: i.tag, uri: uri })
          })
        }
      })
      let links = client.links && client.links.length>0? client.links : <Link[]>[]
      links = [...newLinks, ...links.filter((l:Link) => l.type != 'local')]

      client.links = links
    })
  }
}

const updateInData = (i:any, c:any) => {
  const inDataIndex = inData.value.findIndex(d => d.tag == i.tag)
  if (inDataIndex != -1) {
    fillData(inData.value[inDataIndex].outJson, i, c)
  }
}
</script>
