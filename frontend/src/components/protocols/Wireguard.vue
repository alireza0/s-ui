<template>
  <v-card subtitle="Wireguard">
    <v-row>
      <v-col cols="12" sm="8">
        <v-text-field v-model="data.private_key" :label="$t('types.wg.privKey')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="8">
        <v-text-field v-model="data.peer_public_key" :label="$t('types.wg.pubKey')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="8" v-if="data.pre_shared_key != undefined">
        <v-text-field v-model="data.pre_shared_key" :label="$t('types.wg.psk')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="8">
        <v-text-field v-model="local_ips" :label="$t('types.wg.localIp') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="data.reserved != undefined">
        <v-text-field v-model="reserved" :label="'Reserved ' + $t('commaSeparated')" hide-details></v-text-field>
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
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.interface_name != undefined">
        <v-text-field
          :label="$t('types.wg.ifName')"
          hide-details
          v-model.number="data.interface_name">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.system_interface" color="primary" :label="$t('types.wg.sysIf')" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.gso" color="primary" :label="$t('types.wg.gso')" hide-details></v-switch>
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
              <v-switch v-model="optionPsk" color="primary" :label="$t('types.wg.psk')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionRsrv" color="primary" label="Reserved" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionWorker" color="primary" :label="$t('types.wg.worker')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionMtu" color="primary" label="MTU" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionInterface" color="primary" :label="$t('types.wg.ifName')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionPeers" color="primary" :label="$t('types.wg.multiPeer')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
  <v-card v-if="data.peers != undefined">
    <v-card-subtitle>
      {{ $t('types.wg.peers') }} <v-icon @click="addPeer" icon="mdi-plus" />
    </v-card-subtitle>
    <template v-for="(p, index) in data.peers">
      <v-card style="margin-top: 1rem;">
        <v-card-subtitle>
          {{ $t('types.wg.peer') + ' ' + (index+1) }} <v-icon icon="mdi-delete" @click="data.peers.splice(index,1)" />
        </v-card-subtitle>
        <Peer :data="p" />
      </v-card>
    </template>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'
import Peer from '@/components/WgPeer.vue'
import { WgPeer } from '@/types/outbounds'

export default {
  props: ['data'],
  data() {
    return {
      menu: false,
    }
  },
  methods: {
    addPeer() { 
      this.$props.data.peers.push({server: '', port: ''})
    }
  },
  computed: {
    optionPsk: {
      get(): boolean { return this.$props.data.pre_shared_key != undefined },
      set(v:boolean) { this.$props.data.pre_shared_key = v ? "" : undefined }
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
    optionInterface: {
      get(): boolean { return this.$props.data.interface_name != undefined },
      set(v:boolean) { this.$props.data.interface_name = v ? "" : undefined }
    },
    optionPeers: {
      get(): boolean { return this.$props.data.peers != undefined },
      set(v:boolean) { this.$props.data.peers = v ? <WgPeer[]>[] : undefined }
    },
    local_ips: {
      get() { return this.$props.data.local_address?.join(',') },
      set(v:string) { this.$props.data.local_address = v.length > 0 ? v.split(',') : undefined }
    },
    reserved: {
      get() { return this.$props.data.reserved?.join(',') },
      set(v:string) { 
        if(!v.endsWith(',')) {
          this.$props.data.reserved = v.length > 0 ? v.split(',').map(str => parseInt(str, 10)) : []
        }
      }
    },
  },
  components: { Network, Peer }
}
</script>