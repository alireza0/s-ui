<template>
  <v-card subtitle="Wireguard">
    <v-row>
      <v-col cols="12" sm="8">
        <v-text-field
          v-model="data.private_key"
          :label="$t('types.wg.privKey')"
          append-icon="mdi-key-star"
          @click:append="newKey()"
          hide-details>
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="8">
        <v-text-field
          v-model="public_key"
          readonly
          :label="$t('tls.pubKey')"
          append-icon="mdi-refresh"
          @click:append="getWgPubKey()"
          hide-details>
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="8">
        <v-text-field v-model="address" :label="$t('types.wg.localIp') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('in.port')"
          hide-details
          type="number"
          min=1
          v-model.number="data.listen_port">
        </v-text-field>
      </v-col>
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
    </v-row>
    <v-row>
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
  <v-card v-if="data.peers != undefined">
    <v-card-subtitle>
      {{ $t('types.wg.peers') }}
      <v-chip color="primary" density="compact" variant="elevated" @click="addPeer"><v-icon icon="mdi-plus" /></v-chip>
    </v-card-subtitle>
    <template v-for="(p, index) in data.peers">
      <v-card style="margin-top: 1rem;">
        <v-card-subtitle>
          {{ $t('types.wg.peer') + ' ' + (index+1) }} <v-icon color="error" icon="mdi-delete" @click="delPeer(index)" />
        </v-card-subtitle>
        <Peer :data="p" :ext="data.ext" @refreshPeerKey="$emit('refreshPeerKey', index)" />
      </v-card>
    </template>
  </v-card>
</template>

<script lang="ts">
import Peer from '@/components/WgPeer.vue'

export default {
  props: ['data'],
  emits: ['newWgKey', 'getWgPubKey', 'addPeer', 'delPeer', 'refreshPeerKey'],
  data() {
    return {
      menu: false,
    }
  },
  methods: {
    addPeer() {
      this.$emit('addPeer')
    },
    delPeer(id: number) {
      this.$emit('delPeer', id)
    },
    refreshPeerKey(id: number) {
      this.$emit('refreshPeerKey', id)
    },
    newKey() {
      this.$emit('newWgKey')
    },
    getWgPubKey() {
      const privKey = this.$props.data.private_key
      if (privKey.length == 0) return
      this.$emit('getWgPubKey', privKey)
    },
  },
  computed: {
    optionUdp: {
      get(): boolean { return this.$props.data.udp_timeout != undefined },
      set(v:boolean) { this.$props.data.udp_timeout = v ? "5m" : undefined }
    },
    optionRsrv: {
      get(): boolean { return this.$props.data.reserved != undefined },
      set(v:boolean) { this.$props.data.reserved = v ? [0,0,0] : undefined }
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
    address: {
      get() { return this.$props.data.address?.join(',') },
      set(v:string) { this.$props.data.address = v.length > 0 ? v.split(',') : undefined }
    },
    reserved: {
      get() { return this.$props.data.reserved?.join(',') },
      set(v:string) { 
        if(!v.endsWith(',')) {
          this.$props.data.reserved = v.length > 0 ? v.split(',').map(str => parseInt(str, 10)) : []
        }
      }
    },
    udp_timeout: {
      get() { return this.$props.data.udp_timeout ? parseInt(this.$props.data.udp_timeout.replace('m','')) : 5 },
      set(v:number) { this.$props.data.udp_timeout = v > 0 ? v + 'm' : '5m' }
    },
    public_key: {
      get() { return this.$props.data.ext?.public_key?? '' },
      set(v:string) { this.$props.data.ext.public_key = v }
    }
  },
  components: { Peer }
}
</script>