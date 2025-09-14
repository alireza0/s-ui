<template>
  <v-card subtitle="Talescale">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" v-model="data.ephemeral" :label="$t('types.ts.ephemeral')"></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" v-model="data.accept_routes" :label="$t('types.ts.acceptRoutes')"></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="optionStateDir">
      <v-col cols="12" sm="8">
        <v-text-field v-model="data.state_directory" :label="$t('types.ts.stateDir')"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionAuth">
      <v-col cols="12" sm="8">
        <v-text-field v-model="data.auth_key" :label="$t('types.ts.authKey')"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionCtrlUrl">
      <v-col cols="12" sm="8">
        <v-text-field v-model="data.control_url" :label="$t('types.ts.controlUrl')"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionHostname">
        <v-text-field v-model="data.hostname" :label="$t('types.ts.hostname')"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionUdpTimeout">
        <v-text-field type="number" v-model.number="udpTimeout" min="1" :suffix="$t('date.s')" :label="$t('types.ts.udpTimeout')"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionExitNode">
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.exit_node" :label="$t('types.ts.exitNode')"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" v-model="data.exit_node_allow_lan_access" :label="$t('types.ts.allowLanAccess')"></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="optionAdvRoutes">
      <v-col cols="12" sm="8">
        <v-text-field v-model="advertise_routes" :label="$t('types.ts.advRoutes') + ' ' + $t('commaSeparated')"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" v-model="data.advertise_exit_node" :label="$t('types.ts.advExitNode')"></v-switch>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.ts.options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionStateDir" color="primary" :label="$t('types.ts.stateDir')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionAuth" color="primary" :label="$t('types.ts.authKey')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionCtrlUrl" color="primary" :label="$t('types.ts.controlUrl')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionHostname" color="primary" :label="$t('types.ts.hostname')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionExitNode" color="primary" :label="$t('types.ts.exitNode')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionAdvRoutes" color="primary" :label="$t('types.ts.advRoutes')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionUdpTimeout" color="primary" :label="$t('types.ts.udpTimeout')" hide-details></v-switch>
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
    optionStateDir: {
      get() { return this.$props.data?.state_directory !== undefined },
      set(v: boolean) { this.$props.data.state_directory = v ? "$HOME/.tailscale" : undefined }
    },
    optionAuth: {
      get() { return this.$props.data?.auth_key !== undefined },
      set(v: boolean) { this.$props.data.auth_key = v ? "" : undefined }
    },
    optionCtrlUrl: {
      get() { return this.$props.data?.control_url !== undefined },
      set(v: boolean) { this.$props.data.control_url = v ? "https://controlplane.tailscale.com" : undefined }
    },
    optionHostname: {
      get() { return this.$props.data?.hostname !== undefined },
      set(v: boolean) { this.$props.data.hostname = v ? "localhost" : undefined }
    },
    optionExitNode: {
      get() { return this.$props.data?.exit_node !== undefined },
      set(v: boolean) { 
        if (v) {
          this.$props.data.exit_node = ""
        } else {
          delete this.$props.data.exit_node
          delete this.$props.data.exit_node_allow_lan_access
        }
      }
    },
    optionAdvRoutes: {
      get() { return this.$props.data?.advertise_routes !== undefined },
      set(v: boolean) { 
        if (v) {
          this.$props.data.advertise_routes = []
        } else {
          delete this.$props.data.advertise_routes
          delete this.$props.data.advertise_exit_node
        }
      }
    },
    optionUdpTimeout: {
      get() { return this.$props.data?.udp_timeout !== undefined },
      set(v: boolean) { this.$props.data.udp_timeout = v ? '30s' : undefined }
    },
    udpTimeout: {
      get() { return this.$props.data?.udp_timeout? this.$props.data.udp_timeout.replace('s','') : '' },
      set(v: number) { this.$props.data.udp_timeout = v>1 ? v + 's' : '30s' }
    },
    advertise_routes: {
      get() { return this.$props.data?.advertise_routes?.join(',') ?? "" },
      set(v: string) { this.$props.data.advertise_routes = v.length > 0 ? v.split(',') : [] }
    },
  },
}
</script>