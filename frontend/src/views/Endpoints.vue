<template>
  <EndpointVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :tags="endpointTags"
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
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>endpoints" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('in.addr') }}</v-col>
            <v-col>
              {{ item.address?.length>0 ? item.address[0] : '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col>
              {{ item.listen_port>0 ? item.listen_port : '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('types.wg.peers') }}</v-col>
            <v-col>
              {{ item.peers.length?? '-'  }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('online') }}</v-col>
            <v-col>
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
                <v-btn color="error" variant="outlined" @click="delEndpoint(item.tag)">{{ $t('yes') }}</v-btn>
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
import EndpointVue from '@/layouts/modals/Endpoint.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Endpoint } from '@/types/endpoints';
import { computed, ref } from 'vue'
import { i18n } from '@/locales';
import { push } from 'notivue';

const endpoints = computed((): Endpoint[] => {
  return <Endpoint[]> Data().endpoints
})

const endpointTags = computed((): any[] => {
  return endpoints.value?.map((o:Endpoint) => o.tag)
})

const onlines = computed(() => {
  return [...Data().onlines.inbound?? [], ...Data().onlines.outbound??[] ]
})

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

let delOverlay = ref(new Array<boolean>)

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(endpoints.value.findLast(o => o.id == id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}
const saveModal = async (data:Endpoint) => {
  // Check duplicate tag
  const oldTag = modal.value.id > 0  ? endpoints.value.findLast(i => i.id == modal.value.id)?.tag : null
  if (data.tag != oldTag && endpointTags.value.includes(data.tag)) {
    push.error({
      message: i18n.global.t('error.dplData') + ": " + i18n.global.t('objects.tag')
    })
    return
  }

  // save data
  const success = await Data().save("endpoints", modal.value.id == 0 ? "new" : "edit", data)
  if (success) modal.value.visible = false
}

const stats = ref({
  visible: false,
  resource: "endpoint",
  tag: "",
})

const delEndpoint = async (tag: string) => {
  const index = endpoints.value.findIndex(i => i.tag == tag)
  const success = await Data().save("endpoints", "del", tag)
  if (success) delOverlay.value[index] = false
}

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
</script>