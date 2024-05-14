<template>
    <v-card :subtitle="$t('objects.transport')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" :label="$t('transport.enable')" v-model="tpEnable" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="tpEnable">
        <v-select
          hide-details
          :label="$t('type')"
          :items="Object.keys(trspTypes).map((key,index) => ({title: key, value: Object.values(trspTypes)[index]}))"
          v-model="transportType">
        </v-select>
      </v-col>
    </v-row>
    <Http v-if="Transport.type == trspTypes.HTTP" :transport="Transport" />
    <WebSocket v-if="Transport.type == trspTypes.WebSocket" :transport="Transport" />
    <GRPC v-if="Transport.type == trspTypes.gRPC" :transport="Transport" />
    <HttpUpgrade v-if="Transport.type == trspTypes.HTTPUpgrade" :transport="Transport" />
  </v-card>
</template>

<script lang="ts">
import { TrspTypes, Transport } from '@/types/transport'
import Http from './transports/Http.vue'
import WebSocket from './transports/WebSocket.vue'
import GRPC from './transports/gRPC.vue'
import HttpUpgrade from './transports/HttpUpgrade.vue'
export default {
  props: ['data'],
  data() {
    return {
      trspTypes: TrspTypes
    }
  },
  computed: {
    Transport() {
      return <Transport>this.$props.data.transport
    },
    tpEnable: {
      get() { return Object.hasOwn(this.$props.data.transport, 'type') },
      set(newValue: boolean) { this.$props.data.transport = newValue ? { type: 'http' } : {} }
    },
    transportType: {
      get() { return this.Transport.type },
      set(newValue: string) { this.$props.data.transport = { type: newValue } }
    }
  },
  components: { Http, WebSocket, GRPC, HttpUpgrade }
}
</script>