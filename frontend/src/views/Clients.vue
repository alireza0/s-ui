
<template>
  <ClientModal 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :groups="groups"
    :inboundTags="inboundTags"
    @close="closeModal"
  />
  <ClientBulk 
    v-model="addBulkModal"
    :visible="addBulkModal"
    :groups="groups"
    :inboundTags="inboundTags"
    @close="closeBulk"
  />
  <QrCode
    v-model="qrcode.visible"
    :visible="qrcode.visible"
    :id="qrcode.id"
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
      <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
    </v-col>
    <v-col cols="auto">
      <v-menu v-model="actionMenu" :close-on-content-click="false" location="bottom center">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="text" icon>
            <v-icon icon="mdi-tools" color="primary" />
          </v-btn>
        </template>
        <v-list density="compact" nav>
          <v-list-item link @click="addBulk">
            <template v-slot:prepend>
              <v-icon icon="mdi-account-multiple-plus"></v-icon>
            </template>
            <v-list-item-title v-text="$t('actions.addbulk')"></v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
    </v-col>
    <v-col cols="auto">
      <v-menu v-model="filterMenu" :close-on-content-click="false" location="bottom center">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="text" icon>
            <v-icon :icon="filterSettings.enabled ? 'mdi-filter-check-outline' : 'mdi-filter-menu-outline'" :color="filterSettings.enabled ? 'primary' : ''" />
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
  </v-row>
  <v-row>
    <v-col cols="12">
      <v-data-table
        :headers="headers"
        :items="filterSettings.enabled ? filterSettings.filteredClients : clients"
        :hide-default-footer="filterSettings.enabled ? filterSettings.filteredClients.length<=10 : clients.length<=10"
        :items-per-page="itemPerPage"
        @update:items-per-page="setItemPerPage($event)"
        hide-no-data
        fixed-header
        item-value="name"
        :mobile="smAndDown"
        mobile-breakpoint="sm"
        width="100%"
        class="elevation-3 rounded"
        >
        <template v-slot:item.inbounds="{ item }">
          <span>
          <v-tooltip activator="parent" dir="ltr" location="start" v-if="item.inbounds != ''">
            <span v-for="i in item.inbounds">{{ inbounds.find(inb => inb.id == i)?.tag }}<br /></span>
          </v-tooltip>
          {{ item.inbounds?.length }}
          </span>
        </template>
        <template v-slot:item.volume="{ item }">
          <div class="text-start" v-tooltip:top="$t('stats.usage') + ': ' + HumanReadable.sizeFormat(item.up + item.down)">
            <v-chip
              size="small"
              :color="(item.volume>0 && item.volume<=(item.up + item.down))? 'error': ''"
              label
            >{{ item.volume == 0 ? $t('unlimited') : HumanReadable.sizeFormat(item.volume) }}</v-chip>
          </div>
          <v-progress-linear
            :model-value="percent(item)"
            :color="percentColor(item)"
            v-if="item.volume>0"
            bottom
          >
          </v-progress-linear>
        </template>
        <template v-slot:item.expiry="{ item }">
          <div class="text-start">
            <v-chip
              size="small"
              :color="(item.expiry>0 && item.expiry<=Date.now()/1000)? 'error': ''"
              label
            >{{ HumanReadable.remainedDays(item.expiry) }}</v-chip>
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
        <v-icon icon="mdi-chart-line" @click="showStats(item.name)" v-if="Data().enableTraffic">
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
import ClientBulk from '@/layouts/modals/ClientBulk.vue'
import QrCode from '@/layouts/modals/QrCode.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Client } from '@/types/clients'
import { computed, ref } from 'vue'
import { HumanReadable } from '@/plugins/utils'
import { i18n } from '@/locales'
import { useDisplay } from 'vuetify'

const { smAndDown } = useDisplay()

const clients = computed((): any[] => {
  return Data().clients
})

const isOnline = (cname: string) => computed(() => {
  return Data().onlines?.user ? Data().onlines.user.includes(cname) : false
})

const inbounds = computed((): any[] => {
  return Data().inbounds?? []
})

const inboundTags = computed((): any[] => {
  if (!inbounds.value) return []
  return inbounds.value?.filter(i => i.tag != "" && i.users).map(i => { return { title: i.tag, value: i.id } })
})

const groups = computed((): string[] => {
  if (!clients.value) return []
  if (filterSettings?.value.enabled) return Array.from(new Set(filterSettings.value.filteredClients?.map(c => c.group)))
  return Array.from(new Set(clients.value?.map(c => c.group)))
})

const actionMenu = ref(false)
const filterMenu = ref(false)
const filterSettings = ref({
  enabled: false,
  state: '',
  group: '-',
  text: '',
  filteredClients: <any[]>[]
})

const filterItems = [
  { title: i18n.global.t('none'), value: '' },
  { title: i18n.global.t('disable'), value: 'disable' },
  { title: i18n.global.t('date.expired'), value: 'expired' },
  { title: i18n.global.t('online'), value: 'online' },
]

const headers = [
  { title: i18n.global.t('client.name'), key: 'name' },
  { title: i18n.global.t('client.desc'), key: 'desc' },
  { title: i18n.global.t('client.group'), key: 'group' },
  { title: i18n.global.t('pages.inbounds'), key: 'inbounds', width: 10 },
  { title: i18n.global.t('actions.action'), key: 'actions', sortable: false },
  { title: i18n.global.t('stats.volume'), key: 'volume' },
  { title: i18n.global.t('date.expiry'), key: 'expiry' },
  { title: i18n.global.t('online'), key: 'online' },
  { key: 'data-table-group', width: 0 },
]

const itemPerPage = ref(localStorage.getItem('items-per-page') || '10')

const setItemPerPage = (items: number) => {
  itemPerPage.value = items.toString()
  localStorage.setItem('items-per-page', items.toString())
}

const modal = ref({
  visible: false,
  id: 0,
})

const delOverlay = ref(new Array<boolean>(clients.value.length).fill(false))

const showModal = async (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}

const delClient = async (id: number) => {
  const index = clients.value.findIndex(c => c.id === id)
  const success = await Data().save("clients", "del", id)
  if (success) delOverlay.value[index] = false
}

const qrcode = ref({
  visible: false,
  id: 0,
})

const showQrCode = (id: number) => {
  qrcode.value.id = id
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
      filteredClients = filteredClients.filter(c => c.expiry > 0 && c.expiry < (Date.now()/1000) )
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

const addBulkModal = ref(false)

const addBulk = () => {
  addBulkModal.value = true
  actionMenu.value = false
}

const closeBulk = () => {
  addBulkModal.value = false
}

const percent = (c: Client) => { return c.volume>0 ? Math.round((c.up+c.down) *100 / c.volume) : 0 }
const percentColor = (c: Client) => { return (c.up+c.down) >= c.volume ? 'error' : percent(c)>90 ? 'warning' : 'success' }

</script>