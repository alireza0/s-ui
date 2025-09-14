<template>
  <v-card subtitle="URL Test">
    <v-row>
      <v-col cols="12" sm="6">
        <v-combobox
          v-model="data.outbounds"
          :items="tags"
          :label="$t('pages.outbounds')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" v-if="optionUrl">
        <v-text-field v-model="data.url" :label="$t('types.lb.testUrl')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionInterval">
        <v-text-field
        :label="$t('types.lb.interval')"
        hide-details
        type="number"
        min="3"
        :suffix="$t('date.s')"
        v-model.number="interval"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionTolerance">
        <v-text-field
        :label="$t('types.lb.tolerance')"
        hide-details
        type="number"
        min="0"
        :suffix="$t('date.ms')"
        v-model.number="tolerance"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionIdle">
        <v-text-field
        :label="$t('transport.idleTimeout')"
        hide-details
        type="number"
        min="0"
        :suffix="$t('date.m')"
        v-model.number="idle_timeout"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" :label="$t('types.lb.interruptConn')" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
          <template v-slot:activator="{ props }">
            <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.lb.urlTestOptions') }}</v-btn>
          </template>
          <v-card>
            <v-list>
              <v-list-item>
                <v-switch v-model="optionUrl" color="primary" :label="$t('types.lb.testUrl')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionInterval" color="primary" :label="$t('types.lb.interval')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionTolerance" color="primary" :label="$t('types.lb.tolerance')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionIdle" color="primary" :label="$t('transport.idleTimeout')" hide-details></v-switch>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data', 'tags'],
  data() {
    return {
      menu: false,
    }
  },
  computed: {
    optionUrl: {
      get(): boolean { return this.$props.data.url != undefined },
      set(v:boolean) { this.$props.data.url = v ? 'https://www.gstatic.com/generate_204' : undefined }
    },
    optionInterval: {
      get(): boolean { return this.$props.data.interval != undefined },
      set(v:boolean) { this.$props.data.interval = v ? '3s' : undefined }
    },
    optionTolerance: {
      get(): boolean { return this.$props.data.tolerance != undefined },
      set(v:boolean) { this.$props.data.tolerance = v ? 50 : undefined }
    },
    optionIdle: {
      get(): boolean { return this.$props.data.idle_timeout != undefined },
      set(v:boolean) { this.$props.data.idle_timeout = v ? '30m' : undefined }
    },
    interval: {
      get() { return this.$props.data.interval ? parseInt(this.$props.data.interval.replace('s','')) : 3 },
      set(v:number) { this.$props.data.interval = v > 0 ? v + 's' : '3s' }
    },
    tolerance: {
      get() { return this.$props.data.tolerance ? parseInt(this.$props.data.tolerance) : 0 },
      set(v:number) { this.$props.data.tolerance = v > 0 ? v : 0 }
    },
    idle_timeout: {
      get() { return this.$props.data.idle_timeout ? parseInt(this.$props.data.idle_timeout.replace('m','')) : 30 },
      set(v:number) { this.$props.data.idle_timeout = v > 0 ? v + 'm' : '0m' }
    }
  },
}
</script>