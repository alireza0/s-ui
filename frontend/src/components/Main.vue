<template>
  <LogVue v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
  <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
  <v-container class="fill-height" :loading="loading">
    <v-responsive :class="reloadItems.length>0 ? 'fill-height text-center' : 'align-center'" >
      <v-row class="d-flex align-center justify-center">
        <v-col cols="auto">
          <v-img src="@/assets/logo.svg" :width="reloadItems.length>0 ? 100 : 200"></v-img>
        </v-col>
      </v-row>
      <v-row class="d-flex align-center justify-center">
        <v-col cols="auto">
          <v-dialog v-model="menu" :close-on-content-click="false" transition="scale-transition" max-width="800">
            <template v-slot:activator="{ props }">
              <v-btn v-bind="props" hide-details variant="tonal">{{ $t('main.tiles') }} <v-icon icon="mdi-star-plus" /></v-btn>
            </template>
            <v-card rounded="xl">
              <v-card-title>
                <v-row>
                  <v-col>
                    {{ $t('main.tiles') }}
                  </v-col>
                  <v-spacer></v-spacer>
                  <v-col cols="auto"><v-icon icon="mdi-close" @click="menu = false"></v-icon></v-col>
                </v-row>
              </v-card-title>
              <v-divider></v-divider>
              <v-row v-for="items in menuItems" no-gutters>
                <v-col cols="12">
                  <v-card :subtitle="items.title" variant="flat">
                    <v-card-text>
                      <v-row no-gutters>
                        <v-col cols="12" md="6" lg="3" v-for="item in items.value">
                          <v-switch
                          density="compact"
                          v-model="reloadItems"
                          :value="item.value"
                          color="primary"
                          :label="item.title"
                          hide-details></v-switch>
                        </v-col>
                      </v-row>
                    </v-card-text>
                  </v-card>
                </v-col>
              </v-row>
            </v-card>
          </v-dialog>
          <v-btn variant="tonal" hide-details style="margin-inline-start: 10px;" @click="backupModal.visible = true">{{ $t('main.backup.title') }} <v-icon icon="mdi-backup-restore" /></v-btn>
          <v-btn variant="tonal" hide-details style="margin-inline-start: 10px;" @click="logModal.visible = true">{{ $t('basic.log.title') }} <v-icon icon="mdi-list-box-outline" /></v-btn>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6" md="3" v-for="i in reloadItems" :key="i">
          <v-card class="rounded-lg" variant="outlined" height="210px"
                  :title="menuItems.flatMap(cat => cat.value).find(m => m.value == i)?.title">
            <v-card-text style="padding: 0 16px;" align="center" justify="center">
              <Gauge :tilesData="tilesData" :type="i" v-if="i.charAt(0) == 'g'" />
              <History :tilesData="tilesData" :type="i" v-if="i.charAt(0) == 'h'" />
              <template v-if="i == 'i-sys'">
                <v-row>
                  <v-col cols="3">{{ $t('main.info.host') }}</v-col>
                  <v-col cols="9" style="text-wrap: nowrap; overflow: hidden">{{ tilesData.sys?.hostName }}</v-col>
                  <v-col cols="3">{{ $t('main.info.cpu') }}</v-col>
                  <v-col cols="9">
                    <v-chip density="compact" variant="flat">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        {{ tilesData.sys?.cpuType }}
                      </v-tooltip>
                     {{ tilesData.sys?.cpuCount }} {{ $t('main.info.core') }}
                    </v-chip>
                  </v-col>
                  <v-col cols="3">IP</v-col>
                  <v-col cols="9">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sys?.ipv4?.length>0">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        <span v-html="tilesData.sys?.ipv4?.join('<br />')"></span>
                      </v-tooltip>
                      IPv4
                    </v-chip>
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sys?.ipv6?.length>0">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        <span v-html="tilesData.sys?.ipv6?.join('<br />')"></span>
                      </v-tooltip>
                      IPv6
                    </v-chip>
                  </v-col>
                  <v-col cols="3">S-UI</v-col>
                  <v-col cols="9">
                    <v-chip density="compact" color="blue">
                      v{{ tilesData.sys?.appVersion }}
                    </v-chip>
                  </v-col>
                  <v-col cols="3">{{ $t('main.info.uptime') }}</v-col>
                  <v-col cols="9">{{ HumanReadable.formatSecond(tilesData.uptime) }}</v-col>
                </v-row>
              </template>
              <template v-if="i == 'i-sbd'">
                <v-row>
                  <v-col cols="4">{{ $t('main.info.running') }}</v-col>
                  <v-col cols="8">
                    <v-chip density="compact" color="success" variant="flat" v-if="tilesData.sbd?.running">{{ $t('yes') }}</v-chip> 
                    <v-chip density="compact" color="error" variant="flat" v-else>{{ $t('no') }}</v-chip>
                    <v-chip density="compact" color="transparent" v-if="tilesData.sbd?.running && !loading" style="cursor: pointer;" @click="restartSingbox()">
                      <v-tooltip activator="parent" location="top">
                        {{ $t('actions.restartSb') }}
                      </v-tooltip>
                      <v-icon icon="mdi-restart" color="warning" />
                    </v-chip>
                  </v-col>
                  <v-col cols="4">{{ $t('main.info.memory') }}</v-col>
                  <v-col cols="8">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sbd?.stats?.Alloc">
                      {{ HumanReadable.sizeFormat(tilesData.sbd?.stats?.Alloc) }}
                    </v-chip> 
                  </v-col>
                  <v-col cols="4">{{ $t('main.info.threads') }}</v-col>
                  <v-col cols="8">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sbd?.stats?.NumGoroutine">
                      {{ tilesData.sbd?.stats?.NumGoroutine }}
                    </v-chip>
                  </v-col>
                  <v-col cols="4">{{ $t('main.info.uptime') }}</v-col>
                  <v-col cols="8">{{ HumanReadable.formatSecond(tilesData.sbd?.stats?.Uptime) }}</v-col>
                  <v-col cols="4">{{ $t('online') }}</v-col>
                  <v-col cols="8">
                    <template v-if="tilesData.sbd?.running">
                      <v-chip density="compact" color="primary" variant="flat" v-if="Data().onlines.user">
                        <v-tooltip activator="parent" location="top" :text="$t('pages.clients')" />
                        {{ Data().onlines.user?.length }}
                      </v-chip>
                      <v-chip density="compact" color="success" variant="flat" v-if="Data().onlines.inbound">
                        <v-tooltip activator="parent" location="top" :text="$t('pages.inbounds')" />
                        {{ Data().onlines.inbound?.length }}
                      </v-chip>
                      <v-chip density="compact" color="info" variant="flat" v-if="Data().onlines.outbound">
                        <v-tooltip activator="parent" location="top" :text="$t('pages.outbounds')" />
                        {{ Data().onlines.outbound?.length }}
                      </v-chip>
                    </template>
                  </v-col>
                </v-row>
              </template>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </v-responsive>
  </v-container>
