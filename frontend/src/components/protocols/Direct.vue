<template>
  <v-card subtitle="Direct">
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'in'">
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.direct.overrideAddr')"
        hide-details
        v-model="data.override_address">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.direct.overridePort')"
        type="number"
        min="0"
        hide-details
        v-model.number="override_port">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="direction == 'out'">
        <v-select
        :label="$t('types.direct.proxyProtocol')"
        :items="[1,2]"
        hide-details
        clearable
        @click:clear="delete data.proxy_protocol"
        v-model.number="data.proxy_protocol">
        </v-select>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
  props: ['direction','data'],
  data() {
    return {}
  },
  computed: {
    override_port: {
        get() { return this.$props.data.override_port ? this.$props.data.override_port : ''; },
        set(newValue: any) { this.$props.data.override_port = newValue.length == 0 || newValue == 0 ? undefined : parseInt(newValue); }
    },
  },
  components: { Network }
}
</script>