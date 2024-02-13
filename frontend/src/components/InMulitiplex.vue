<template>
  <v-card :subtitle="$t('in.multiplex')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="Enable Multiplex" v-model="muxEnable" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="mux.enabled">
        <v-switch color="primary" label="Reject Non-Padded" v-model="mux.padding" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="mux.enabled">
        <v-switch color="primary" label="Enable Brutal" v-model="burtalEnable" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="mux.brutal?.enabled">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Uplink Bandwidth"
        hide-details
        type="number"
        suffix="Mbps"
        v-model.number="up_mbps">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Downlink Bandwidth"
        hide-details
        type="number"
        suffix="Mbps"
        min="0"
        v-model.number="down_mbps">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { iMultiplex } from '@/types/inMultiplex'
export default {
  props: ['inbound'],
  data() {
    return {}
  },
  computed: {
    mux(): iMultiplex {
      return <iMultiplex> this.$props.inbound.multiplex
    },
    muxEnable: {
      get(): boolean { return this.$props.inbound.multiplex ? this.mux.enabled : false },
      set(newValue:boolean) { this.$props.inbound.multiplex = newValue ? { enabled: newValue } : {} }
    },
    burtalEnable: {
      get(): boolean { return this.mux.brutal ? this.mux.brutal.enabled : false },
      set(newValue:boolean) { this.mux.brutal = { enabled: newValue, up_mbps: 100, down_mbps: 100 } }
    },
    down_mbps: {
      get() { return this.mux.brutal && this.mux.brutal.down_mbps ? this.mux.brutal.down_mbps : 0 },
      set(newValue:any) { 
        if (this.mux.brutal){
          this.mux.brutal.down_mbps = newValue.length != 0 ? newValue : 0
        }
      }
    },
    up_mbps: {
      get() { return this.mux.brutal && this.mux.brutal.up_mbps ? this.mux.brutal.up_mbps : 0 },
      set(newValue:any) {
        if (this.mux.brutal){
          this.mux.brutal.up_mbps = newValue.length != 0 ? newValue : 0
        }
      }
    },
  }
}
</script>