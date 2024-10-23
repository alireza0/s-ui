<template>
  <v-card subtitle="Hysteria2">
    <v-row v-if="direction == 'in'">
      <v-col cols="12" sm="6" md="4" v-if="data.masquerade != undefined">
        <v-text-field
        label="HTTP3 server on auth fail"
        hide-details
        v-model="data.masquerade">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.ignore_client_bandwidth" color="primary" :label="$t('types.hy.ignoreBw')" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-else>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.pw')"
        hide-details
        v-model="data.password">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <Network :data="data" />
      </v-col>
    </v-row>
    <v-row v-if="!data.ignore_client_bandwidth">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('stats.upload')"
        hide-details
        type="number"
        :suffix="$t('stats.Mbps')"
        min="0"
        v-model.number="up_mbps">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('stats.download')"
        hide-details
        type="number"
        :suffix="$t('stats.Mbps')"
        min="0"
        v-model.number="down_mbps">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="data.obfs != undefined">
      <v-col cols="12" sm="6" md="4">
       <v-text-field
        :label="$t('types.hy.obfs')"
        hide-details
        v-model="data.obfs.password">
        </v-text-field>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.hy.hy2Options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionObfs" color="primary" :label="$t('types.hy.obfs')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionMasq" color="primary" label="Masquerade" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
  props: ['direction', 'data'],
  data() {
    return {
      menu: false,
    }
  },
  computed: {
    down_mbps: {
      get() { return this.$props.data.down_mbps?? 0 },
      set(newValue:number) { this.$props.data.down_mbps = newValue>0 ? newValue : undefined }
    },
    up_mbps: {
      get() { return this.$props.data.up_mbps?? 0 },
      set(newValue:number) { this.$props.data.up_mbps = newValue>0 ? newValue : undefined }
    },
    optionObfs: {
      get(): boolean { return this.$props.data.obfs != undefined },
      set(v:boolean) { this.$props.data.obfs = v ? { type: "salamander", password: "" } : undefined }
    },
    optionMasq: {
      get(): boolean { return this.$props.data.masquerade != undefined },
      set(v:boolean) { this.$props.data.masquerade = v ? "" : undefined }
    }
  },
  components: { Network }
}
</script>