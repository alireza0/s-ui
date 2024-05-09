<template>
  <v-card subtitle="TUIC">
    <v-row v-if="direction == 'out'">
      <v-col cols="12" sm="6">
        <v-text-field v-model="data.uuid" label="UUID" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.password" label="Password" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="UDP Relay Mode"
          :items="['native', 'quic']"
          clearable
          @click:clear="delete data.udp_relay_mode"
          v-model="data.udp_relay_mode">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="UDP Over Stream" v-model="data.udp_over_stream" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="Congestion Control"
          :items="congestion_controls"
          v-model="data.congestion_control">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" label="Zero-RTT Handshake" v-model="data.zero_rtt_handshake" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'in'">
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
import Network from '@/components/Network.vue'

export default {
  props: ['direction', 'data'],
  data() {
    return {
      congestion_controls: [
        "cubic","new_reno", "bbr"
      ]
    }
  },
  computed: {
    auth_timeout: {
      get() { return this.$props.data.auth_timeout ? parseInt(this.$props.data.auth_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.data.auth_timeout = newValue ? newValue + 's' : '' }
    },
    heartbeat: {
      get() { return this.$props.data.heartbeat ? parseInt(this.$props.data.heartbeat.replace('s','')) : '' },
      set(newValue:number) { this.$props.data.heartbeat = newValue ? newValue + 's' : '' }
    }
  },
  components: { Network }
}
</script>