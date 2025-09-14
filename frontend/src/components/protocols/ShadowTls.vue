<template>
  <v-card subtitle="ShadowTls">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="[1,2,3]"
          :label="$t('version')"
          :disabled="data.id > 0"
          v-model="version">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.password != undefined">
        <v-text-field
        :label="$t('types.pw')"
        hide-details
        v-model="data.password">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="Inbound.wildcard_sni != undefined">
        <v-select label="Wildcard SNI" :items="['off', 'authed', 'all']" clearable v-model="Inbound.wildcard_sni"></v-select>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.shdwTls.hs')"
        hide-details
        v-model="Inbound.handshake.server">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('out.port')"
        type="number"
        min="0"
        hide-details
        v-model.number="server_port">
        </v-text-field>
      </v-col>
    </v-row>
    <Dial :dial="Inbound.handshake" />
    <v-row v-if="Inbound.handshake_for_server_name != undefined">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.shdwTls.addHS')"
        hide-details
        v-model="handshake_server">
        <template v-slot:append>
          <v-chip 
            color="primary"
            density="compact"
            variant="elevated"
            :disabled="handshake_server == ''"
            @click="addHandshakeServer()">
            <v-icon icon="mdi-plus" />
          </v-chip>
        </template>
        </v-text-field>
      </v-col>
    </v-row>
    <v-card
      v-for="(value, key) in Inbound.handshake_for_server_name"
      border
      density="compact"
      style="margin: 5px;"
      color="background">
      <v-card-title>
        <v-row>
          <v-col>{{ key }}
            <v-icon icon="mdi-delete" color="error" size="small"
            @click="Inbound.handshake_for_server_name ? delete Inbound.handshake_for_server_name[key] : null" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-row>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          :label="$t('types.shdwTls.hs')"
          hide-details
          v-model="value.server">
          </v-text-field>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          :label="$t('out.port')"
          type="number"
          min="0"
          hide-details
          v-model.number="value.server_port">
          </v-text-field>
        </v-col>
      </v-row>
      <Dial :dial="value" />
    </v-card>
  </v-card>
</template>

<script lang="ts">
import { ShadowTLS } from '@/types/inbounds'
import Dial from '../Dial.vue'

export default {
  props: ['data'],
  data() {
    return {
      handshake_server: ''
    }
  },
  methods: {
    addHandshakeServer() {
      this.data.handshake_for_server_name[this.handshake_server] = {}
      // Clear the input field after adding the server
      this.handshake_server = ''
    }
  },
  mounted() {
    this.version = this.Inbound.version
  },
  computed: {
    version: {
      get() {
        this.version = this.Inbound.version
        return this.Inbound.version
      },
      set(newValue: any) {
        switch (newValue) {
        case 1:
          delete this.Inbound.password
          delete this.Inbound.handshake_for_server_name
          delete this.Inbound.wildcard_sni
          break
        case 2:
          if (!this.Inbound.password) {
            this.Inbound.password = ""
          }
          if (!this.Inbound.handshake_for_server_name) {
            this.Inbound.handshake_for_server_name = {}
          }
          delete this.Inbound.wildcard_sni
          break
        case 3:
          delete this.Inbound.password
          if (!this.Inbound.handshake_for_server_name) {
            this.Inbound.handshake_for_server_name = {}
          }
          if (!this.Inbound.wildcard_sni) {
            this.Inbound.wildcard_sni = ""
          }
          break
        }
        this.Inbound.version = newValue
      }
    },
    Inbound(): ShadowTLS {
      return <ShadowTLS>this.$props.data
    },
    server_port: {
      get() { return this.Inbound.handshake.server_port ? this.Inbound.handshake.server_port : 443 },
      set(newValue: any) { this.Inbound.handshake.server_port = newValue.length == 0 || newValue == 0 ? 443 : parseInt(newValue) }
    },
  },
  components: { Dial }
}
</script>