
<template>
  <ClientModal 
    v-model="modal.visible"
    :visible="modal.visible"
    :index="modal.index"
    :data="modal.data"
    :groups="groups"
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
  <v-row justify="center" align="center">
    <v-col cols="auto">
      <v-btn color="primary" @click="showModal(-1)">{{ $t('actions.add') }}</v-btn>
    </v-col>
    <v-col cols="auto">
      <v-menu v-model="filterMenu" :close-on-content-click="false" location="bottom center">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('filter') }}
            <v-badge color="error" dot v-if="filterSettings.enabled" floating />
          </v-btn>
        </template>
        <v-card>
          <v-container>
            <v-row>
              <v-col>
                <v-select
                variant="underlined"
                density="compact"
                :label="$t('type')"
                :items="filterItems"
                v-model="filterSettings.state">
                </v-select>
              </v-col>
            </v-row>
            <v-row>
              <v-col>
                <v-select
                variant="underlined"
                density="compact"
                :label="$t('client.group')"
                :items="[ {title: $t('all'), value: '-'}, ...groups.map(g => ({ title: g.length>0 ? g : $t('none'), value: g}))]"
                v-model="filterSettings.group">
                </v-select>
              </v-col>
            </v-row>
            <v-row>
              <v-col>
                <v-text-field
                variant="underlined"
                density="compact"
                :label="$t('client.name')"
                v-model="filterSettings.text">
                </v-text-field>
              </v-col>
            </v-row>
          </v-container>
          <v-card-actions>
            <v-card-actions>
              <v-spacer></v-spacer>
              <v-btn
                color="blue-darken-1"
                variant="outlined"
                @click="clearFilter"
              >
                {{ $t('actions.del') }}
              </v-btn>
              <v-btn
                color="blue-darken-1"
                variant="tonal"
                @click="doFilter"
              >
                {{ $t('actions.update') }}
              </v-btn>
            </v-card-actions>
          </v-card-actions>
        </v-card>
      </v-menu>
    </v-col>
    <v-col cols="auto">
      <v-switch v-model="tableView" color="primary" hide-details>
        <template v-slot:label><v-icon icon="mdi-table"></v-icon></template>
      </v-switch>
    </v-col>
  </v-row>
  <template v-for="group in groups" v-if="!tableView">
    <v-row>
      <v-col class="v-card-subtitle">
        {{ group.length>0 ? group : $t('none') }}
        <v-badge :content="(filterSettings.enabled ? filterSettings.filteredClients : clients).filter(c => c.group == group).length" inline color="info" />
        <v-icon
          :icon="openedGroups.includes(group) ? 'mdi-arrow-collapse-up' : 'mdi-arrow-collapse-down'"
          size="small"
          variant="text"
          @click="toggleGroupOpen(group)"
        ></v-icon>
      </v-col>
    </v-row>
    <v-row v-if="openedGroups.includes(group)">
    <template v-for="item in (filterSettings.enabled ? filterSettings.filteredClients : clients).filter(c => c.group == group)" :key="item.id">
      <v-col cols="12" sm="4" md="3" lg="2">
        <v-card rounded="xl" elevation="5" min-width="200">
          <v-card-title>
            <v-row>
              <v-col>{{ item.name }}</v-col>
              <v-spacer></v-spacer>
              <v-col cols="auto">
                <v-switch color="primary"
                v-model="item.enable"
                @update:model-value="buildInboundsUsers(item.inbounds)"
                hideDetails density="compact" />
              </v-col>
            </v-row>
          </v-card-title>
          <v-card-subtitle style="margin-top: -20px;">
            <v-row>
              <v-col>{{ item.desc }}</v-col>
            </v-row>
          </v-card-subtitle>
          <v-card-text>
            <v-row>
              <v-col>{{ $t('pages.inbounds') }}</v-col>
              <v-col dir="ltr">
                <v-tooltip activator="parent" dir="ltr" location="bottom" v-if="item.inbounds != ''">
                  <span v-for="i in item.inbounds">{{ i }}<br /></span>
                </v-tooltip>
                {{ item.inbounds.length }}
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
                <template v-if="isOnline(item.name).value">
                  <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
                </template>
                <template v-else>-</template>
              </v-col>
            </v-row>
          </v-card-text>
          <v-divider></v-divider>
          <v-card-actions style="padding: 0;">
            <v-btn icon="mdi-account-edit" @click="showModal(item.id)">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('actions.edit') + item.id"></v-tooltip>
            </v-btn>
            <v-btn style="margin-inline-start:0;" icon="mdi-account-minus" color="warning" @click="delOverlay[clients.findIndex(c => c.id == item.id)] = true">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
            </v-btn>
            <v-overlay
              v-model="delOverlay[clients.findIndex(c => c.id == item.id)]"
              contained
              class="align-center justify-center"
            >
              <v-card :title="$t('actions.del')" rounded="lg">
                <v-divider></v-divider>
                <v-card-text>{{ $t('confirm') }}</v-card-text>
                <v-card-actions>
                  <v-btn color="error" variant="outlined" @click="delClient(item.id)">{{ $t('yes') }}</v-btn>
                  <v-btn color="success" variant="outlined" @click="delOverlay[clients.findIndex(c => c.id == item.id)] = false">{{ $t('no') }}</v-btn>
                </v-card-actions>
              </v-card>
            </v-overlay>
            <v-btn icon="mdi-qrcode" @click="showQrCode(item.id)">
              <v-icon />
              <v-tooltip activator="parent" location="top" text="QR-Code"></v-tooltip>
            </v-btn>
            <v-btn icon="mdi-chart-line" @click="showStats(item.name)" v-if="v2rayStats.users.includes(item.name)">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
            </v-btn>
          </v-card-actions>
        </v-card>      
      </v-col>
    </template>
    </v-row>
  </template>
  <v-row v-else>
    <v-col cols="12">
      <v-data-table
        :headers="headers"
        :items="filterSettings.enabled ? filterSettings.filteredClients : clients"
        :hide-default-footer="filterSettings.enabled ? filterSettings.filteredClients.length<=10 : clients.length<=10"
        hide-no-data
        fixed-header
        :group-by="groupBy"
        item-value="name"
        :mobile="smAndDown"
        mobile-breakpoint="sm"
        width="100%"
        class="elevation-3 rounded"
        >
        <template v-slot:group-header="{ item, columns, toggleGroup, isGroupOpen }">
          <tr>
            <td :colspan="columns.length" @click="toggleGroup(item)" style="min-height: fit-content; text-align: center;">
              <v-icon :icon="isGroupOpen(item) ? '$expand' : '$next'"></v-icon>
              {{ item.value.length>0 ? item.value : $t('none') }}
              <v-badge :content="(filterSettings.enabled ? filterSettings.filteredClients : clients).filter(c => c.group == item.value).length" inline color="success" />
            </td>
          </tr>
        </template>
        <template v-slot:item.volume="{ item }">
          <div class="text-start">
            <v-chip
              size="small"
              label
            >{{ item.volume == 0 ? $t('unlimited') : HumanReadable.sizeFormat(item.volume) }}</v-chip>
          </div>
        </template>
        <template v-slot:item.expiry="{ item }">
          <div class="text-start">
            <v-chip
              size="small"
              label
            >{{ item.expiry == 0 ? $t('unlimited') : HumanReadable.remainedDays(item.expiry)?? $t('date.expired') }}</v-chip>
          </div>
        </template>
        <template v-slot:item.online="{ item }">
          <div class="text-start">
            <template v-if="isOnline(item.name).value">
              <v-chip density="comfortable" size="small" color="success" variant="flat">{{ $t('online') }}</v-chip>
            </template>
            <template v-else>-</template>
          </div>
        </template>
        <template v-slot:item.actions="{ item }">
        <v-icon
          class="me-2"
          @click="showModal(item.id)"
        >
          mdi-pencil
        </v-icon>
        <v-menu
          v-model="delOverlay[clients.findIndex(c => c.id == item.id)]"
          :close-on-content-click="false"
          location="top center"
        >
          <template v-slot:activator="{ props }">
            <v-icon
              class="me-2"
              color="error"
              v-bind="props"
            >
              mdi-delete
            </v-icon>
          </template>
          <v-card :title="$t('actions.del')" rounded="lg">
            <v-divider></v-divider>
            <v-card-text>{{ $t('confirm') }}</v-card-text>
            <v-card-actions>
              <v-btn color="error" variant="outlined" @click="delClient(item.id)">{{ $t('yes') }}</v-btn>
              <v-btn color="success" variant="outlined" @click="delOverlay[clients.findIndex(c => c.id == item.id)] = false">{{ $t('no') }}</v-btn>
            </v-card-actions>
          </v-card>
        </v-menu>
        <v-icon
          class="me-2"
          @click="showQrCode(item.id)"
        >
          mdi-qrcode
        </v-icon>
        <v-icon icon="mdi-chart-line" @click="showStats(item.name)" v-if="v2rayStats.users.includes(item.name)">
          <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')"></v-tooltip>
        </v-icon>
      </template>
      </v-data-table>
    </v-col>
  </v-row>
