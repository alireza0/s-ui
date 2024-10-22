<template>
  <v-card :subtitle="$t('objects.multiplex')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" :label="$t('mux.enable')" v-model="muxEnable" hide-details></v-switch>
      </v-col>
      <template v-if="mux.enabled">
        <template v-if="direction=='out'">
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :items="[ 'smux', 'yamux', 'h2mux']"
              :label="$t('protocol')"
              clearable
              @click:clear="mux.protocol=undefined"
              v-model="mux.protocol">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
            :label="$t('mux.maxConn')"
            hide-details
            type="number"
            min=0
            v-model.number="max_connections">
            </v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
            :label="$t('mux.minStr')"
            hide-details
            type="number"
            min=0
            v-model.number="min_streams">
            </v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
            :label="$t('mux.maxStr')"
            hide-details
            type="number"
            :min="min_streams"
            v-model.number="max_streams">
            </v-text-field>
          </v-col>
        </template>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" :label="$t('mux.padding')" v-model="mux.padding" hide-details></v-switch>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" :label="$t('mux.enableBrutal')" v-model="burtalEnable" hide-details></v-switch>
        </v-col>
      </template>
    </v-row>
    <v-row v-if="mux.brutal?.enabled">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('stats.upload')"
        hide-details
        type="number"
        :suffix="$t('stats.Mbps')"
        v-model.number="up_mbps">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('stats.download')"
        hide-details
        type="number"
        :suffix="$t('stats.Mbps')"
        min="0"
        v-model.number="down_mbps">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { oMultiplex } from '@/types/multiplex'
export default {
  props: ['data', 'direction'],
  data() {
    return {}
  },
  computed: {
    mux(): oMultiplex {
      if (!Object.hasOwn(this.$props.data,"multiplex")) this.$props.data.multiplex = {}
      return <oMultiplex> this.$props.data.multiplex
    },
    muxEnable: {
      get(): boolean { return this.mux ? this.mux.enabled : false },
      set(newValue:boolean) { this.$props.data.multiplex = newValue ? { enabled: newValue } : {} }
    },
    max_connections: {
      get(): number { return this.mux.max_connections ? this.mux.max_connections : 0 },
      set(newValue:number) { this.mux.max_connections = newValue > 0 ? newValue : undefined }
    },
    min_streams: {
      get(): number { return this.mux.min_streams ? this.mux.min_streams : 0 },
      set(newValue:number) { this.mux.min_streams = newValue > 0 ? newValue : undefined }
    },
    max_streams: {
      get(): number { return this.mux.max_streams ? this.mux.max_streams : 0 },
      set(newValue:number) { this.mux.max_streams = newValue > 0 ? newValue : undefined }
    },
    burtalEnable: {
      get(): boolean { return this.mux.brutal ? this.mux.brutal.enabled : false },
      set(newValue:boolean) { this.mux.brutal = newValue ? { enabled: newValue, up_mbps: 100, down_mbps: 100 } : undefined }
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