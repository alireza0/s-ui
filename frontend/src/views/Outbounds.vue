<template>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>outbounds" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle>
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>Server</v-col>
            <v-col dir="ltr">
              {{ (item.server ?? '') + ' ' + (item.server_port ?? '') }}
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed } from 'vue'

const appConfig = Data().config
const outbounds = computed((): any[] => {
  if (!appConfig || !('outbounds' in appConfig) || !Array.isArray(appConfig.outbounds)) {
    return []
  }
  return appConfig.outbounds
})
</script>