</template>
<style>
.v-data-table__tr--mobile td {
  height: fit-content;
  min-height: 36px !important;
}
.v-data-table__tr--mobile td div {
  width:max-content;
}
</style>
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
import { i18n } from '@/locales'
import { push } from 'notivue'
import { useDisplay } from 'vuetify'

const { smAndDown } = useDisplay()

const clients = computed((): any[] => {
  return Data().clients
})

const isOnline = (cname: string) => computed(() => {
  return Data().onlines?.user ? Data().onlines.user.includes(cname) : false
})

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const v2rayStats = computed((): V2rayApiStats => {
  return <V2rayApiStats> appConfig.value.experimental.v2ray_api.stats
})

const inbounds = computed((): Inbound[] => {
  return <Inbound[]> appConfig.value?.inbounds
})

const inboundTags = computed((): string[] => {
  if (!inbounds.value) return []
  return inbounds.value?.filter(i => i.tag != "" && Object.hasOwn(i,'users')).map(i => i.tag)
})

const groups = computed((): string[] => {
  if (!clients.value) return []
  if (filterSettings?.value.enabled) return Array.from(new Set(filterSettings.value.filteredClients?.map(c => c.group)))
  return Array.from(new Set(clients.value?.map(c => c.group)))
})

const filterMenu = ref(false)
const filterSettings = ref({
  enabled: false,
  state: '',
  group: '-',
  text: '',
  filteredClients: <any[]>[]
})
const tableView = ref(false)

