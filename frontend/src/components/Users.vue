<template>
  <v-card :subtitle="$t('pages.clients')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select v-model="data.model" :items="initUsersModels" @update:model-value="data.values = []" hide-details></v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.model == 'group'">
        <v-select v-model="data.values" multiple chips :items="groupNames" :label="$t('client.group')" hide-details></v-select>
      </v-col>
      <v-col cols="12" sm="8" v-if="data.model == 'client'">
        <v-select v-model="data.values" multiple chips :items="clientNames" :label="$t('pages.clients')" hide-details></v-select>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { i18n } from '@/locales'


export default {
  props: ['data', 'clients'],
  data() {
    return {
      initUsersModels: [
        { title: i18n.global.t('none'), value: 'none' },
        { title: i18n.global.t('all'), value: 'all' },
        { title: i18n.global.t('client.group'), value: 'group' },
        { title: i18n.global.t('pages.clients'), value: 'client' },
      ],
    }
  },
  computed: {
    clientNames() {
      return this.$props.clients.map((c:any) => { return { title: c.name, value: c.id } } )
    },
    groupNames() {
      return Array.from(new Set(this.$props.clients.map((c:any) => c.group)))
    },
  }
}
</script>