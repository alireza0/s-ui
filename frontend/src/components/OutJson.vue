<template>
  <v-card :subtitle="type">
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="type == inTypes.SOCKS">
        <v-select
          hide-details
          :items="['4','4a','5']"
          :label="$t('version')"
          v-model="inData.outJson.version">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="needNetwork">
        <Network :data="inData.outJson" />
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="needUot">
        <UoT :data="inData.outJson" />
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="type == inTypes.HTTP">
        <v-text-field
        :label="$t('transport.path')"
        hide-details
        v-model="inData.outJson.path">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="type == inTypes.VMess || type == inTypes.VLESS">
        <v-select
          hide-details
          :label="$t('types.vless.udpEnc')"
          :items="['none','packetaddr','xudp']"
          v-model="packet_encoding">
        </v-select>
      </v-col>
      <template v-if="type == inTypes.VMess">
        <v-col cols="12" sm="6" md="4">
          <v-select
            hide-details
            :label="$t('types.vmess.security')"
            :items="vmessSecurities"
            v-model="inData.outJson.security">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch v-model="inData.outJson.global_padding" color="primary" :label="$t('types.vmess.globalPadding')" hide-details></v-switch>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch v-model="inData.outJson.authenticated_length" color="primary" :label="$t('types.vmess.authLen')" hide-details></v-switch>
        </v-col>
      </template>
      <v-col cols="12" sm="6" md="4" v-if="type == inTypes.Hysteria">
        <v-text-field
        label="Recv window"
        hide-details
        type="number"
        min="0"
        v-model.number="inData.outJson.recv_window">
        </v-text-field>
      </v-col>
      <template v-if="type == inTypes.TUIC">
        <v-col cols="12" sm="6" md="4">
          <v-select
            hide-details
            label="UDP Relay Mode"
            :items="['native', 'quic']"
            clearable
            @click:clear="delete inData.outJson.udp_relay_mode"
            v-model="inData.outJson.udp_relay_mode">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" label="UDP Over Stream" v-model="inData.outJson.udp_over_stream" hide-details></v-switch>
        </v-col>
      </template>
    </v-row>
    <Headers :data="inData.outJson" v-if="type == inTypes.HTTP" />
  </v-card>
</template>

<script lang="ts">
import { InTypes } from '@/types/inbounds'
import Network from './Network.vue'
import UoT from './UoT.vue'
import Headers from './Headers.vue'

export default {
  props: ['inData', 'type'],
  data() {
    return {
      inTypes: InTypes,
      vmessSecurities: [
        "auto",
        "none",
        "zero",
        "aes-128-gcm",
        "aes-128-ctr",
        "chacha20-poly1305",
      ],
      haveNetwork: [
        InTypes.SOCKS,
        InTypes.Shadowsocks,
        InTypes.VMess,
        InTypes.Trojan,
        InTypes.Hysteria,
        InTypes.VLESS,
        InTypes.TUIC,
        InTypes.Hysteria2,
      ],
      havUoT: [
        InTypes.SOCKS,
        InTypes.Shadowsocks,
      ],
    }
  },
  computed: {
    needNetwork():boolean { return this.haveNetwork.includes(this.$props.type) },
    needUot():boolean { return this.havUoT.includes(this.$props.type) },
    packet_encoding: {
      get() { return this.$props.inData.outJson.packet_encoding != undefined ? this.$props.inData.outJson.packet_encoding : 'none'; },
      set(v:string) { this.$props.inData.outJson.packet_encoding = v != "none" ? v : undefined }
    },
  },
  components: { Network, UoT, Headers }
}
</script>