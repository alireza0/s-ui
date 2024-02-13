<template>
  <v-card subtitle="TUIC">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="Congestion Control"
          :items="congestion_controls"
          v-model="inbound.congestion_control">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="Zero-RTT Handshake" v-model="inbound.zero_rtt_handshake" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Authentication Timeout"
        hide-details
        type="number"
        suffix="s"
        min="1"
        v-model.number="auth_timeout">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Heartbeat"
        hide-details
        type="number"
        suffix="s"
        min="1"
        v-model.number="heartbeat">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { TUIC } from '@/types/inbounds'
export default {
  props: ['inbound'],
  data() {
    return {
      congestion_controls: [
        "cubic","new_reno", "bbr"
      ]
    }
  },
  computed: {
    Inbound(): TUIC {
      return <TUIC> this.$props.inbound
    },
    auth_timeout: {
      get() { return this.Inbound.auth_timeout ? parseInt(this.Inbound.auth_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.inbound.auth_timeout = newValue ? newValue + 's' : '' }
    },
    heartbeat: {
      get() { return this.Inbound.heartbeat ? parseInt(this.Inbound.heartbeat.replace('s','')) : '' },
      set(newValue:number) { this.$props.inbound.heartbeat = newValue ? newValue + 's' : '' }
    }
  }
}
</script>