<template>
  <v-card subtitle="TProxy">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <Network :data="inbound" />
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
  props: ['inbound'],
  data() {
    return {
    }
  },
  components: { Network }
}
</script>