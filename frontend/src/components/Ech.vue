<template>
  <v-card subtitle="ECH" style="background-color: inherit;">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" :label="$t('enable')" v-model="enabled" hide-details></v-switch>
      </v-col>
    </v-row>
    <template v-if="enabled">
      <v-row>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" label="Post-Quantum Schemes" v-model="ech.pq_signature_schemes_enabled" hide-details></v-switch>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" label="Disable Adaptive Size" v-model="ech.dynamic_record_sizing_disabled" hide-details></v-switch>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="auto">
          <v-btn-toggle v-model="useEchPath"
          class="rounded-xl"
          density="compact"
          variant="outlined"
          shaped
          mandatory>
            <v-btn
              @click="delete ech.key"
            >{{ $t('tls.usePath') }}</v-btn>
            <v-btn
              @click="delete ech.key_path"
            >{{ $t('tls.useText') }}</v-btn>
          </v-btn-toggle>
        </v-col>
      </v-row>
      <v-row v-if="useEchPath == 0">
        <v-col cols="12" sm="6">
          <v-text-field
            :label="$t('tls.keyPath')"
            hide-details
            v-model="ech.key_path">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-else>
        <v-col cols="12" sm="6">
          <v-textarea
            :label="$t('tls.key')"
            hide-details
            v-model="echKeyText">
          </v-textarea>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6">
          <v-textarea
            :label="$t('tls.cert')"
            hide-details
            v-model="echConfigText">
          </v-textarea>
        </v-col>
      </v-row>
    </template>
  </v-card>
</template>

<script lang="ts">
import { ech } from '@/types/inTls'

export default {
  props: ['iTls','oTls'],
  data() {
    return {
      useEchPath: 0
    }
  },
  computed: {
    ech() {
      return <ech>this.$props.iTls.ech
    },
    enabled: {
      get() { return this.ech?.enabled?? false },
      set(v: boolean) { 
        this.$props.iTls.ech = v ? { enabled: true } : undefined
        this.$props.oTls.ech = v ? {} : undefined
      }
    },
    echKeyText: {
      get(): string { return this.ech?.key ? this.ech.key.join('\n') : '' },
      set(newValue:string) { this.ech.key = newValue.split('\n') }
    },
    echConfigText: {
      get(): string { return this.oTls.ech?.config ? this.oTls.ech.config.join('\n') : '' },
      set(newValue:string) { this.oTls.ech.config = newValue.split('\n') }
    },
  }
}
</script>