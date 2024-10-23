<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.grpcServiceName')"
      hide-details
      v-model="transport.service_name">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-switch
        color="primary"
        v-model="transport.permit_without_stream"
        :label="$t('transport.grpcPws')"
        hide-details>
      </v-switch>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.idleTimeout')"
      hide-details
      type="number"
      suffix="s"
      min="1"
      v-model.number="idle_timeout">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.pingTimeout')"
      hide-details
      type="number"
      suffix="s"
      min="1"
      v-model.number="ping_timeout">
      </v-text-field>
    </v-col>
  </v-row>
</template>

<script lang="ts">
import { gRPC } from '../../types/transport'
export default {
  props: ['transport'],
  data() {
    return {
    }
  },
  computed: {
    GRPC(): gRPC {
      return <gRPC> this.$props.transport?? {}
    },
    idle_timeout: {
      get() { return this.GRPC.idle_timeout ? parseInt(this.GRPC.idle_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.transport.idle_timeout = newValue ? newValue + 's' : '' }
    },
    ping_timeout: {
      get() { return this.GRPC.ping_timeout ? parseInt(this.GRPC.ping_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.transport.ping_timeout = newValue ? newValue + 's' : '' }
    }
  }
}
</script>