<template>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>rules" :key="item.name">
      <v-card rounded="xl" elevation="5" min-width="200" :title="index">
        <v-card-text>
          <v-row>
            <v-col>Type</v-col>
            <v-col dir="ltr">
              {{ item.type }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Mode</v-col>
            <v-col dir="ltr">
              {{ item.mode }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.outbound') }}</v-col>
            <v-col dir="ltr">
              {{ item.outbound }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.rules') }}</v-col>
            <v-col dir="ltr">
              {{ item.rules ? item.rules.length : 0 }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Invert</v-col>
            <v-col dir="ltr">
              {{ item.invert }}
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref } from 'vue'

const appConfig = Data().config

const route = computed((): any => {
  if (!appConfig || !('route' in appConfig)) {
    return []
  }
  return appConfig.route
})

const rules = computed((): any[] => {
  const data = route.value
  if (!route || !('rules' in data) || !Array.isArray(data.rules)){
    return []
  }
  return data.rules
})
</script>