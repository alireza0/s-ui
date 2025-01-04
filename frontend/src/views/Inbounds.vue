<template>
  <InboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :inTags="inTags"
    :outTags="outTags"
    :tlsConfigs="tlsConfigs"
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
      <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>inbounds" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle style="margin-top: -20px;">
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
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col dir="ltr">
              {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.clients') }}</v-col>
            <v-col dir="ltr">
              <template v-if="inboundWithUsers.includes(item.tag)">
                <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="findInboundUsers(item.tag).length > 0">
                  <span v-for="u in findInboundUsers(item.tag)">{{ u }}<br /></span>
                </v-tooltip>
                {{ findInboundUsers(item.tag).length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col dir="ltr">
              <template v-if="onlines.includes(item.tag)">
                <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
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
                <v-btn color="error" variant="outlined" @click="delInbound(item.id)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>      
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config } from '@/types/config'
import { computed, onMounted, ref } from 'vue'
import { Inbound, inboundWithUsers } from '@/types/inbounds'
import { Client } from '@/types/clients'
import { Link, LinkUtil } from '@/plugins/link'
import { i18n } from '@/locales'
import { push } from 'notivue'

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> Data().inbounds
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
})

const inTags = computed((): string[] => {
  return inbounds.value?.map(i => i.tag)
})

const outTags = computed((): string[] => {
  return appConfig.value.outbounds?.map(i => i.tag)
})

const clients = computed((): Client[] => {
  return <Client[]> Data().clients
})

const onlines = computed(() => {
  return Data().onlines.inbound ? inbounds.value.map(i => Data().onlines.inbound.includes(i.tag)) : []
})

const modal = ref({
  visible: false,
  id: 0,
})

let delOverlay = ref(new Array<boolean>)

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = async (data:Inbound) => {
  // Check duplicate tag
  const oldInbound = modal.value.id > 0 ? inbounds.value.findLast(i => i.id == modal.value.id) : null
  if (data.tag != oldInbound?.tag && inTags.value.includes(data.tag)) {
    push.error({
      message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
    })
    return
  }
  
  let userLinkDiff = []
  // Update links
  if (data.id > 0 && oldInbound != null) {
    userLinkDiff = updateLinks(data,oldInbound)
  }

  // save data
  const success = await Data().save("inbounds", modal.value.id == 0 ? "new" : "edit", data, userLinkDiff)
  if (success) modal.value.visible = false
}
const updateLinks = (i: Inbound, o: Inbound): any[] => {
  let diff = <any[]>[]
  const uClients = clients.value.filter(c => c.inbounds.includes(i.id))
  if (uClients.length == 0) return diff

  if (inboundWithUsers.includes(o.type) && !inboundWithUsers.includes(i.type)){
    // Remove old inbound links if new type does not support users
    uClients.forEach((u:Client) => {
      u.inbounds = u.inbounds.filter(i => i != o.id)
      const otherLocalLinks = u.links.filter(l => l.type == 'local' && l.remark != o.tag)
      let links = u.links && u.links.length>0? u.links : <Link[]>[]
      links = [...otherLocalLinks, ...links.filter(l => l.type != 'local')]

      diff.push({ id: u.id, links: links, inbounds: u.inbounds })
    })
  } else if(inboundWithUsers.includes(i.type)){
    // Add new inbound links if new type supports users
    const tls = tlsConfigs?.value.findLast((t:any) => t.id == i.tls_id)
    uClients.forEach((u:Client) => {
      const otherLocalLinks = u.links.filter(l => l.type == 'local' && l.remark != i.tag)
      const uris = LinkUtil.linkGenerator(u,i, tls, i.addrs)
      let newLinks = <Link[]>[]
      if (uris.length>0){
        uris.forEach(uri => {
          newLinks.push(<Link>{ type: 'local', remark: i.tag, uri: uri })
        })
      }
      let links = u.links && u.links.length>0? u.links : <Link[]>[]
      links = [...otherLocalLinks, ...newLinks, ...links.filter(l => l.type != 'local')]

      diff.push({ id: u.id, links: links, inbounds: u.inbounds })
    })
  }

  return diff
}
const delInbound = async (id: number) => {
  const index = inbounds.value.findIndex(i => i.id == id)
  const inb = inbounds.value[index]
  const tag = inb.tag

  let diff = <any[]>[]
  // delete inbound in client table
  const inboundClients = clients.value.filter(c => c.inbounds.includes(id))
  inboundClients.forEach((c:Client) => {
    c.inbounds = c.inbounds.filter((x:number) => x!=id)
    c.links = c.links.filter((x:any) => x.remark!=tag)
    diff.push({ id: c.id, links: c.links, inbounds: c.inbounds })
  })

  const success = await Data().save("inbounds", "del", tag, diff)
  if (success) delOverlay.value[index] = false
}

const findInboundUsers = (i: Inbound): string[] => {
  return clients.value.filter(c => c.inbounds.includes(i.id)).map(c => c.name)
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