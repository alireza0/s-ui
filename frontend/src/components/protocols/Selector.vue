<template>
  <v-card subtitle="Selector">
    <v-row>
      <v-col cols="12" sm="6">
        <v-combobox
          v-model="data.outbounds"
          :items="tags"
          :label="$t('pages.outbounds')"
          multiple
          @update:model-value="updateDefault"
          chips
          hide-details
        ></v-combobox>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-combobox
          v-model="data.default"
          :items="data.outbounds"
          :label="$t('types.lb.defaultOut')"
          clearable
          hide-details
        ></v-combobox>
      </v-col>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" :label="$t('types.lb.interruptConn')" hide-details></v-switch>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data','tags'],
  data() {
    return {}
  },
  methods: {
    updateDefault() {
      if (!this.$props.data.outbounds?.includes(this.$props.data.default)) {
        delete this.$props.data.default
      }
    }
  },
}
</script>