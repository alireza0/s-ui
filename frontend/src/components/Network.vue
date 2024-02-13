<template>
  <v-select
    hide-details
    :label="$t('network')"
    :items="networks"
    v-model="Network">
  </v-select>
</template>

<script lang="ts">
export default {
  props: ['inbound'],
  data() {
    return {
      networks: [
        { title: "TCP/UDP", value: '' },
        { title: "TCP", value: 'tcp' },
        { title: "UDP", value: 'udp' },
      ],
    }
  },
  computed: {
    Network: {
      get():string { return this.$props.inbound.network?? '' },
      set(v:string) { this.$props.inbound.network = v != '' ? v : undefined }
    }
  }
}
</script>