<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.hosts')"
      hide-details
      v-model="transport.host">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('transport.path')"
      hide-details
      v-model="transport.path">
      </v-text-field>
    </v-col>
  </v-row>
  <Headers :data="transport" />
</template>

<script lang="ts">
import Headers from '../Headers.vue'
export default {
  props: ['transport'],
  data() {
    return {
    }
  },
  components: { Headers }
}
</script>