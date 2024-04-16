<template>
  <v-card subtitle="Hysteria2">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Masquerade"
        hide-details
        v-model="hysteria2.masquerade"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="hysteria2.ignore_client_bandwidth" color="primary" label="Ignore Client Bandwidth" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="!hysteria2.ignore_client_bandwidth">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Uplink Limit"
        hide-details
        type="number"
        suffix="Mbps"
        v-model.number="up_mbps">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Downlink Limit"
        hide-details
        type="number"
        suffix="Mbps"
        min="0"
        v-model.number="down_mbps">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="hysteria2.obfs">
      <v-col cols="12" sm="6" md="4">
       <v-text-field
        label="obfs Password"
        hide-details
        v-model="hysteria2.obfs.password">
        </v-text-field>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details>Options</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionObfs" color="primary" label="Obfs" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
import { Hysteria2, createInbound } from '@/types/inbounds'

export default {
  props: ['inbound'],
  data() {
    return {
      menu: false,
      hysteria2: <Hysteria2> createInbound("hysteria2",{ "tag": "" }),
    }
  },
  computed: {
    down_mbps: {
      get() { return this.hysteria2.down_mbps ? this.hysteria2.down_mbps : 0 },
      set(newValue:number) { this.hysteria2.down_mbps = newValue?? undefined }
    },
    up_mbps: {
      get() { return this.hysteria2.up_mbps ? this.hysteria2.up_mbps : 0 },
      set(newValue:number) { this.hysteria2.up_mbps = newValue?? undefined }
    },
    optionObfs: {
      get(): boolean { return this.hysteria2.obfs != undefined },
      set(v:boolean) { this.$props.inbound.obfs = v ? { type: "salamander", password: ""} : undefined }
    }
  },
  mounted() {
    this.hysteria2 = <Hysteria2> this.$props.inbound
  }
}
</script>