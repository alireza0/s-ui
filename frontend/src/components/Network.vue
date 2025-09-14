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
  props: ['data'],
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
      get():string { return this.$props.data.network?? '' },
      set(v:string) { this.$props.data.network = v != '' ? v : undefined }
    }
  }
}
</script>