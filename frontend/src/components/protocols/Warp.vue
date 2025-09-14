<template>
  <v-card subtitle="Warp">
    <template v-if="data.id>0">
      <table dir="ltr" width="100%">
        <tbody>
          <tr>
            <td>Device ID</td>
            <td>{{ data.ext.device_id }}</td>
          </tr>
          <tr>
            <td>Access Token</td>
            <td>{{ data.ext.access_token }}</td>
          </tr>
          <tr>
            <td>{{ $t('types.wg.privKey') }}</td>
            <td>{{ data.private_key }}</td>
          </tr>
          <tr>
            <td>{{ $t('types.wg.localIp') }}</td>
            <td>{{ data.address.join(',') }}</td>
          </tr>
          <tr>
            <td colspan="2">
              <v-text-field
                v-model="data.ext.license_key"
                label="License Key"
                hide-details>
              </v-text-field>
            </td>
          </tr>
        </tbody>
      </table>
      <v-card :subtitle="$t('types.wg.peer')">
        <v-row>
          <v-col cols="12" sm="6">
            <v-text-field
              :label="$t('out.addr')"
              hide-details
              v-model="data.peers[0].address">
            </v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              :label="$t('out.port')"
              hide-details
              type="number"
              min=1
              v-model.number="data.peers[0].port">
            </v-text-field>
          </v-col>
        </v-row>
        <table dir="ltr" width="100%">
          <tbody>
            <tr>
              <td>{{ $t('types.wg.pubKey') }}</td>
              <td>{{ data.peers[0].public_key }}</td>
            </tr>
            <tr>
              <td>{{ $t('types.wg.allowedIp') }}</td>
              <td>{{ data.peers[0].allowed_ips.join(',') }}</td>
            </tr>
            <tr>
              <td>Reserved</td>
              <td>[{{ data.peers[0].reserved.join(',') }}]</td>
            </tr>
          </tbody>
        </table>
      </v-card>
    </template>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="data.udp_timeout != undefined">
        <v-text-field
          label="UDP Timeout"
          hide-details
          type="number"
          min=0
          :suffix="$t('date.m')"
          v-model.number="udp_timeout">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.workers != undefined">
        <v-text-field
        :label="$t('types.wg.worker')"
          hide-details
          type="number"
          min=1
          v-model.number="data.workers">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.mtu != undefined">
        <v-text-field
          label="MTU"
          hide-details
          type="number"
          min=0
          v-model.number="data.mtu">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.system" color="primary" :label="$t('types.wg.sysIf')" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.system">
        <v-text-field
          :label="$t('types.wg.ifName')"
          hide-details
          v-model="ifName">
        </v-text-field>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.wg.options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionUdp" color="primary" label="UDP Timeout" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionWorker" color="primary" :label="$t('types.wg.worker')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionMtu" color="primary" label="MTU" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
  data() {
    return {
      menu: false,
    }
  },
  methods: {
  },
  computed: {
    optionUdp: {
      get(): boolean { return this.$props.data.udp_timeout != undefined },
      set(v:boolean) { this.$props.data.udp_timeout = v ? "5m" : undefined }
    },
    optionWorker: {
      get(): boolean { return this.$props.data.workers != undefined },
      set(v:boolean) { this.$props.data.workers = v ? 2 : undefined }
    },
    optionMtu: {
      get(): boolean { return this.$props.data.mtu != undefined },
      set(v:boolean) { this.$props.data.mtu = v ? 1408 : undefined }
    },
    ifName: {
      get() { return this.$props.data.name?? '' },
      set(v:string) { this.$props.data.name = v.length > 0 ? v : undefined }
    },
    udp_timeout: {
      get() { return this.$props.data.udp_timeout ? parseInt(this.$props.data.udp_timeout.replace('m','')) : 5 },
      set(v:number) { this.$props.data.udp_timeout = v > 0 ? v + 'm' : '5m' }
    }
  }
}
</script>