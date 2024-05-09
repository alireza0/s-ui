<template>
  <v-card subtitle="Selector">
    <v-row>
      <v-col cols="12" sm="6">
        <v-text-field v-model="outbounds" label="Outbounds(comma separated)" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.default" label="Default" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" label="Interrupt exist connections" hide-details></v-switch>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
  data() {
    return {}
  },
  computed: {
    outbounds: {
      get() { return this.$props.data.outbounds ? this.$props.data.outbounds.join(',') : '' },
      set(v:string) { this.$props.data.outbounds = v.length > 0 ? v.split(',') : undefined }
    },
  },
}
</script>