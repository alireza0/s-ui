<template>
  <v-card subtitle="SOCKS">
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
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="['4','4a','5']"
          :label="$t('version')"
          v-model="data.version">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <Network :data="data" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <UoT :data="data" />
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'
import UoT from '@/components/UoT.vue'

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
  components: { Network, UoT }
}
</script>