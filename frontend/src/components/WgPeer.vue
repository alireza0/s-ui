<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('out.addr')"
      hide-details
      v-model="address">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('out.port')"
      type="number"
      min="0"
      hide-details
      v-model.number="port">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      label="KeepAlive"
      type="number"
      min="0"
      :suffix="$t('date.s')"
      hide-details
      v-model.number="keepAlive">
      </v-text-field>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6">
      <v-text-field v-model="data.public_key" :label="$t('types.wg.pubKey')" hide-details></v-text-field>
    </v-col>
    <v-col cols="12" sm="6">
      <v-text-field v-model="data.pre_shared_key" :label="$t('types.wg.psk')" hide-details></v-text-field>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6">
      <v-text-field v-model="allowed_ips" :label="$t('types.wg.allowedIp') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
    </v-col>
    <v-col cols="12" sm="6">
      <v-text-field v-model="reserved" :label="'Reserved ' + $t('commaSeparated')" hide-details></v-text-field>
    </v-col>
  </v-row>
</template>

<script lang="ts">
import { KeepAlive } from 'vue';

export default {
  props: ['data'],
  data() {
    return {}
  },
  computed: {
    allowed_ips: {
      get() { return this.$props.data.allowed_ips?.join(',') },
      set(v:string) { this.$props.data.allowed_ips = v.length > 0 ? v.split(',') : undefined }
    },
    reserved: {
      get() { return this.$props.data.reserved?.join(',') },
      set(v:string) {
        if(!v.endsWith(',')) {
          this.$props.data.reserved = v.length > 0 ? v.split(',').map(str => parseInt(str, 10)) : undefined
        }
      }
    },
    address: {
      get() { return this.$props.data.address },
      set(v:string) { this.$props.data.address = v.length > 0 ? v : undefined }
    },
    port: {
      get() { return this.$props.data.port },
      set(v:number) { this.$props.data.port = v > 0 ? v : undefined }
    },
    keepAlive: {
      get() { return this.$props.data.persistent_keepalive_interval?? 0 },
      set(v:number) { this.$props.data.persistent_keepalive_interval = v > 0 ? v : undefined }
    }
  }
}
</script>