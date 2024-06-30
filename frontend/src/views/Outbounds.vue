<template>
  <OutboundVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :stats="modal.stats"
    :data="modal.data"
    :tags="outboundTags"
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
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>outbounds" :key="item.tag">
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
              {{ item.server?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col dir="ltr">
              {{ item.server_port?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col dir="ltr">
              {{ Object.hasOwn(item,'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-'  }}
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
                <v-btn color="error" variant="outlined" @click="delOutbound(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-chart-line" @click="showStats(item.tag)" v-if="v2rayStats.outbounds.includes(item.tag)">
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
import OutboundVue from '@/layouts/modals/Outbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Config, V2rayApiStats } from '@/types/config';
import { Outbound } from '@/types/outbounds';
import { computed, ref } from 'vue'
import { i18n } from '@/locales';
import { push } from 'notivue';

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const outbounds = computed((): Outbound[] => {
  return <Outbound[]> appConfig.value.outbounds
})

const outboundTags = computed((): string[] => {
  return outbounds.value?.map((o:Outbound) => o.tag)
})

const onlines = computed(() => {
  return Data().onlines.outbound ? outbounds.value.map(i => Data().onlines.outbound.includes(i.tag)) : []
})

const v2rayStats = computed((): V2rayApiStats => {
  return <V2rayApiStats> appConfig.value.experimental?.v2ray_api.stats
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
  modal.value.data = id == -1 ? '' : JSON.stringify(outbounds.value[id])
  modal.value.stats = id == -1 ? false : v2rayStats.value.outbounds.includes(outbounds.value[id].tag)
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}
const saveModal = (data:Outbound, stats: boolean) => {
  // Check duplicate tag
  const oldTag = modal.value.id != -1 ? outbounds.value[modal.value.id].tag : null
  if (data.tag != oldTag && outboundTags.value.includes(data.tag)) {
    push.error({
      message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
    })
    return
  }
  // New or Edit
  if (modal.value.id == -1) {
    outbounds.value.push(data)
    if (stats && data.tag.length>0) {
      v2rayStats.value.outbounds.push(data.tag)
    }
  } else {
    const sIndex = v2rayStats.value.outbounds.findIndex(i => i == data.tag) // Find if new tag exists

    if (stats) {
      // Add if dos not exist
      if (data.tag.length>0 && sIndex == -1) v2rayStats.value.outbounds.push(data.tag)
    } else {
      // Delete if exists
      if (sIndex != -1) v2rayStats.value.outbounds.splice(sIndex,1)
    }

    outbounds.value[modal.value.id] = data
  }
  modal.value.visible = false
}

const stats = ref({
  visible: false,
  resource: "outbound",
  tag: "",
})

const delOutbound = (index: number) => {
  const inb = outbounds.value[index]
  outbounds.value.splice(index,1)
  const tag = inb.tag

  // Delete stats if exists and will be orphaned
  const tagCounts = outbounds.value.filter(i => i.tag == inb.tag).length
  const sIndex = v2rayStats.value.outbounds.findIndex(i => i == inb.tag)
  if (tagCounts == 1 && sIndex != -1){
    v2rayStats.value.outbounds.splice(sIndex,1)
  }
  if (index < Data().oldData.config.outbounds.length){
    Data().delOutbound(index)
  }
  delOverlay.value[index] = false
}

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>