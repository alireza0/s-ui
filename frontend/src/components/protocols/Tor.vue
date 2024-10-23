<template>
  <v-card subtitle="Tor">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.executable_path" :label="$t('types.tor.execPath')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.data_directory" :label="$t('types.tor.dataDir')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="extra_args" :label="$t('types.tor.extArgs') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
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
    extra_args: {
      get() { return this.$props.data.extra_args?.join(',') },
      set(v:string) { this.$props.data.extra_args = v.length > 0 ? v.split(',') : undefined }
    },
  },
}
</script>