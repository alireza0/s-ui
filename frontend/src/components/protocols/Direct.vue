<template>
  <v-card subtitle="Direct">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <Network :inbound="inbound" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Override Address"
        hide-details
        v-model="inbound.override_address">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Override Port"
        type="number"
        min="0"
        hide-details
        v-model="override_port">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
    props: ['inbound'],
    data() {
        return {}
    },
    computed: {
        override_port: {
            get() { return this.$props.inbound.override_port ? this.$props.inbound.override_port : ''; },
            set(newValue: any) { this.$props.inbound.override_port = newValue.length == 0 || newValue == 0 ? undefined : parseInt(newValue); }
        },
    },
    components: { Network }
}
</script>