<template>
  <v-select
    hide-details
    label="UDP over TCP"
    :items="versions"
    v-model="udp_over_tcp">
  </v-select>
</template>

<script lang="ts">
export default {
  props: ['data'],
  data() {
    return {
      versions: [
        { title: this.$t('disable'), value: 0 },
        { title: "1", value: 1 },
        { title: "2", value: 2 },
      ],
    }
  },
  computed: {
    udp_over_tcp: {
      get():number { return this.$props.data.udp_over_tcp?.version?? 0 },
      set(v:number) { this.$props.data.udp_over_tcp = v > 0 ? { enabled: true, version: v } : undefined }
    }
  }
}
</script>