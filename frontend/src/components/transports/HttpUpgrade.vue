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
</template>

<script lang="ts">
export default {
  props: ['transport'],
  data() {
    return {
    }
  }
}
</script>