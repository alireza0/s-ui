<template>
  <v-card subtitle="URL Test">
    <v-row>
      <v-col cols="12" sm="6">
        <v-text-field v-model="outbounds" label="Outbounds(comma separated)" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" v-if="optionUrl">
        <v-text-field v-model="data.url" label="URL" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionInterval">
        <v-text-field
        label="Interval"
        hide-details
        type="number"
        min="3"
        suffix="s"
        v-model.number="interval"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionTolerance">
        <v-text-field
        label="Tolerance"
        hide-details
        type="number"
        min="0"
        suffix="ms"
        v-model.number="tolerance"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionIdle">
        <v-text-field
        label="Idle Timeout"
        hide-details
        type="number"
        min="0"
        suffix="m"
        v-model.number="idle_timeout"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6">
        <v-switch v-model="data.interrupt_exist_connections" color="primary" label="Interrupt exist connections" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
          <template v-slot:activator="{ props }">
            <v-btn v-bind="props" hide-details>SSH Options</v-btn>
          </template>
          <v-card>
            <v-list>
              <v-list-item>
                <v-switch v-model="optionUrl" color="primary" label="Test URL" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionInterval" color="primary" label="Interval" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionTolerance" color="primary" label="Tolerance" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionIdle" color="primary" label="Idle Timeout" hide-details></v-switch>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
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
    },
    outbounds: {
      get() { return this.$props.data.outbounds ? this.$props.data.outbounds.join(',') : '' },
      set(v:string) { this.$props.data.outbounds = v.length > 0 ? v.split(',') : undefined }
    },
  },
}
</script>