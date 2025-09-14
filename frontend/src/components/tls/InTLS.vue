<template>
  <v-card :subtitle="$t('objects.tls')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :label="$t('template')"
          :items="tlsItems"
          v-model="inbound.tls_id">
        </v-select>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { i18n } from '@/locales'
export default {
  props: ['inbound', 'tlsConfigs'],
  computed: {
    tlsItems(): any[] {
      return [ { title: i18n.global.t('none'), value: 0 }, ...this.$props.tlsConfigs?.map((t:any) => { return { title: t.name, value: t.id } } )]
    }
  }
}
</script>