<template>
  <v-card subtitle="Direct">
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'in'">
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Override Address"
        hide-details
        v-model="data.override_address">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Override Port"
        type="number"
        min="0"
        hide-details
        v-model="override_port">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'out'">
        <v-text-field
        label="Proxy Protocol"
        type="number"
        min="0"
        max="2"
        hide-details
        v-model="proxy_protocol">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
  props: ['direction','data'],
  data() {
    return {}
  },
  computed: {
    override_port: {
        get() { return this.$props.data.override_port ? this.$props.data.override_port : ''; },
        set(newValue: any) { this.$props.data.override_port = newValue.length == 0 || newValue == 0 ? undefined : parseInt(newValue); }
    },
    proxy_protocol: {
      get() { return this.$props.data.proxy_protocol ? this.$props.data.proxy_protocol : ''; },
      set(newValue: any) { this.$props.data.proxy_protocol = newValue.length == 0 || newValue == 0 ? undefined : parseInt(newValue); }
    },
  },
  components: { Network }
}
</script>