</template>

<script lang="ts" setup>
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import Gauge from '@/components/tiles/Gauge.vue'
import History from '@/components/tiles/History.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { i18n } from '@/locales'
import LogVue from '@/layouts/modals/Logs.vue'
import Backup from '@/layouts/modals/Backup.vue'

const loading = ref(false)
const menu = ref(false)
const menuItems = [
  { title: i18n.global.t('main.gauges'), value: [
    { title: i18n.global.t('main.gauge.cpu'), value: "g-cpu" },
    { title: i18n.global.t('main.gauge.mem'), value: "g-mem" },
    { title: i18n.global.t('main.gauge.dsk'), value: "g-dsk" },
    { title: i18n.global.t('main.gauge.swp'), value: "g-swp" },
    ]
  },
  { title: i18n.global.t('main.charts'), value: [
    { title: i18n.global.t('main.chart.cpu'), value: "h-cpu" },
    { title: i18n.global.t('main.chart.mem'), value: "h-mem" },
    { title: i18n.global.t('main.chart.net'), value: "h-net" },
    { title: i18n.global.t('main.chart.pnet'), value: "hp-net" },
    { title: i18n.global.t('main.chart.dio'), value: "h-dio" },
    ]
  },
  { title: i18n.global.t('main.infos'), value: [
    { title: i18n.global.t('main.info.sys'), value: "i-sys" },
    { title: i18n.global.t('main.info.sbd'), value: "i-sbd" },
    ]
  },
]

const tilesData = ref(<any>{})

const reloadItems = computed({
  get() { return Data().reloadItems },
  set(v:string[]) {
    if (Data().reloadItems.length == 0 && v.length>0) startTimer()
    if (Data().reloadItems.length > 0 && v.length == 0) stopTimer()
    Data().reloadItems = v
    v.length>0 ? localStorage.setItem("reloadItems",v.join(',')) : localStorage.removeItem("reloadItems")
  }
})

const reloadData = async () => {
  const request = [...new Set(reloadItems.value.map(r => r.split('-')[1]))]
  const data = await HttpUtils.get('api/status',{ r: request.join(',')})
  if (data.success) {
    tilesData.value = data.obj
  }
}

let intervalId: NodeJS.Timeout | null = null

const startTimer = () => {
  intervalId = setInterval(() => {
    reloadData()
  }, 2000)
}

const stopTimer = () => {
  if (intervalId) {
    clearInterval(intervalId)
    intervalId = null
  }
}

onMounted(() => {
  if (Data().reloadItems.length != 0) {
    reloadData()
    startTimer()
  }
})

onBeforeUnmount(() => {
  stopTimer()
})

const logModal = ref({ visible: false })

const backupModal = ref({ visible: false })

const restartSingbox = async () => {
  loading.value = true
  await HttpUtils.post('api/restartSb',{})
  loading.value = false
}
</script>
