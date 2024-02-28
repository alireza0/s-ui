<template>
  <InboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :stats="modal.stats"
    :data="modal.data"
    @close="closeModal"
    @save="saveModal"
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
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>inbounds" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle>
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('in.addr') }}</v-col>
            <v-col dir="ltr">
              {{ item.listen }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col dir="ltr">
              {{ item.listen_port }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.tls') }}</v-col>
            <v-col dir="ltr">
              {{ Object.hasOwn(item,'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-'  }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.clients') }}</v-col>
            <v-col dir="ltr">
              <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="Object.hasOwn(item,'users')">
                <span v-for="u in findInbounsUsers(item)">{{ u }}<br /></span>
              </v-tooltip>
              {{ Array.isArray(item.users) ? item.users.length : '-' }}
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
          <v-btn icon="mdi-file-edit" @click="showModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delOverlay[index] = true">
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
                <v-btn color="error" variant="outlined" @click="delInbound(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag)" />
        </v-card-actions>
      </v-card>      
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config, V2rayApiStats } from '@/types/config'
import { computed, ref } from 'vue'
import { InTypes, Inbound, InboundWithUser, ShadowTLS, VLESS } from '@/types/inbounds'
import { Client } from '@/types/clients'
import { Link, LinkUtil } from '@/plugins/link'

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> appConfig.value.inbounds
})

const clients = computed((): Client[] => {
  return <Client[]> Data().clients
})

const onlines = computed(() => {
  return Data().onlines.inbound ? inbounds.value.map(i => Data().onlines.inbound.includes(i.tag)) : []
})

const v2rayStats = computed((): V2rayApiStats => {
  return <V2rayApiStats> appConfig.value.experimental.v2ray_api.stats
})

const modal = ref({
  visible: false,
  id: -1,
  data: "",
  stats: false,
})

let delOverlay = ref(new Array<boolean>)

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == -1 ? '' : JSON.stringify(inbounds.value[id])
  modal.value.stats = id == -1 ? false : v2rayStats.value.inbounds.includes(inbounds.value[id].tag)
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = (data:Inbound, stats: boolean) => {
  // New or Edit
  if (modal.value.id == -1) {
    inbounds.value.push(data)
    if (stats && data.tag.length>0) {
      v2rayStats.value.inbounds.push(data.tag)
    }
  } else {
    const oldTag = inbounds.value[modal.value.id].tag
    const sIndex = v2rayStats.value.inbounds.findIndex(i => i == data.tag) // Find if new tag exists

    if (oldTag != data.tag) {
      v2rayStats.value.inbounds = v2rayStats.value.inbounds.filter(item => item != oldTag)
      changeClientInboundsTag(oldTag,data.tag)
    }

    if (stats) {
      // Add if dos not exist
      if (data.tag.length>0 && sIndex == -1) v2rayStats.value.inbounds.push(data.tag)
    } else {
      // Delete if exists
      if (sIndex != -1) v2rayStats.value.inbounds.splice(sIndex,1)
    }

    inbounds.value[modal.value.id] = data
  }
  // Set users
  data = buildInboundsUsers(data)
  // Update links
  if (Object.hasOwn(data,'users')) updateLinks(data)
  modal.value.visible = false
}
const updateLinks = (i: InboundWithUser) => {
  if(i.users && i.users.length>0){
    i.users.forEach((u:any) => {
      const client = clients.value.find(c => u.username? c.name == u.username : c.name == u.name)
      if (client){
        const clientInbounds = <Inbound[]>inbounds.value.filter(i => client?.inbounds.split(',').includes(i.tag))
        const newLinks = <Link[]>[]
        clientInbounds.forEach(i =>{
          const uri = LinkUtil.linkGenerator(client.name,i)
          if (uri.length>0){
            newLinks.push(<Link>{ type: 'local', remark: i.tag, uri: uri })
          }
        })
        let links = client.links && client.links.length>0? <Link[]>JSON.parse(client.links) : <Link[]>[]
        links = [...newLinks, ...links.filter(l => l.type != 'local')]

        client.links = JSON.stringify(links)
      }
    })
  }
}
const delInbound = (index: number) => {
  const inb = inbounds.value[index]
  inbounds.value.splice(index,1)
  const tag = inb.tag

  if (Object.hasOwn(inb,'users')){
    const inbU = <InboundWithUser>inb
    if (inbU.users && inbU.users.length>0){
      inbU.users.forEach((u:any) => {
        const c_index = clients.value.findIndex(c => u.username? u.username == c.name : u.user == c.name)
        if (c_index != -1) {
          const clientInbounds = clients.value[c_index].inbounds.split(',').filter((x:string) => x!=tag)
          clients.value[c_index].inbounds = clientInbounds.join(',')
        }
      })
    }
  }


  // Delete stats if exists and will be orphaned
  const tagCounts = inbounds.value.filter(i => i.tag == inb.tag).length
  const sIndex = v2rayStats.value.inbounds.findIndex(i => i == inb.tag)
  if (tagCounts == 1 && sIndex != -1){
    v2rayStats.value.inbounds.splice(sIndex,1)
  }
  if (index < Data().oldData.config.inbounds.length){
    Data().delInbound(index)
  }
  delOverlay.value[index] = false
}
const buildInboundsUsers = (inbound:InboundWithUser):Inbound => {
    const users = <any>[]
    const inboundClients = clients.value.filter(c => c.enable && c.inbounds.split(',').includes(inbound.tag))
    inboundClients.forEach(c => {
      const clientConfig = JSON.parse(c.config)
      // Remove flow in non tls VLESS
      if (inbound.type == InTypes.VLESS) {
        const vlessInbound = <VLESS>inbound
        if (!vlessInbound.tls?.enabled || vlessInbound.transport?.type) delete(clientConfig["vless"].flow)
      }
      users.push(clientConfig[inbound.type])
    })
    inbound.users = users

    // Exceptions for Naive and ShadowTLSv3
    if (users.length == 0){
      if (inbound.type == InTypes.Naive){
        inbound.users = <any>[{}]
      } else {
        if (inbound.type == InTypes.ShadowTLS){
          const ssTls = <ShadowTLS>inbound
          if (ssTls.version == 3) inbound.users = <any>[{}]
        }
      }
    }

    return <Inbound>inbound
}
const changeClientInboundsTag = (oldtag: string, newTag:string) => {
  clients.value.forEach((c, c_index) => {
    const inboundsArray = c.inbounds.split(',')
    const inbound_index = inboundsArray.findIndex(i => i == oldtag)
    if (inbound_index != -1) {
      inboundsArray[inbound_index] = newTag
      clients.value[c_index].inbounds = inboundsArray.join(',')
    }
  })
}
const findInbounsUsers = (inbound: InboundWithUser): string[] => {
  if (inbound.users === null || !Array.isArray(inbound.users) || inbound.users.length == 0) return []

  const users = inbound.users.map(user => "username" in user ? user.username : user.name)
  return users
}

const stats = ref({
  visible: false,
  resource: "inbound",
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