const filterItems = [
  { title: i18n.global.t('none'), value: '' },
  { title: i18n.global.t('disable'), value: 'disable' },
  { title: i18n.global.t('date.expired'), value: 'expired' },
  { title: i18n.global.t('online'), value: 'online' },
]

const headers = [
  { title: i18n.global.t('client.name'), key: 'name' },
  { title: i18n.global.t('client.desc'), key: 'desc', sortable: false },
  { title: i18n.global.t('actions.action'), key: 'actions', sortable: false},
  { title: i18n.global.t('stats.volume'), key: 'volume' },
  { title: i18n.global.t('date.expiry'), key: 'expiry' },
  { title: i18n.global.t('online'), key: 'online' },
  { key: 'data-table-group', width: 0 },
]
const groupBy = [
  {
    key: 'group'
  }
]

const modal = ref({
  visible: false,
  index: -1,
  data: "",
  stats: false,
})

const delOverlay = ref(new Array<boolean>(clients.value.length).fill(false))

const showModal = (id: number) => {
  const index = id == -1 ? -1 : clients.value.findIndex(c => c.id == id)
  modal.value.index = index
  modal.value.data = index == -1 ? '' : JSON.stringify(clients.value[index])
  modal.value.stats = index == -1 ? false : v2rayStats.value.users.includes(clients.value[index].name)
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = (data:any, stats:boolean) => {
  // Check duplicate name
  const oldName = modal.value.index != -1 ? clients.value[modal.value.index].name : null
  if (data.name != oldName && clients.value.findIndex(c => c.name == data.name) != -1) {
    push.error({
      message: i18n.global.t('error.dplData') + ": " + i18n.global.t('client.name')
    })
    return
  }
  if(modal.value.index == -1) {
    clients.value.push(data)
  } else {
    clients.value[modal.value.index] = data
  }

  // Rebuild affected inbounds
  buildInboundsUsers(data.inbounds)

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
        const inboundClients = clients.value.filter(c => c.enable && c.inbounds.includes(tag))
        inboundClients.forEach(c => {
          // Remove flow in non tls VLESS
          if (newInbound.type == InTypes.VLESS) {
            const vlessInbound = <VLESS>newInbound
            if (!vlessInbound.tls?.enabled || vlessInbound.transport?.type) delete(c.config?.vless?.flow)
          }
          users.push(c.config[newInbound.type])
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
const updateLinks = (c:Client):Link[] => {
  const clientInbounds = <Inbound[]>inbounds.value.filter(i => c.inbounds.includes(i.tag))
  const newLinks = <Link[]>[]
  clientInbounds.forEach(i =>{
    const tlsConfig = <any>Data().tlsConfigs?.findLast((t:any) => t.inbounds.includes(i.tag))
    const cData = <any>Data().inData?.findLast((d:any) => d.tag == i.tag)
    const addrs = cData ? <any[]>cData.addrs : []
    const uris = LinkUtil.linkGenerator(c,i, tlsConfig?.client?? {}, addrs)
    if (uris.length>0){
      uris.forEach(uri => {
        newLinks.push(<Link>{ type: 'local', remark: i.tag, uri: uri })
      })
    }
  })
  let links = c.links && c.links.length>0? c.links : <Link[]>[]
  links = [...newLinks, ...links.filter(l => l.type != 'local')]

  return links
}
const delClient = (id: number) => {
  const clientIndex = clients.value.findIndex(c => c.id === id)
  const oldData = createClient(clients.value[clientIndex])

  // Delete stats if exists and will be orphaned
  const tagCounts = clients.value.filter(i => i.name == oldData.name).length
  const sIndex = v2rayStats.value.users.findIndex(i => i == oldData.name)
  if (tagCounts == 1 && sIndex != -1){
    v2rayStats.value.users.splice(sIndex,1)
  }

  clients.value.splice(clientIndex,1)
  buildInboundsUsers(oldData.inbounds)
  if (id>0) Data().delClient(id)
  delOverlay.value[clientIndex] = false
}

const qrcode = ref({
  visible: false,
  index: 0,
})

const showQrCode = (id: number) => {
  const clientIndex = clients.value.findIndex(c => c.id === id)
  qrcode.value.index = clientIndex
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

var openedGroups = ref(<string[]>[""])

const toggleGroupOpen = (g: string) => {
  const index = openedGroups.value.findIndex(og => og == g)
  index == -1 ? openedGroups.value.push(g) : openedGroups.value.splice(index,1)
}

const doFilter = () => {
  let filteredClients = clients.value.slice()
  if (filterSettings.value.group != '-') {
    filteredClients = filteredClients.filter(c => c.group == filterSettings.value.group)
  }
  if (filterSettings.value.text.length>0) {
    const txt = filterSettings.value.text
    filteredClients = filteredClients.filter(c => c.name.search(txt) != -1 || c.desc.search(txt) != -1)
  }
  switch (filterSettings.value.state) {
    case "disable":
      filteredClients = filteredClients.filter(c => c.enable == false)
      break
    case "expired":
      filteredClients = filteredClients.filter(c => HumanReadable.remainedDays(c.expiry) == null)
      break
    case "online":
      filteredClients = filteredClients.filter(c => Data().onlines?.user?.includes(c.name))
      break
  }
  filterSettings.value.filteredClients = filteredClients
  filterSettings.value.enabled = true
  filterMenu.value = false
}

const clearFilter = () => {
  filterSettings.value = {
    enabled: false,
    state: '',
    group: '-',
    text: '',
    filteredClients: <any[]>[]
  }
  filterMenu.value = false
}
</script>