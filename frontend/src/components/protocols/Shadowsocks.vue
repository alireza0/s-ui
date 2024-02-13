<template>
  <v-card subtitle="Shadowsocks">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          label="Method"
          :items="ssMethods"
          v-model="inbound.method">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="inbound.password" label="Password" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <Network :inbound="inbound" />
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import Network from '@/components/Network.vue'

export default {
    props: ['inbound'],
    data() {
        return {
            ssMethods: [
                "none",
                "aes-128-gcm",
                "aes-192-gcm",
                "aes-256-gcm",
                "chacha20-ietf-poly1305",
                "xchacha20-ietf-poly1305",
                "2022-blake3-aes-128-gcm",
                "2022-blake3-aes-256-gcm",
                "2022-blake3-chacha20-poly1305"
            ]
        }
    },
    components: { Network }
}
</script>