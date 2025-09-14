<template>
  <v-card subtitle="HTTP">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.un')"
        hide-details
        v-model="username">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('types.pw')"
        hide-details
        v-model="password">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('transport.path')"
        hide-details
        v-model="data.path">
        </v-text-field>
      </v-col>
    </v-row>
    <Headers :data="data" />
  </v-card>
</template>

<script lang="ts">
import Headers from '@/components/Headers.vue'

export default {
  props: ['data'],
  data() {
    return {}
  },
  computed: {
    username: {
      get(): string { return this.data.username?.length > 0 ? this.data.username : '' },
      set(v:string) { this.data.username = v.length > 0 ? v : undefined },
    },
    password: {
      get(): string { return this.data.password?.length > 0 ? this.data.password : '' },
      set(v:string) { this.data.password = v.length > 0 ? v : undefined },
    },
  },
  components: { Headers }
}
</script>