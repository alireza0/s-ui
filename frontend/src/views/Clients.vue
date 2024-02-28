
<template>
  <ClientModal 
    v-model="modal.visible"
    :visible="modal.visible"
    :index="modal.index"
    :data="modal.data"
    :stats="modal.stats"
    :inboundTags="inboundTags"
    @close="closeModal"
    @save="saveModal"
  />
  <QrCode
    v-model="qrcode.visible"
    :visible="qrcode.visible"
    :index="qrcode.index"
    @close="closeQrCode"
  />
  <Stats
    v-model="stats.visible"
    :visible="stats.visible"
    :resource="stats.resource"
    :tag="stats.tag"
    @close="closeStats"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showModal(-1)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in clients" :key="item.id">
      <v-card rounded="xl" elevation="5" min-width="200">
        <v-card-title>
          <v-row>
            <v-col>{{ item.name }}</v-col>
            <v-spacer></v-spacer>
            <v-col cols="auto">
              <v-switch color="primary" v-model="clients[index].enable" hideDetails density="compact" />
            </v-col>
          </v-row>
        </v-card-title>
        <v-divider></v-divider>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('pages.inbounds') }}</v-col>
            <v-col dir="ltr">
              <v-tooltip activator="parent" dir="ltr" location="bottom">
                <span v-for="i in item.inbounds.split(',')">{{ i }}<br /></span>
              </v-tooltip>
              {{ item.inbounds.split(',').length }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('stats.volume') }}</v-col>
            <v-col dir="ltr">
              {{ item.volume == 0 ? $t('unlimited') : HumanReadable.sizeFormat(item.volume) }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('date.expiry') }}</v-col>
            <v-col dir="ltr">
              {{ item.expiry == 0 ? $t('unlimited') : HumanReadable.remainedDays(item.expiry)?? $t('date.expired') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('stats.usage') }}</v-col>
            <v-col dir="ltr">
              <v-tooltip activator="parent" location="bottom">
                {{ $t('stats.upload') }}:{{ HumanReadable.sizeFormat(item.up) }}<br />
                {{ $t('stats.download') }}:{{ HumanReadable.sizeFormat(item.down) }}<br />
                <template v-if="item.volume>0">
                  {{ $t('remained') }}: {{ HumanReadable.sizeFormat(item.volume - (item.up + item.down)) }}
                </template>
              </v-tooltip>
              {{ HumanReadable.sizeFormat(item.up + item.down) }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col dir="ltr">
              <template v-if="onlines[index]">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-account-edit" @click="showModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn style="margin-inline-start:0;" icon="mdi-account-minus" color="warning" @click="delOverlay[index] = true">
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
                <v-btn color="error" variant="outlined" @click="delClient(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-qrcode" @click="showQrCode(index)" />
          <v-btn icon="mdi-chart-line" @click="showStats(item.name)" />
        </v-card-actions>
      </v-card>      
    </v-col>
  </v-row>
</template>
<script lang="ts" setup>
import Data from '@/store/modules/data'
import ClientModal from '@/layouts/modals/Client.vue'
import QrCode from '@/layouts/modals/QrCode.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Client, createClient } from '@/types/clients'
import { computed, ref } from 'vue'
import { Config, V2rayApiStats } from '@/types/config'
import { InTypes, Inbound,InboundWithUser, ShadowTLS, VLESS } from '@/types/inbounds'
import { Link, LinkUtil } from '@/plugins/link'
import { HumanReadable } from '@/plugins/utils'

const clients = computed((): any[] => {
  return Data().clients
})

const onlines = computed(() => {
  return Data().onlines.user ? clients.value.map(c => Data().onlines.user.includes(c.name)) : []
})

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const v2rayStats = computed((): V2rayApiStats => {
  return <V2rayApiStats> appConfig.value.experimental.v2ray_api.stats
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> appConfig.value.inbounds
})

const inboundTags = computed((): string[] => {
  if (!inbounds.value) return []
  return inbounds.value.filter(i => i.tag != "" && Object.hasOwn(i,'users')).map(i => i.tag)
})

const modal = ref({
  visible: false,
  index: -1,
  data: "",
  stats: false,
})

const delOverlay = ref(new Array<boolean>(clients.value.length).fill(false))

const showModal = (index: number) => {
  modal.value.index = index
  modal.value.data = index == -1 ? '' : JSON.stringify(clients.value[index])
  modal.value.stats = index == -1 ? false : v2rayStats.value.users.includes(clients.value[index].name)
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = (data:any, stats:boolean) => {
  const inboundTags: string[] = data.inbounds.split(',')?? []
  let oldName:string = ""
  if(modal.value.index == -1) {
    clients.value.push(data)
  } else {
    const oldData = createClient(clients.value[modal.value.index])
    oldName = oldData.name
    oldData.inbounds.split(',').forEach((i:string) => {
      if (!inboundTags.includes(i)) inboundTags.push(i)
    })
    clients.value[modal.value.index] = data
  }

  // Rebuild affected inbounds
  buildInboundsUsers(inboundTags)

  // Rebuild links
  data.links = updateLinks(data)

  // Set Client Stats
  const sIndex = v2rayStats.value.users.findIndex(i => i == data.name) // Find if new user exists

  if (oldName != data.name) {
    v2rayStats.value.users = v2rayStats.value.users.filter(item => item != oldName)
  }

  if (stats) {
    // Add if dos not exist
    if (data.name.length>0 && sIndex == -1) v2rayStats.value.users.push(data.name)
  } else {
    // Delete if exists
    if (sIndex != -1) v2rayStats.value.users.splice(sIndex,1)
  }

  modal.value.visible = false
}
const buildInboundsUsers = (inboundTags:string[]) => {
    inboundTags.forEach(tag => {
      const inbound_index = inbounds.value.findIndex(i => i.tag == tag)
      if (inbound_index != -1){
        const users = <any>[]
        const newInbound = <InboundWithUser>inbounds.value[inbound_index]
        const inboundClients = clients.value.filter(c => c.enable && c.inbounds.split(',').includes(tag))
        inboundClients.forEach(c => {
          const clientConfig = JSON.parse(c.config)
          // Remove flow in non tls VLESS
          if (newInbound.type == InTypes.VLESS) {
            const vlessInbound = <VLESS>newInbound
            if (!vlessInbound.tls?.enabled || vlessInbound.transport?.type) delete(clientConfig["vless"].flow)
          }
          users.push(clientConfig[newInbound.type])
        })
        newInbound.users = users

        // Exceptions for Naive and ShadowTLSv3
        if (users.length == 0){
          if (newInbound.type == InTypes.Naive) {
            newInbound.users = <any>[{}]
          } else {
            if (newInbound.type == InTypes.ShadowTLS){
              const ssTls = <ShadowTLS>newInbound
              if (ssTls.version == 3) newInbound.users = <any>[{}]
            }
          }
        }

        inbounds.value[inbound_index] = newInbound
      }
    })
}
const updateLinks = (c:Client):string => {
  const clientInbounds = <Inbound[]>inbounds.value.filter(i => c.inbounds.split(',').includes(i.tag))
  const newLinks = <Link[]>[]
  clientInbounds.forEach(i =>{
    const uri = LinkUtil.linkGenerator(c.name,i)
    if (uri.length>0){
      newLinks.push(<Link>{ type: 'local', remark: i.tag, uri: uri })
    }
  })
  let links = c.links && c.links.length>0? <Link[]>JSON.parse(c.links) : <Link[]>[]
  links = [...newLinks, ...links.filter(l => l.type != 'local')]

  return JSON.stringify(links)
}
const delClient = (clientIndex: number) => {
  const id = clients.value[clientIndex].id
  const oldData = createClient(clients.value[clientIndex])

  // Delete stats if exists and will be orphaned
  const tagCounts = clients.value.filter(i => i.name == oldData.name).length
  const sIndex = v2rayStats.value.users.findIndex(i => i == oldData.name)
  if (tagCounts == 1 && sIndex != -1){
    v2rayStats.value.users.splice(sIndex,1)
  }

  clients.value.splice(clientIndex,1)
  buildInboundsUsers(oldData.inbounds.split(','))
  if (id>0) Data().delClient(id)
  delOverlay.value[clientIndex] = false
}

const qrcode = ref({
  visible: false,
  index: 0,
})

const showQrCode = (index: number) => {
  qrcode.value.index = index
  qrcode.value.visible = true
}
const closeQrCode = () => {
  qrcode.value.visible = false
}

const stats = ref({
  visible: false,
  resource: "user",
  tag: "",
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>