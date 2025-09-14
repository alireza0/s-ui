<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.hosts')"
      hide-details
      v-model="hosts">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.path')"
      hide-details
      v-model="transport.path">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-select
      :label="$t('transport.httpMethod')"
      hide-details
      clearable
      @click:clear="delete transport.method"
      :items="methodList"
      v-model="transport.method">
      </v-select>
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
  <Headers :data="transport" />
</template>

<script lang="ts">
import { HTTP } from '../../types/transport'
import Headers from '../Headers.vue'
export default {
  props: ['transport'],
  data() {
    return {
      methodList: ['POST', 'GET', 'PUT', 'PATCH', 'DELETE']
    }
  },
  computed: {
    Http(): HTTP {
      return <HTTP> this.$props.transport?? {}
    },
    hosts: {
      get() { return this.Http.host ? this.Http.host.join(',') : '' },
      set(newValue:string) { this.$props.transport.host = newValue.length>0 ? newValue.split(',') : [] }
    },
    idle_timeout: {
      get() { return this.Http.idle_timeout ? parseInt(this.Http.idle_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.transport.idle_timeout = newValue ? newValue + 's' : '' }
    },
    ping_timeout: {
      get() { return this.Http.ping_timeout ? parseInt(this.Http.ping_timeout.replace('s','')) : '' },
      set(newValue:number) { this.$props.transport.ping_timeout = newValue ? newValue + 's' : '' }
    }
  },
  components: { Headers }
}
</script>